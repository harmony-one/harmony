package effective

// Eligibility represents ability to participate in EPoS auction
// that occurs just once an epoch on beaconchain
type Eligibility byte

const (
	// Nil is a default state that represents a no-op
	Nil Eligibility = iota
	// FirstTimeCandidate ..
	FirstTimeCandidate
	// Candidate means allowed in epos auction
	Candidate
	// InCommitteeAndSigning means already won a spot in epos auction
	InCommitteeAndSigning
	// InCommitteeAndFellBelowThreshold ..
	InCommitteeAndFellBelowThreshold
	// NoCandidacy set by protocol means validator
	// did not sign enough over 66%
	// of the time in an epoch and so they are removed from
	// the possibility of being in the epos auction, which happens
	// only once an epoch and only
	// by beaconchain, aka shard.BeaconChainShardID.
	// NoCandidacy can also be set by a staking transaction, to remove oneself
	// from the eligible pool of candidates for EPoS auction
	NoCandidacy
	// BannedForever records whether this validator is banned
	// from the network because they double-signed
	// it can never be undone
	BannedForever
)

func (e Eligibility) String() string {
	switch e {
	case FirstTimeCandidate:
		return "first time created validator that is eligible for upcoming EPoS auction"
	case Candidate:
		return "eligible for slot in upcoming epoch's EPoS auction"
	case InCommitteeAndSigning:
		return "in committee and signing a sufficient amount of blocks"
	case InCommitteeAndFellBelowThreshold:
		return "in committee but current signing threshold jeopardizes EPoS candidacy in next epoch"
	case NoCandidacy:
		return "lost EPoS candidacy due to missed signing threshold in previous epoch(s)"
	case BannedForever:
		return "banned from network forever because of a recorded double signing"
	default:
		return "nil"
	}
}
