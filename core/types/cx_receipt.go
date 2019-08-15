package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// CXReceipt represents a receipt for cross-shard transaction
type CXReceipt struct {
	TxHash    common.Hash // hash of the cross shard transaction in source shard
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
	if len(cs) == 0 {
		return []byte{}
	}
	enc, _ := rlp.EncodeToBytes(cs[i])
	return enc
}

// ToShardID returns the destination shardID of the cxReceipt
func (cs CXReceipts) ToShardID(i int) uint32 {
	if len(cs) == 0 {
		return 0
	}
	return cs[i].ToShardID
}

// MaxToShardID returns the maximum destination shardID of cxReceipts
func (cs CXReceipts) MaxToShardID() uint32 {
	maxShardID := uint32(0)
	if len(cs) == 0 {
		return maxShardID
	}
	for i := 0; i < len(cs); i++ {
		if maxShardID < cs[i].ToShardID {
			maxShardID = cs[i].ToShardID
		}
	}
	return maxShardID
}

// NewCrossShardReceipt creates a cross shard receipt
func NewCrossShardReceipt(txHash common.Hash, from common.Address, to *common.Address, shardID uint32, toShardID uint32, amount *big.Int) *CXReceipt {
	return &CXReceipt{TxHash: txHash, From: from, To: to, ShardID: shardID, ToShardID: toShardID, Amount: amount}
}

// CXMerkleProof represents the merkle proof of a collection of ordered cross shard transactions
type CXMerkleProof struct {
	BlockNum      *big.Int      // block header's hash
	BlockHash     common.Hash   // block header's Hash
	ShardID       uint32        // block header's shardID
	CXReceiptHash common.Hash   // root hash of the cross shard receipts in a given block
	ShardIDs      []uint32      // order list, records destination shardID
	CXShardHashes []common.Hash // ordered hash list, each hash corresponds to one destination shard's receipts root hash
}

// CalculateIncomingReceiptsHash calculates the incoming receipts list hash
// the list is already sorted by shardID and then by blockNum before calling this function
// or the list is from the block field which is already sorted
func CalculateIncomingReceiptsHash(receiptsList []CXReceipts) common.Hash {
	if len(receiptsList) == 0 {
		return EmptyRootHash
	}

	incomingReceipts := CXReceipts{}
	for _, receipts := range receiptsList {
		incomingReceipts = append(incomingReceipts, receipts...)
	}

	return DeriveSha(incomingReceipts)
}
