package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
)

// Distributor ..
type Distributor interface {
	Award(
		pie numeric.Dec,
		earners []common.Address,
		hook func(earner common.Address, due *big.Int),
	) numeric.Dec
}
