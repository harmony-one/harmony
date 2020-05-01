package types

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/pkg/errors"
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

// Copy makes a deep copy of the receiver.
func (r *CXReceipt) Copy() *CXReceipt {
	if r == nil {
		return nil
	}
	cpy := *r
	if cpy.To != nil {
		to := *cpy.To
		cpy.To = &to
	}
	cpy.Amount = new(big.Int).Set(cpy.Amount)
	return &cpy
}

// CXReceipts is a list of CXReceipt
type CXReceipts []*CXReceipt

// Len returns the length of s.
func (cs CXReceipts) Len() int { return len(cs) }

// Copy makes a deep copy of the receiver.
func (cs CXReceipts) Copy() (cpy CXReceipts) {
	for _, r := range cs {
		cpy = append(cpy, r.Copy())
	}
	return cpy
}

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

// ComputeMerkleRoot computes the merkle root of this list of receipts
// The root is the hash of [shardID, receiptsRoot, shardID, receiptsRoot...]
// The receiptRoot is the merkle root hash of the receipts for a specific shards.
func (cs CXReceipts) ComputeMerkleRoot() common.Hash {
	byteBuffer := bytes.NewBuffer([]byte{})
	for i := 0; i <= int(cs.MaxToShardID()); i++ {
		shardReceipts := cs.GetToShardReceipts(uint32(i))
		if len(shardReceipts) != 0 {
			sKey := make([]byte, 4)
			binary.BigEndian.PutUint32(sKey, uint32(i))
			byteBuffer.Write(sKey)
			hash := DeriveSha(shardReceipts)
			byteBuffer.Write(hash[:])
		}
	}
	if byteBuffer.Len() == 0 {
		return EmptyRootHash
	}
	return crypto.Keccak256Hash(byteBuffer.Bytes())
}

// GetToShardReceipts filters the cross shard receipts with given destination shardID
func (cs CXReceipts) GetToShardReceipts(shardID uint32) CXReceipts {
	cxs := CXReceipts{}
	for i := range cs {
		cx := cs[i]
		if cx.ToShardID == shardID {
			cxs = append(cxs, cx)
		}
	}
	return cxs
}

// CXMerkleProof represents the merkle proof of a collection of ordered cross shard transactions
type CXMerkleProof struct {
	BlockNum      *big.Int      // blockNumber of source shard
	BlockHash     common.Hash   // blockHash of source shard
	ShardID       uint32        // shardID of source shard
	CXReceiptHash common.Hash   // root hash of the cross shard receipts in a given block
	ShardIDs      []uint32      // order list, records destination shardID
	CXShardHashes []common.Hash // ordered hash list, each hash corresponds to one destination shard's receipts root hash
}

// Copy makes a deep copy of the receiver.
func (cxmp *CXMerkleProof) Copy() *CXMerkleProof {
	if cxmp == nil {
		return nil
	}
	cpy := *cxmp
	cpy.BlockNum = new(big.Int).Set(cpy.BlockNum)
	cpy.ShardIDs = append(cxmp.ShardIDs[:0:0], cpy.ShardIDs...)
	cpy.CXShardHashes = append(cpy.CXShardHashes[:0:0], cpy.CXShardHashes...)
	return &cpy
}

// CXReceiptsProof carrys the cross shard receipts and merkle proof
type CXReceiptsProof struct {
	Receipts     CXReceipts
	MerkleProof  *CXMerkleProof
	Header       *block.Header
	CommitSig    []byte
	CommitBitmap []byte
}

// Copy makes a deep copy of the receiver.
func (cxp *CXReceiptsProof) Copy() *CXReceiptsProof {
	return &CXReceiptsProof{
		Receipts:     cxp.Receipts.Copy(),
		MerkleProof:  cxp.MerkleProof.Copy(),
		Header:       CopyHeader(cxp.Header),
		CommitSig:    append(cxp.CommitSig[:0:0], cxp.CommitSig...),
		CommitBitmap: append(cxp.CommitBitmap[:0:0], cxp.CommitSig...),
	}
}

// CXReceiptsProofs is a list of CXReceiptsProof
type CXReceiptsProofs []*CXReceiptsProof

// Len returns the length of s.
func (cs CXReceiptsProofs) Len() int { return len(cs) }

// Copy makes a deep copy of the receiver.
func (cs CXReceiptsProofs) Copy() (cpy CXReceiptsProofs) {
	for _, cxrp := range cs {
		cpy = append(cpy, cxrp.Copy())
	}
	return cpy
}

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
		return uint32(0), errors.New("[GetShardID] CXReceiptsProof or its receipts is NIL")
	}
	for i, cx := range cxp.Receipts {
		if i == 0 {
			shardID = cx.ToShardID
		} else if shardID == cx.ToShardID {
			continue
		} else {
			return shardID, errors.New("[GetShardID] CXReceiptsProof contains distinct ToShardID")
		}
	}
	return shardID, nil
}

// ContainsEmptyField checks whether the given CXReceiptsProof contains empty field
func (cxp *CXReceiptsProof) ContainsEmptyField() bool {
	anyNil := cxp == nil ||
		cxp.Receipts == nil ||
		cxp.MerkleProof == nil ||
		cxp.Header == nil
	anyZero := len(cxp.CommitSig)+len(cxp.CommitBitmap) == 0
	return anyNil || anyZero
}
