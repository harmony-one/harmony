package reward

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

// Distributor ..
type Distributor interface {
	Award(
		pie numeric.Dec,
		earners shard.SlotList,
		hook func(earner common.Address, due *big.Int),
	) numeric.Dec
}
