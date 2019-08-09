package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// CXReceipt represents a receipt for cross-shard transaction
type CXReceipt struct {
	Nonce     uint64
	From      common.Address
	To        common.Address
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
