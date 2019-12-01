package quorum

import (
	"fmt"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/votepower"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	// "github.com/harmony-one/harmony/staking/effective"
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
	Prepare:    "Announce",
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
	AddSignature(p Phase, PubKey *bls.PublicKey, sig *bls.Sign)
	// Caller assumes concurrency protection
	SignersCount(Phase) int64
	Reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	SignatoryTracker
	ReadAllSignatures(Phase) []*bls.Sign
	ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign
	TwoThirdsSignersCount() int64
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
	slash.Slasher
	WithJSONDump
	ToggleActive(*bls.PublicKey) bool
	SetVoters(shard.SlotList) (*TallyResult, error)
	Policy() Policy
	IsQuorumAchieved(Phase) bool
	ComputeTotalPowerByMask(*bls_cosi.Mask) *numeric.Dec
	QuorumThreshold() numeric.Dec
	AmIMemberOfCommitee() bool
	IsRewardThresholdAchieved() bool
}

// These maps represent the signatories (validators), keys are BLS public keys
// and values are BLS private key signed signatures
type cIdentities struct {
	// Public keys of the committee including leader and validators
	publicKeys []*bls.PublicKey
	prepare    map[string]*bls.Sign
	commit     map[string]*bls.Sign
	// viewIDSigs: every validator
	// sign on |viewID|blockHash| in view changing message
	viewID      map[string]*bls.Sign
	seenCounter map[[shard.PublicKeySizeInBytes]byte]int
}

type depInject struct {
	shardIDProvider   func() (uint32, error)
	publicKeyProvider func() (*bls.PublicKey, error)
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

func (s *cIdentities) SlashThresholdMet(key shard.BlsPublicKey) bool {
	s.seenCounter[key]++
	return s.seenCounter[key] == slash.UnavailabilityInConsecutiveBlockSigning
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
		return int64(len(s.prepare))
	case Commit:
		return int64(len(s.commit))
	case ViewChange:
		return int64(len(s.viewID))
	default:
		return 0

	}
}

func (s *cIdentities) AddSignature(p Phase, PubKey *bls.PublicKey, sig *bls.Sign) {
	hex := PubKey.SerializeToHexStr()
	switch p {
	case Prepare:
		s.prepare[hex] = sig
	case Commit:
		s.commit[hex] = sig
	case ViewChange:
		s.viewID[hex] = sig
	}
}

func (s *cIdentities) Reset(ps []Phase) {
	for i := range ps {
		switch m := map[string]*bls.Sign{}; ps[i] {
		case Prepare:
			s.prepare = m
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
	m := map[string]*bls.Sign{}
	hex := PubKey.SerializeToHexStr()

	switch p {
	case Prepare:
		m = s.prepare
	case Commit:
		m = s.commit
	case ViewChange:
		m = s.viewID
	}

	payload, ok := m[hex]
	if !ok {
		return nil
	}
	return payload
}

func (s *cIdentities) ReadAllSignatures(p Phase) []*bls.Sign {
	m := map[string]*bls.Sign{}
	switch p {
	case Prepare:
		m = s.prepare
	case Commit:
		m = s.commit
	case ViewChange:
		m = s.viewID
	}
	sigs := make([]*bls.Sign, 0, len(m))
	for _, value := range m {
		sigs = append(sigs, value)
	}
	return sigs
}

func newMapBackedSignatureReader() *cIdentities {
	return &cIdentities{
		[]*bls.PublicKey{}, map[string]*bls.Sign{},
		map[string]*bls.Sign{}, map[string]*bls.Sign{},
		map[[shard.PublicKeySizeInBytes]byte]int{},
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
		roster := votepower.NewRoster()
		return &stakedVoteWeight{
			c.SignatureReader,
			c.DependencyInjectionWriter,
			c.DependencyInjectionWriter.(DependencyInjectionReader),
			c.SignatureReader.(slash.ThresholdDecider),
			*roster,
		}
	default:
		// Should not be possible
		return nil
	}
}
