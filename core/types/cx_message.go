package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// CXMessage contains information about a cross-shard message.
type CXMessage struct {
	To, From           common.Address
	ToShard, FromShard uint32
	Value              *big.Int

	CXMessageReceipt
}

func (m CXMessage) ToReceipt(txHash common.Hash) CXReceipt {
	return CXReceipt{
		TxHash:         txHash,
		From:           m.From,
		To:             &m.To,
		ShardID:        m.FromShard,
		ToShardID:      m.ToShard,
		Amount:         m.Value,
		MessageReceipt: &m.CXMessageReceipt,
	}
}

// CXMessageReceipt contains information needed in a CXReceipt that
// is for a cross shard message, rather than just a balance transfer.
type CXMessageReceipt struct {
	Payload                       []byte
	GasBudget, GasPrice, GasLimit *big.Int
	GasLeftoverTo                 common.Address
	Nonce                         uint64
}
