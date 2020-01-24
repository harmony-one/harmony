package quorum

import (
	"fmt"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
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

var phaseNames = map[Phase]string{
	Prepare:    "Prepare",
	Commit:     "Prepare",
	ViewChange: "Commit",
}

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
	Participants() []*bls.PublicKey
	IndexOf(*bls.PublicKey) int
	ParticipantsCount() int64
	NextAfter(*bls.PublicKey) (bool, *bls.PublicKey)
	UpdateParticipants(pubKeys []*bls.PublicKey)
	DumpParticipants() []string
}

// SignatoryTracker ..
type SignatoryTracker interface {
	ParticipantTracker
	AddSignature(
		p Phase, PubKey *bls.PublicKey,
		sig *bls.Sign, roundLeader *bls.PublicKey, roundNumber uint64,
	)
	// Caller assumes concurrency protection
	SignersCount(Phase) int64
	reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	SignatoryTracker
	ReadAllSignatures(Phase) []*bls.Sign
	ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign
	TwoThirdsSignersCount() int64
	// 96 bytes aggregated signature
	AggregateVotes(p Phase) *bls.Sign
}

// DependencyInjectionWriter ..
type DependencyInjectionWriter interface {
	SetShardIDProvider(func() (uint32, error))
	SetMyPublicKeyProvider(func() (*bls.PublicKey, error))
}

// DependencyInjectionReader ..
type DependencyInjectionReader interface {
	ShardIDProvider() func() (uint32, error)
	MyPublicKey() func() (*bls.PublicKey, error)
}

//WithJSONDump representation dump
type WithJSONDump interface {
	JSON() string
}

// Decider ..
type Decider interface {
	SignatureReader
	DependencyInjectionWriter
	WithJSONDump
	ToggleActive(*bls.PublicKey) bool
	SetVoters(shard.SlotList, bool) (*TallyResult, error)
	Policy() Policy
	IsQuorumAchieved(Phase) bool
	IsQuorumAchievedByMask(mask *bls_cosi.Mask, debug bool) bool
	QuorumThreshold() numeric.Dec
	AmIMemberOfCommitee() bool
	IsRewardThresholdAchieved() bool
	ResetPrepareAndCommitVotes()
	ResetViewChangeVotes()
}

// These maps represent the signatories (validators), keys are BLS public keys
// and values are BLS private key signed signatures
type cIdentities struct {
	// Public keys of the committee including leader and validators
	publicKeys   []*bls.PublicKey
	announcement *votepower.Round
	commit       *votepower.Round
	// viewIDSigs: every validator
	// sign on |viewID|blockHash| in view changing message
	viewID      *votepower.Round
	seenCounter map[[shard.PublicKeySizeInBytes]byte]int
}

type depInject struct {
	shardIDProvider   func() (uint32, error)
	publicKeyProvider func() (*bls.PublicKey, error)
}

func (s *cIdentities) AggregateVotes(p Phase) *bls.Sign {
	return bls_cosi.AggregateSig(s.ReadAllSignatures(p))
}

func (s *cIdentities) IndexOf(pubKey *bls.PublicKey) int {
	idx := -1
	for k, v := range s.publicKeys {
		if v.IsEqual(pubKey) {
			idx = k
		}
	}
	return idx
}

func (s *cIdentities) NextAfter(pubKey *bls.PublicKey) (bool, *bls.PublicKey) {
	found := false
	idx := s.IndexOf(pubKey)
	if idx != -1 {
		found = true
	}
	idx = (idx + 1) % int(s.ParticipantsCount())
	return found, s.publicKeys[idx]
}

func (s *cIdentities) Participants() []*bls.PublicKey {
	return s.publicKeys
}

func (s *cIdentities) UpdateParticipants(pubKeys []*bls.PublicKey) {
	// TODO - might need to put reset of seen counter in separate method
	s.seenCounter = make(map[[shard.PublicKeySizeInBytes]byte]int, len(pubKeys))
	for i := range pubKeys {
		k := shard.BlsPublicKey{}
		k.FromLibBLSPublicKey(pubKeys[i])
		s.seenCounter[k] = 0
	}
	s.publicKeys = append(pubKeys[:0:0], pubKeys...)
}

func (s *cIdentities) DumpParticipants() []string {
	keys := make([]string, len(s.publicKeys))
	for i := range s.publicKeys {
		keys[i] = s.publicKeys[i].SerializeToHexStr()
	}
	return keys
}

func (s *cIdentities) ParticipantsCount() int64 {
	return int64(len(s.publicKeys))
}

func (s *cIdentities) SignersCount(p Phase) int64 {
	switch p {
	case Prepare:
		return int64(len(s.announcement.BallotBox))
	case Commit:
		return int64(len(s.commit.BallotBox))
	case ViewChange:
		return int64(len(s.viewID.BallotBox))
	default:
		return 0

	}
}

func (s *cIdentities) AddSignature(
	p Phase, PubKey *bls.PublicKey,
	sig *bls.Sign, roundLeader *bls.PublicKey, roundNumber uint64,
) {
	hex := PubKey.SerializeToHexStr()
	ballot := &votepower.Ballot{
		*shard.FromLibBLSPublicKeyUnsafe(PubKey),
		*shard.FromLibBLSPublicKeyUnsafe(roundLeader),
		roundNumber,
		sig,
	}

	switch p {
	case Prepare:
		s.announcement.BallotBox[hex] = ballot
	case Commit:
		s.commit.BallotBox[hex] = ballot
	case ViewChange:
		s.viewID.BallotBox[hex] = ballot
	}
}

func (s *cIdentities) reset(ps []Phase) {
	for i := range ps {
		switch m := votepower.NewRound(); ps[i] {
		case Prepare:
			s.announcement = m
		case Commit:
			s.commit = m
		case ViewChange:
			s.viewID = m
		}
	}
}

func (s *cIdentities) TwoThirdsSignersCount() int64 {
	return s.ParticipantsCount()*2/3 + 1
}

func (s *cIdentities) ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign {
	var ballotBox map[string]*votepower.Ballot
	hex := PubKey.SerializeToHexStr()

	switch p {
	case Prepare:
		ballotBox = s.announcement.BallotBox
	case Commit:
		ballotBox = s.commit.BallotBox
	case ViewChange:
		ballotBox = s.viewID.BallotBox
	}

	payload, ok := ballotBox[hex]
	if !ok {
		return nil
	}
	return payload.Signature
}

func (s *cIdentities) ReadAllSignatures(p Phase) []*bls.Sign {
	var m map[string]*votepower.Ballot
	switch p {
	case Prepare:
		m = s.announcement.BallotBox
	case Commit:
		m = s.commit.BallotBox
	case ViewChange:
		m = s.viewID.BallotBox
	}
	sigs := make([]*bls.Sign, 0, len(m))
	for _, value := range m {
		sigs = append(sigs, value.Signature)
	}
	return sigs
}

func newMapBackedSignatureReader() *cIdentities {
	return &cIdentities{
		publicKeys:   []*bls.PublicKey{},
		announcement: votepower.NewRound(),
		commit:       votepower.NewRound(),
		viewID:       votepower.NewRound(),
		seenCounter:  map[[shard.PublicKeySizeInBytes]byte]int{},
	}
}

type composite struct {
	DependencyInjectionWriter
	DependencyInjectionReader
	SignatureReader
}

func (d *depInject) SetShardIDProvider(p func() (uint32, error)) {
	d.shardIDProvider = p
}

func (d *depInject) ShardIDProvider() func() (uint32, error) {
	return d.shardIDProvider
}

func (d *depInject) SetMyPublicKeyProvider(p func() (*bls.PublicKey, error)) {
	d.publicKeyProvider = p
}

func (d *depInject) MyPublicKey() func() (*bls.PublicKey, error) {
	return d.publicKeyProvider
}

// NewDecider ..
func NewDecider(p Policy) Decider {
	signatureStore := newMapBackedSignatureReader()
	deps := &depInject{}
	c := &composite{deps, deps, signatureStore}
	switch p {
	case SuperMajorityVote:
		return &uniformVoteWeight{
			c.DependencyInjectionWriter, c.DependencyInjectionReader, c,
		}
	case SuperMajorityStake:
		return &stakedVoteWeight{
			c.SignatureReader,
			c.DependencyInjectionWriter,
			c.DependencyInjectionWriter.(DependencyInjectionReader),
			*votepower.NewRoster(),
			newBallotBox(),
		}
	default:
		// Should not be possible
		return nil
	}
}
