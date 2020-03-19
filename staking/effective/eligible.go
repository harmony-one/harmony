package effective

// Eligibility represents ability to participate in EPoS auction
// that occurs just once an epoch on beaconchain
type Eligibility byte

const (
	// Nil is a default state that represents a no-op
	Nil Eligibility = iota
	// Active means allowed in epos auction
	Active
	// Inactive means validator did not sign enough over 66%
	// of the time in an epoch and so they are removed from
	// the possibility of being in the epos auction, which happens
	// only once an epoch and only
	// by beaconchain, aka shard.BeaconChainShardID
	Inactive
	// Banned records whether this validator is banned
	// from the network because they double-signed
	// it can never be undone
	Banned
)

// Candidacy is a more semantically meaningful
// value that is derived from core protocol logic but
// meant more for the presentation of user, like at RPC
type Candidacy byte

const (
	// Unknown ..
	Unknown Candidacy = iota
	// Candidate ..
	Candidate = iota
	// NotCandidate ..
	NotCandidate
	// ElectedAndSigning ..
	ElectedAndSigning
	// ElectedAndFellBelowThreshold ..
	ElectedAndFellBelowThreshold
)

func (c Candidacy) String() string {
	switch c {
	case Candidate:
		return "eligible to be elected next epoch"
	case NotCandidate:
		return "not eligible to be elected next epoch"
	case ElectedAndSigning:
		return "currently elected and signing enough blocks to be eligible for election next epoch"
	case ElectedAndFellBelowThreshold:
		return "currently elected and not signing enough blocks to be eligible for election next epoch"
	default:
		return "unknown"
	}
}

// ValidatorStatus ..
func ValidatorStatus(currentlyInCommittee, isActive bool) Candidacy {
	switch {
	case currentlyInCommittee && isActive:
		return ElectedAndSigning
	case !currentlyInCommittee && isActive:
		return Candidate
	case currentlyInCommittee && !isActive:
		return ElectedAndFellBelowThreshold
	case !currentlyInCommittee && !isActive:
		return NotCandidate
	default:
		return Unknown
	}
}
