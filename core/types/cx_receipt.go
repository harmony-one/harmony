package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// CXReceipt represents a receipt for cross-shard transaction
type CXReceipt struct {
	TxHash    common.Hash // hash of the cross shard transaction in source shard
	Nonce     uint64
	From      common.Address
	To        *common.Address
	ShardID   uint32
	ToShardID uint32
	Amount    *big.Int
}

// CXReceipts is a list of CXReceipt
type CXReceipts []*CXReceipt

// Len returns the length of s.
func (cs CXReceipts) Len() int { return len(cs) }

// Swap swaps the i'th and the j'th element in s.
func (cs CXReceipts) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (cs CXReceipts) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(cs[i])
	return enc
}

// ToShardID returns the destination shardID of the cxReceipt
func (cs CXReceipts) ToShardID(i int) uint32 {
	return cs[i].ToShardID
}

// NewCrossShardReceipt creates a cross shard receipt
func NewCrossShardReceipt(txHash common.Hash, nonce uint64, from common.Address, to *common.Address, shardID uint32, toShardID uint32, amount *big.Int) *CXReceipt {
	return &CXReceipt{TxHash: txHash, Nonce: nonce, From: from, To: to, ShardID: shardID, ToShardID: toShardID, Amount: amount}
}
