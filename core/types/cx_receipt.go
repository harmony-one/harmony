package types

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/ctxerror"
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

// CXReceiptsProof carrys the cross shard receipts and merkle proof
type CXReceiptsProof struct {
	Receipts    CXReceipts
	MerkleProof *CXMerkleProof
}

// CXReceiptsProofs is a list of CXReceiptsProof
type CXReceiptsProofs []*CXReceiptsProof

// Len returns the length of s.
func (cs CXReceiptsProofs) Len() int { return len(cs) }

// Swap swaps the i'th and the j'th element in s.
func (cs CXReceiptsProofs) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (cs CXReceiptsProofs) GetRlp(i int) []byte {
	if len(cs) == 0 {
		return []byte{}
	}
	enc, _ := rlp.EncodeToBytes(cs[i])
	return enc
}

// ToShardID returns the destination shardID of the cxReceipt
// Not used
func (cs CXReceiptsProofs) ToShardID(i int) uint32 {
	return 0
}

// MaxToShardID returns the maximum destination shardID of cxReceipts
// Not used
func (cs CXReceiptsProofs) MaxToShardID() uint32 {
	return 0
}

// GetToShardID get the destination shardID, return error if there is more than one unique shardID
func (cxp *CXReceiptsProof) GetToShardID() (uint32, error) {
	var shardID uint32
	if cxp == nil || len(cxp.Receipts) == 0 {
		return uint32(0), ctxerror.New("[GetShardID] CXReceiptsProof or its receipts is NIL")
	}
	for i, cx := range cxp.Receipts {
		if i == 0 {
			shardID = cx.ToShardID
		} else if shardID == cx.ToShardID {
			continue
		} else {
			return shardID, ctxerror.New("[GetShardID] CXReceiptsProof contains distinct ToShardID")
		}
	}
	return shardID, nil
}

// IsValidCXReceiptsProof checks whether the given CXReceiptsProof is consistency with itself
// Remaining to check whether there is a corresonding block finalized
func (cxp *CXReceiptsProof) IsValidCXReceiptsProof() error {
	toShardID, err := cxp.GetToShardID()
	if err != nil {
		return ctxerror.New("[IsValidCXReceiptsProof] invalid shardID").WithCause(err)
	}

	merkleProof := cxp.MerkleProof
	shardRoot := common.Hash{}
	foundMatchingShardID := false
	byteBuffer := bytes.NewBuffer([]byte{})

	// prepare to calculate source shard outgoing cxreceipts root hash
	for j := 0; j < len(merkleProof.ShardIDs); j++ {
		sKey := make([]byte, 4)
		binary.BigEndian.PutUint32(sKey, merkleProof.ShardIDs[j])
		byteBuffer.Write(sKey)
		byteBuffer.Write(merkleProof.CXShardHashes[j][:])
		if merkleProof.ShardIDs[j] == toShardID {
			shardRoot = merkleProof.CXShardHashes[j]
			foundMatchingShardID = true
		}
	}

	if !foundMatchingShardID {
		return ctxerror.New("[IsValidCXReceiptsProof] Didn't find matching shardID")
	}

	sourceShardID := merkleProof.ShardID
	sourceBlockNum := merkleProof.BlockNum
	sourceOutgoingCXReceiptsHash := merkleProof.CXReceiptHash

	sha := DeriveSha(cxp.Receipts)

	// (1) verify the CXReceipts trie root match
	if sha != shardRoot {
		return ctxerror.New("[IsValidCXReceiptsProof] Trie Root of ReadCXReceipts Not Match", "sourceShardID", sourceShardID, "sourceBlockNum", sourceBlockNum, "calculated", sha, "got", shardRoot)
	}

	// (2) verify the outgoingCXReceiptsHash match
	outgoingHashFromSourceShard := crypto.Keccak256Hash(byteBuffer.Bytes())
	if outgoingHashFromSourceShard != sourceOutgoingCXReceiptsHash {
		return ctxerror.New("[ProcessReceiptMessage] IncomingReceiptRootHash from source shard not match", "sourceShardID", sourceShardID, "sourceBlockNum", sourceBlockNum, "calculated", outgoingHashFromSourceShard, "got", sourceOutgoingCXReceiptsHash)
	}

	return nil
}
