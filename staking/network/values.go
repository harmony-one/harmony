package network

import "math/big"

// Reward ..
type Reward struct {
	AccumulatorSnapshot     *big.Int `json:"block-reward-accumulator"`
	CurrentStakedPercentage *big.Int `json:"block-reward-accumulator"`
}
