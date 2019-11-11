package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
)

// Distributor ..
type Distributor interface {
	Award(
		Pie *big.Int,
		earners []common2.Address,
		hook func(earner common.Address, due *big.Int),
	) (payout *big.Int)
}
