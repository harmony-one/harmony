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

// Tracker ..
type Tracker interface {
	AddSignature(p Phase, PubKey *bls.PublicKey, sig *bls.Sign)
	SignatoriesCount(Phase) int64
	Reset([]Phase)
}

// SignatureReader ..
type SignatureReader interface {
	Tracker
	ReadAllSignatures(Phase) []*bls.Sign
	ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign
}

// These maps represent the signatories (validators), keys are BLS public keys
// and values are BLS private key signed signatures
type mapImpl struct {
	prepare map[string]*bls.Sign
	commit  map[string]*bls.Sign
	// viewIDSigs: every validator
	// sign on |viewID|blockHash| in view changing message
	viewID map[string]*bls.Sign
}

func (s *mapImpl) SignatoriesCount(p Phase) int64 {
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

func (s *mapImpl) AddSignature(p Phase, PubKey *bls.PublicKey, sig *bls.Sign) {
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

func (s *mapImpl) Reset(ps []Phase) {
	for _, p := range ps {
		switch p {
		case Prepare:
			s.prepare = map[string]*bls.Sign{}
		case Commit:
			s.commit = map[string]*bls.Sign{}
		case ViewChange:
		}
	}
}

func (s *mapImpl) ReadSignature(p Phase, PubKey *bls.PublicKey) *bls.Sign {
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

func (s *mapImpl) ReadAllSignatures(p Phase) []*bls.Sign {
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
	return &mapImpl{
		map[string]*bls.Sign{}, map[string]*bls.Sign{}, map[string]*bls.Sign{},
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
	return true
}

// QuorumThreshold ..
func (v *uniformVoteWeight) QuorumThreshold() int64 {
	return 0
}

// RewardThreshold ..
func (v *uniformVoteWeight) IsRewardThresholdAchieved() bool {
	return true
}

// func New() {
// 	return &{

// 	}
// }

// type DecideByStake struct{}

// func (s *DecideByStake) Mechanism() Mechanism {
// 	return SuperMajorityStake
// }

// func (s *DecideByStake) IsSatisfied(sigs, constituents int) bool {

// }

// func (s *DecideByStake) QuorumThreshold(constituents int) int {

// }

// func (s *DecideByStake) RewardThreshold(constituents int) int {

// }
