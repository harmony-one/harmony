package quorum

import (
	"github.com/harmony-one/bls/ffi/go/bls"
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

// Policy is the rule we used to decide is quorum achieved
type Policy byte

const (
	// SuperMajorityVote is a 2/3s voting mechanism, pre-PoS
	SuperMajorityVote Policy = iota
	// SuperMajorityStake is 2/3s of total staked amount for epoch
	SuperMajorityStake
)

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
	SignatoriesCount(Phase) int64
	Reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	SignatoryTracker
	ReadAllSignatures(Phase) []*bls.Sign
	ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign
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
	viewID map[string]*bls.Sign
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
	s.publicKeys = append(pubKeys[:0:0], pubKeys...)
}

func (s *cIdentities) DumpParticipants() []string {
	keys := make([]string, len(s.publicKeys))
	for i := 0; i < len(s.publicKeys); i++ {
		keys[i] = s.publicKeys[i].SerializeToHexStr()
	}
	return keys
}

func (s *cIdentities) ParticipantsCount() int64 {
	return int64(len(s.publicKeys))
}

func (s *cIdentities) SignatoriesCount(p Phase) int64 {
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
	for _, p := range ps {
		switch m := map[string]*bls.Sign{}; p {
		case Prepare:
			s.prepare = m
		case Commit:
			s.commit = m
		case ViewChange:
			s.viewID = m
		}
	}
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
	sigs := []*bls.Sign{}
	m := map[string]*bls.Sign{}

	switch p {
	case Prepare:
		m = s.prepare
	case Commit:
		m = s.commit
	case ViewChange:
		m = s.viewID
	}

	for _, sig := range m {
		sigs = append(sigs, sig)
	}
	return sigs
}

func newMapBackedSignatureReader() SignatureReader {
	return &cIdentities{
		[]*bls.PublicKey{}, map[string]*bls.Sign{},
		map[string]*bls.Sign{}, map[string]*bls.Sign{},
	}
}

// Decider ..
type Decider interface {
	SignatureReader
	Policy() Policy
	IsQuorumAchieved(Phase) bool
	QuorumThreshold() int64
	IsRewardThresholdAchieved() bool
}

type uniformVoteWeight struct {
	SignatureReader
}

// NewDecider ..
func NewDecider(p Policy) Decider {
	switch p {
	case SuperMajorityVote:
		return &uniformVoteWeight{newMapBackedSignatureReader()}
	// case SuperMajorityStake:
	default:
		// Should not be possible
		return nil
	}
}

// Policy ..
func (v *uniformVoteWeight) Policy() Policy {
	return SuperMajorityVote
}

// IsQuorumAchieved ..
func (v *uniformVoteWeight) IsQuorumAchieved(p Phase) bool {
	return v.SignatoriesCount(p) >= v.QuorumThreshold()
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() int64 {
	return v.ParticipantsCount()*2/3 + 1
}

// RewardThreshold ..
func (v *uniformVoteWeight) IsRewardThresholdAchieved() bool {
	return v.SignatoriesCount(Commit) >= (v.ParticipantsCount() * 9 / 10)
}
