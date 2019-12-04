package slash

import (
	"math/big"

	"github.com/harmony-one/harmony/shard"
)

var (
	// MissedThresholdForInactive ..
	MissedThresholdForInactive = big.NewInt(int64(shard.Schedule.BlocksPerEpoch()))
)

// Slasher ..
type Slasher interface {
	ShouldSlash(shard.BlsPublicKey) bool
}

// ThresholdDecider ..
type ThresholdDecider interface {
	SlashThresholdMet(shard.BlsPublicKey) bool
}
