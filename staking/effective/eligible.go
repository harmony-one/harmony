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

const (
	eAndFell = "elected in current epoch committee " +
		"but sustained current rate will evict epos candidacy"
)

func (c Candidacy) String() string {
	switch c {
	case Candidate:
		return "candidate for upcoming epoch's epos-auction"
	case NotCandidate:
		return "not a valid candidate for upcoming epoch's epos-auction"
	case ElectedAndSigning:
		return "elected in current epoch committee and signing above required epos threshold"
	case ElectedAndFellBelowThreshold:
		return eAndFell
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
