package common

import (
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

type setRawStakeHack interface {
	SetRawStake(key shard.BLSPublicKey, d numeric.Dec)
}

// SetRawStake is a hack, return value is if was successful or not at setting
func SetRawStake(q quorum.Decider, key shard.BLSPublicKey, d numeric.Dec) bool {
	if setter, ok := q.(setRawStakeHack); ok {
		setter.SetRawStake(key, d)
		return true
	}
	return false
}
