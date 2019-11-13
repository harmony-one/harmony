package slash

import "github.com/harmony-one/harmony/shard"

const (
	// UnavailabilityInConsecutiveBlockSigning is how many blocks in a row
	// before "slashing by unavailability" occurs
	UnavailabilityInConsecutiveBlockSigning = 1380
)

// Slasher ..
type Slasher interface {
	ShouldSlash(shard.BlsPublicKey) bool
}

// ThresholdDecider ..
type ThresholdDecider interface {
	SlashThresholdMet(shard.BlsPublicKey) bool
}
