package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// CXMessage contains information about a cross-shard message.
type CXMessage struct {
	To, From                      common.Address
	ToShard, FromShard            uint32
	Payload                       []byte
	GasBudget, GasPrice, GasLimit *big.Int
	GasLeftoverTo                 common.Address
	Nonce                         uint64
	Value                         *big.Int
}
