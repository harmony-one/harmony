package quorum

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
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
	Participants() []*bls.PublicKey
	IndexOf(*bls.PublicKey) int
	ParticipantsCount() int64
	NextAfter(*bls.PublicKey) (bool, *bls.PublicKey)
	UpdateParticipants(pubKeys []*bls.PublicKey)
}

// SignatoryTracker ..
type SignatoryTracker interface {
	ParticipantTracker
	SubmitVote(
		p Phase, PubKey *bls.PublicKey,
		sig *bls.Sign, headerHash common.Hash,
		height, viewID uint64,
	) (*votepower.Ballot, error)
	// Caller assumes concurrency protection
	SignersCount(Phase) int64
	reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	SignatoryTracker
	ReadAllBallots(Phase) []*votepower.Ballot
	ReadBallot(p Phase, PubKey *bls.PublicKey) *votepower.Ballot
	TwoThirdsSignersCount() int64
	// 96 bytes aggregated signature
	AggregateVotes(p Phase) *bls.Sign
}

// DependencyInjectionWriter ..
type DependencyInjectionWriter interface {
	SetMyPublicKeyProvider(func() (*multibls.PublicKey, error))
}

// DependencyInjectionReader ..
type DependencyInjectionReader interface {
	MyPublicKey() func() (*multibls.PublicKey, error)
}

// Decider ..
type Decider interface {
	fmt.Stringer
	SignatureReader
	DependencyInjectionWriter
	SetVoters(subCommittee *shard.Committee) (*TallyResult, error)
	Policy() Policy
	IsQuorumAchieved(Phase) bool
	IsQuorumAchievedByMask(mask *bls_cosi.Mask) bool
	QuorumThreshold() numeric.Dec
	AmIMemberOfCommitee() bool
	IsRewardThresholdAchieved() bool
	ResetPrepareAndCommitVotes()
	ResetViewChangeVotes()
}

// Registry ..
type Registry struct {
	Deciders      map[string]Decider `json:"quorum-deciders"`
	ExternalCount int                `json:"external-slot-count"`
}

// NewRegistry ..
func NewRegistry(extern int) Registry {
	return Registry{map[string]Decider{}, extern}
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
	publicKeys []*bls.PublicKey
	prepare    *votepower.Round
	commit     *votepower.Round
	// viewIDSigs: every validator
	// sign on |viewID|blockHash| in view changing message
	viewChange *votepower.Round
}

type depInject struct {
	shardIDProvider   func() (uint32, error)
	publicKeyProvider func() (*multibls.PublicKey, error)
}

func (s *cIdentities) AggregateVotes(p Phase) *bls.Sign {
	ballots := s.ReadAllBallots(p)
	sigs := make([]*bls.Sign, 0, len(ballots))
	for _, ballot := range ballots {
		sig := &bls.Sign{}
		// NOTE invariant that shouldn't happen by now
		// but pointers are pointers
		if ballot != nil {
			sig.DeserializeHexStr(common.Bytes2Hex(ballot.Signature))
			sigs = append(sigs, sig)
		}
	}
	return bls_cosi.AggregateSig(sigs)
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
	for i := range pubKeys {
		k := shard.BlsPublicKey{}
		k.FromLibBLSPublicKey(pubKeys[i])
	}
	s.publicKeys = append(pubKeys[:0:0], pubKeys...)
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

func (s *cIdentities) SubmitVote(
	p Phase, PubKey *bls.PublicKey,
	sig *bls.Sign, headerHash common.Hash,
	height, viewID uint64,
) (*votepower.Ballot, error) {
	// Note safe to assume by this point because key has been
	// checked earlier
	key := *shard.FromLibBLSPublicKeyUnsafe(PubKey)
	ballot := &votepower.Ballot{
		SignerPubKey:    key,
		BlockHeaderHash: headerHash,
		Signature:       common.Hex2Bytes(sig.SerializeToHexStr()),
		Height:          height,
		ViewID:          viewID,
	}
	switch p {
	case Prepare:
		s.prepare.BallotBox[key] = ballot
	case Commit:
		s.commit.BallotBox[key] = ballot
	case ViewChange:
		s.viewChange.BallotBox[key] = ballot
	default:
		return nil, errors.Wrapf(errPhaseUnknown, "given: %s", p.String())
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

func (s *cIdentities) ReadBallot(p Phase, PubKey *bls.PublicKey) *votepower.Ballot {
	ballotBox := map[shard.BlsPublicKey]*votepower.Ballot{}
	key := *shard.FromLibBLSPublicKeyUnsafe(PubKey)

	switch p {
	case Prepare:
		ballotBox = s.prepare.BallotBox
	case Commit:
		ballotBox = s.commit.BallotBox
	case ViewChange:
		ballotBox = s.viewChange.BallotBox
	}

	payload, ok := ballotBox[key]
	if !ok {
		return nil
	}
	return payload
}

func (s *cIdentities) ReadAllBallots(p Phase) []*votepower.Ballot {
	m := map[shard.BlsPublicKey]*votepower.Ballot{}
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

func newBallotsBackedSignatureReader() *cIdentities {
	return &cIdentities{
		publicKeys: []*bls.PublicKey{},
		prepare:    votepower.NewRound(),
		commit:     votepower.NewRound(),
		viewChange: votepower.NewRound(),
	}
}

type composite struct {
	DependencyInjectionWriter
	DependencyInjectionReader
	SignatureReader
}

func (d *depInject) SetMyPublicKeyProvider(p func() (*multibls.PublicKey, error)) {
	d.publicKeyProvider = p
}

func (d *depInject) MyPublicKey() func() (*multibls.PublicKey, error) {
	return d.publicKeyProvider
}

// NewDecider ..
func NewDecider(p Policy, shardID uint32) Decider {
	signatureStore := newBallotsBackedSignatureReader()
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
			*votepower.NewRoster(shardID),
			newBallotBox(),
		}
	default:
		// Should not be possible
		return nil
	}
}
