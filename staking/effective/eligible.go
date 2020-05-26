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

func (e Eligibility) String() string {
	switch e {
	case Active:
		return "active"
	case Inactive:
		return "inactive"
	case Banned:
		return doubleSigningBanned
	default:
		return "unknown"
	}
}

// Candidacy is a more semantically meaningful
// value that is derived from core protocol logic but
// meant more for the presentation of user, like at RPC
type Candidacy byte

const (
	// Unknown ..
	Unknown Candidacy = iota
	// ForeverBanned ..
	ForeverBanned
	// Candidate ..
	Candidate = iota
	// NotCandidate ..
	NotCandidate
	// Elected ..
	Elected
)

const (
	doubleSigningBanned = "banned forever from network because was caught double-signing"
)

func (c Candidacy) String() string {
	switch c {
	case ForeverBanned:
		return doubleSigningBanned
	case Candidate:
		return "eligible to be elected next epoch"
	case NotCandidate:
		return "not eligible to be elected next epoch"
	case Elected:
		return "currently elected"
	default:
		return "unknown"
	}
}

// ValidatorStatus ..
func ValidatorStatus(currentlyInCommittee bool, status Eligibility) Candidacy {
	switch {
	case status == Banned:
		return ForeverBanned
	case currentlyInCommittee:
		return Elected
	case !currentlyInCommittee && status == Active:
		return Candidate
	case !currentlyInCommittee && status != Active:
		return NotCandidate
	default:
		return Unknown
	}
}

// BootedStatus ..
type BootedStatus byte

const (
	// Booted ..
	Booted BootedStatus = iota
	// NotBooted ..
	NotBooted
	// LostEPoSAuction ..
	LostEPoSAuction
	// TurnedInactiveOrInsufficientUptime ..
	TurnedInactiveOrInsufficientUptime
	// BannedForDoubleSigning ..
	BannedForDoubleSigning
)

func (r BootedStatus) String() string {
	switch r {
	case Booted:
		return "booted"
	case LostEPoSAuction:
		return "lost epos auction"
	case TurnedInactiveOrInsufficientUptime:
		return "manually turned inactive or insufficient uptime"
	case BannedForDoubleSigning:
		return doubleSigningBanned
	default:
		return "not booted"
	}
}
