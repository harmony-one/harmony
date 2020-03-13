package apr

import (
	"github.com/ethereum/go-ethereum/common"

	"math/big"
)

// Computed ..
type Computed struct {
	Result *big.Int
}

// ComputeForValidator ..
func ComputeForValidator(addr common.Address) *Computed {
	// estimated_reward_per_year = avg_reward_per_block * blocks_per_year
	// avg_reward_per_block = total_reward_per_epoch / blocks_per_epoch

	// estimated_reward_per_year / total_stake
	return &Computed{nil}
}
