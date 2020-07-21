package hmy

import (
	"math/big"
	"sync"
)

// totalStakeCache ..
type totalStakeCache struct {
	mutex       sync.Mutex
	blockHeight int64
	stake       *big.Int
	// duration is in blocks
	duration uint64
}

// TODO: better implementation of GetTotalStakingSnapshot()...
// newTotalStakeCache ..
func newTotalStakeCache(duration uint64) *totalStakeCache {
	return &totalStakeCache{
		mutex:       sync.Mutex{},
		blockHeight: -1,
		stake:       nil,
		duration:    duration,
	}
}
