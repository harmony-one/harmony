package chainconfig

import (
	"math/big"
)

var (
	// WaitlistOneBlock is the block height to fork for waitlist one
	WaitlistOneBlock = big.NewInt(100)
	// WaitlistTwoBlock is the block height to fork for waitlist two
	WaitlistTwoBlock = big.NewInt(200)
)

// ChainConfig contains the critical config of the blockchain
type ChainConfig struct {
	WaitlistOneBlock big.Int
}

// isForked returns whether a fork scheduled at block s is active at the given head block.
func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
}

// IsWaitlistOneBlock returns whether n is the WaitlistOneBlock
func (c *ChainConfig) IsWaitlistOneBlock(n *big.Int) bool {
	return isForked(c.WaitlistOneBlock, n)
}
