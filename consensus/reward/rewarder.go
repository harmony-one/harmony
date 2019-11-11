package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Distributor ..
type Distributor interface {
	Award(
		Pie *big.Int,
		earners []common.Address,
		hook func(earner common.Address, due *big.Int),
	) (payout *big.Int)
}
