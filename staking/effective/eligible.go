package effective

// Eligibility ..
type Eligibility byte

const (
	// Active means allowed in epos auction
	Active Eligibility = iota
	// Inactive means validator did not sign enough over 66%
	// of the time in an epoch and so they are removed from
	// the possibility of being in the epos auction, which happens
	// only once an epoch and only
	// by beaconchain, aka shard.BeaconChainShardID
	Inactive
	// Banned records whether this validator is banned
	// from the network because they double-signed
	// it can never be undo
	Banned
)
