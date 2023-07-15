package quorum

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
)

// Phase is a phase that needs quorum to proceed
type Phase byte

const (
	// Prepare ..
	Prepare Phase = iota
	// Commit ..
	Commit
	// ViewChange ..
	ViewChange
)

var (
	phaseNames = map[Phase]string{
		Prepare:    "Prepare",
		Commit:     "Commit",
		ViewChange: "viewChange",
	}
	errPhaseUnknown = errors.New("invariant of known phase violated")
)

func (p Phase) String() string {
	if name, ok := phaseNames[p]; ok {
		return name
	}
	return fmt.Sprintf("Unknown Quorum Phase %+v", byte(p))
}

// Policy is the rule we used to decide is quorum achieved
type Policy byte

const (
	// SuperMajorityVote is a 2/3s voting mechanism, pre-PoS
	SuperMajorityVote Policy = iota
	// SuperMajorityStake is 2/3s of total staked amount for epoch
	SuperMajorityStake
)

var policyNames = map[Policy]string{
	SuperMajorityStake: "SuperMajorityStake",
	SuperMajorityVote:  "SuperMajorityVote",
}

func (p Policy) String() string {
	if name, ok := policyNames[p]; ok {
		return name
	}
	return fmt.Sprintf("Unknown Quorum Policy %+v", byte(p))

}

// ParticipantTracker ..
type ParticipantTracker interface {
	Participants() multibls.PublicKeys
	IndexOf(bls.SerializedPublicKey) int
	ParticipantsCount() int64
	// NthNextValidator returns key for next validator. It assumes external validators and leader rotation.
	NthNextValidator(slotList shard.SlotList, pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper)
	NthNextHmy(instance shardingconfig.Instance, pubkey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper)
	NthNextHmyExt(shardingconfig.Instance, *bls.PublicKeyWrapper, int) (bool, *bls.PublicKeyWrapper)
	FirstParticipant(shardingconfig.Instance) *bls.PublicKeyWrapper
	UpdateParticipants(pubKeys, allowlist []bls.PublicKeyWrapper)
}

// SignatoryTracker ..
type SignatoryTracker interface {
	ParticipantTracker
	// This func shouldn't be called directly from outside of quorum. Use AddNewVote instead.
	submitVote(
		p Phase, pubkeys []bls.SerializedPublicKey,
		sig *bls_core.Sign, headerHash common.Hash,
		height, viewID uint64,
	) (*votepower.Ballot, error)
	// Caller assumes concurrency protection
	SignersCount(Phase) int64
	reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	SignatoryTracker
	ReadBallot(p Phase, pubkey bls.SerializedPublicKey) *votepower.Ballot
	TwoThirdsSignersCount() int64
	// 96 bytes aggregated signature
	AggregateVotes(p Phase) *bls_core.Sign
}

// Decider ..
type Decider interface {
	fmt.Stringer
	SignatureReader
	SetVoters(subCommittee *shard.Committee, epoch *big.Int) (*TallyResult, error)
	Policy() Policy
	AddNewVote(
		p Phase, pubkeys []*bls_cosi.PublicKeyWrapper,
		sig *bls_core.Sign, headerHash common.Hash,
		height, viewID uint64,
	) (*votepower.Ballot, error)
	IsQuorumAchieved(Phase) bool
	IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool
	QuorumThreshold() numeric.Dec
	IsAllSigsCollected() bool
	ResetPrepareAndCommitVotes()
	ResetViewChangeVotes()
	CurrentTotalPower(p Phase) (*numeric.Dec, error)
}

// Registry ..
type Registry struct {
	Deciders      map[string]Decider `json:"quorum-deciders"`
	ExternalCount int                `json:"external-slot-count"`
	MedianStake   numeric.Dec        `json:"epos-median-stake"`
	Epoch         int                `json:"epoch"`
}

// NewRegistry ..
func NewRegistry(extern int, epoch int) Registry {
	return Registry{map[string]Decider{}, extern, numeric.ZeroDec(), epoch}
}

// Transition  ..
type Transition struct {
	Previous Registry `json:"previous"`
	Current  Registry `json:"current"`
}

// These maps represent the signatories (validators), keys are BLS public keys
// and values are BLS private key signed signatures
type cIdentities struct {
	// Public keys of the committee including leader and validators
	publicKeys  []bls.PublicKeyWrapper
	keyIndexMap map[bls.SerializedPublicKey]int
	// every element is a index of publickKeys
	allowlistIndex []int
	prepare        *votepower.Round
	commit         *votepower.Round
	// viewIDSigs: every validator
	// sign on |viewID|blockHash| in view changing message
	viewChange *votepower.Round
}

func (s *cIdentities) AggregateVotes(p Phase) *bls_core.Sign {
	ballots := s.ReadAllBallots(p)
	sigs := make([]*bls_core.Sign, 0, len(ballots))
	collectedKeys := map[bls_cosi.SerializedPublicKey]struct{}{}
	for _, ballot := range ballots {
		sig := &bls_core.Sign{}
		// NOTE invariant that shouldn't happen by now
		// but pointers are pointers

		// If the multisig from any of the signers in this ballot are already collected,
		// we need to skip this ballot as its multisig is a duplicate.
		alreadyCollected := false
		for _, key := range ballot.SignerPubKeys {
			if _, ok := collectedKeys[key]; ok {
				alreadyCollected = true
				break
			}
		}
		if alreadyCollected {
			continue
		}

		for _, key := range ballot.SignerPubKeys {
			collectedKeys[key] = struct{}{}
		}

		if ballot != nil {
			sig.DeserializeHexStr(common.Bytes2Hex(ballot.Signature))
			sigs = append(sigs, sig)
		}
	}
	return bls_cosi.AggregateSig(sigs)
}

func (s *cIdentities) IndexOf(pubKey bls.SerializedPublicKey) int {
	if index, ok := s.keyIndexMap[pubKey]; ok {
		return index
	}
	return -1
}

// NthNext return the Nth next pubkey, next can be negative number
func (s *cIdentities) NthNext(pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	found := false

	idx := s.IndexOf(pubKey.Bytes)
	if idx != -1 {
		found = true
	}
	numNodes := int(s.ParticipantsCount())
	// sanity check to avoid out of bound access
	if numNodes <= 0 || numNodes > len(s.publicKeys) {
		numNodes = len(s.publicKeys)
	}
	idx = (idx + next) % numNodes
	return found, &s.publicKeys[idx]
}

// NthNextValidator return the Nth next pubkey nodes, but from another validator.
func (s *cIdentities) NthNextValidator(slotList shard.SlotList, pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	found := false

	if len(s.publicKeys) == 0 {
		return false, pubKey
	}
	if next < 0 {
		return false, pubKey
	}

	publicToAddress := make(map[bls.SerializedPublicKey]common.Address)
	for _, slot := range slotList {
		publicToAddress[slot.BLSPublicKey] = slot.EcdsaAddress
	}

	idx := s.IndexOf(pubKey.Bytes)
	if idx != -1 {
		found = true
	} else {
		utils.Logger().Error().
			Str("key", pubKey.Bytes.Hex()).
			Msg("[NthNextHmy] pubKey not found")
	}
	for {
		numNodes := len(s.publicKeys)
		idx = (idx + next) % numNodes
		if publicToAddress[s.publicKeys[idx].Bytes] == publicToAddress[pubKey.Bytes] {
			// same validator, go next
			idx++
			continue
		}
		return found, &s.publicKeys[idx]
	}
}

func (s *cIdentities) NthNextHmy(instance shardingconfig.Instance, pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	found := false

	idx := s.IndexOf(pubKey.Bytes)
	if idx != -1 {
		found = true
	} else {
		utils.Logger().Error().
			Str("key", pubKey.Bytes.Hex()).
			Msg("[NthNextHmy] pubKey not found")
	}
	numNodes := instance.NumHarmonyOperatedNodesPerShard()
	// sanity check to avoid out of bound access
	if numNodes <= 0 || numNodes > len(s.publicKeys) {
		numNodes = len(s.publicKeys)
	}
	idx = (idx + next) % numNodes
	return found, &s.publicKeys[idx]
}

// NthNextHmyExt return the Nth next pubkey of Harmony + allowlist nodes, next can be negative number
func (s *cIdentities) NthNextHmyExt(instance shardingconfig.Instance, pubKey *bls.PublicKeyWrapper, next int) (bool, *bls.PublicKeyWrapper) {
	found := false

	idx := s.IndexOf(pubKey.Bytes)
	if idx != -1 {
		found = true
	}
	numHmyNodes := instance.NumHarmonyOperatedNodesPerShard()
	// sanity check to avoid out of bound access
	if numHmyNodes <= 0 || numHmyNodes > len(s.publicKeys) {
		numHmyNodes = len(s.publicKeys)
	}
	nth := idx
	if idx >= numHmyNodes {
		nth = sort.SearchInts(s.allowlistIndex, idx) + numHmyNodes
	}

	numExtNodes := instance.ExternalAllowlistLimit()
	if numExtNodes > len(s.allowlistIndex) {
		numExtNodes = len(s.allowlistIndex)
	}

	totalNodes := numHmyNodes + numExtNodes
	// (totalNodes + next%totalNodes) can convert negitive 'next' to positive
	nth = (nth + totalNodes + next%totalNodes) % totalNodes
	if nth < numHmyNodes {
		idx = nth
	} else {
		// find index of external slot key
		idx = s.allowlistIndex[nth-numHmyNodes]
	}
	return found, &s.publicKeys[idx]
}

// FirstParticipant returns the first participant of the shard
func (s *cIdentities) FirstParticipant(instance shardingconfig.Instance) *bls.PublicKeyWrapper {
	return &s.publicKeys[0]
}

func (s *cIdentities) Participants() multibls.PublicKeys {
	return s.publicKeys
}

func (s *cIdentities) UpdateParticipants(pubKeys, allowlist []bls.PublicKeyWrapper) {
	keyIndexMap := map[bls.SerializedPublicKey]int{}
	for i := range pubKeys {
		keyIndexMap[pubKeys[i].Bytes] = i
	}
	for _, key := range allowlist {
		if i, exist := keyIndexMap[key.Bytes]; exist {
			s.allowlistIndex = append(s.allowlistIndex, i)
		}
	}
	sort.Ints(s.allowlistIndex)
	s.publicKeys = pubKeys
	s.keyIndexMap = keyIndexMap
}

func (s *cIdentities) ParticipantsCount() int64 {
	return int64(len(s.publicKeys))
}

func (s *cIdentities) SignersCount(p Phase) int64 {
	switch p {
	case Prepare:
		return int64(len(s.prepare.BallotBox))
	case Commit:
		return int64(len(s.commit.BallotBox))
	case ViewChange:
		return int64(len(s.viewChange.BallotBox))
	default:
		return 0

	}
}

func (s *cIdentities) submitVote(
	p Phase, pubkeys []bls.SerializedPublicKey,
	sig *bls_core.Sign, headerHash common.Hash,
	height, viewID uint64,
) (*votepower.Ballot, error) {
	seenKeys := map[bls.SerializedPublicKey]struct{}{}
	for _, pubKey := range pubkeys {
		if _, ok := seenKeys[pubKey]; ok {
			return nil, errors.Errorf("duplicate key found in votes %x", pubKey)
		}
		seenKeys[pubKey] = struct{}{}

		if ballet := s.ReadBallot(p, pubKey); ballet != nil {
			return nil, errors.Errorf("vote is already submitted %x", pubKey)
		}
	}

	ballot := &votepower.Ballot{
		SignerPubKeys:   pubkeys,
		BlockHeaderHash: headerHash,
		Signature:       common.Hex2Bytes(sig.SerializeToHexStr()),
		Height:          height,
		ViewID:          viewID,
	}

	// For each of the keys signed in the multi-sig, a separate ballot with the same multisig is recorded
	// This way it's easier to check if a specific key already signed or not.
	for _, pubKey := range pubkeys {
		switch p {
		case Prepare:
			s.prepare.BallotBox[pubKey] = ballot
		case Commit:
			s.commit.BallotBox[pubKey] = ballot
		case ViewChange:
			s.viewChange.BallotBox[pubKey] = ballot
		default:
			return nil, errors.Wrapf(errPhaseUnknown, "given: %s", p.String())
		}
	}
	return ballot, nil
}

func (s *cIdentities) reset(ps []Phase) {
	for i := range ps {
		switch m := votepower.NewRound(); ps[i] {
		case Prepare:
			s.prepare = m
		case Commit:
			s.commit = m
		case ViewChange:
			s.viewChange = m
		}
	}
}

func (s *cIdentities) TwoThirdsSignersCount() int64 {
	return s.ParticipantsCount()*2/3 + 1
}

func (s *cIdentities) ReadBallot(p Phase, pubkey bls.SerializedPublicKey) *votepower.Ballot {
	ballotBox := map[bls.SerializedPublicKey]*votepower.Ballot{}

	switch p {
	case Prepare:
		ballotBox = s.prepare.BallotBox
	case Commit:
		ballotBox = s.commit.BallotBox
	case ViewChange:
		ballotBox = s.viewChange.BallotBox
	}

	payload, ok := ballotBox[pubkey]
	if !ok {
		return nil
	}
	return payload
}

func (s *cIdentities) ReadAllBallots(p Phase) []*votepower.Ballot {
	m := map[bls.SerializedPublicKey]*votepower.Ballot{}
	switch p {
	case Prepare:
		m = s.prepare.BallotBox
	case Commit:
		m = s.commit.BallotBox
	case ViewChange:
		m = s.viewChange.BallotBox
	}
	ballots := make([]*votepower.Ballot, 0, len(m))
	for i := range m {
		ballots = append(ballots, m[i])
	}
	return ballots
}

func newCIdentities() *cIdentities {
	return &cIdentities{
		publicKeys:  []bls.PublicKeyWrapper{},
		keyIndexMap: map[bls.SerializedPublicKey]int{},
		prepare:     votepower.NewRound(),
		commit:      votepower.NewRound(),
		viewChange:  votepower.NewRound(),
	}
}

func newBallotsBackedSignatureReader() *cIdentities {
	return newCIdentities()
}

// NewDecider ..
func NewDecider(p Policy, shardID uint32) Decider {
	switch p {
	case SuperMajorityVote:
		return &uniformVoteWeight{
			SignatureReader:            newBallotsBackedSignatureReader(),
			lastPowerSignersCountCache: make(map[Phase]int64),
		}
	case SuperMajorityStake:
		return &stakedVoteWeight{
			SignatureReader: newBallotsBackedSignatureReader(),
			roster:          *votepower.NewRoster(shardID),
			voteTally:       newVoteTally(),
			lastPower:       make(map[Phase]numeric.Dec),
		}
	default:
		// Should not be possible
		return nil
	}
}
