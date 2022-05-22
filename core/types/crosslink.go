package types

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
)

// CrossLink is only used on beacon chain to store the hash links from other shards
// signature and bitmap correspond to |blockNumber|parentHash| byte array
// Capital to enable rlp encoding
// Here we replace header to signatures only, the basic assumption is the committee will not be
// corrupted during one epoch, which is the same as consensus assumption
type CrossLink struct {
	HashF        common.Hash
	BlockNumberF *big.Int
	ViewIDF      *big.Int
	SignatureF   [96]byte //aggregated signature
	BitmapF      []byte   //corresponding bitmap mask for agg signature
	ShardIDF     uint32   //will be verified with signature on |blockNumber|blockHash| is correct
	EpochF       *big.Int
}

// NewCrossLink returns a new cross link object
func NewCrossLink(header *block.Header, parentHeader *block.Header) *CrossLink {
	parentBlockNum := big.NewInt(0)
	parentViewID := big.NewInt(0)
	parentEpoch := big.NewInt(0)
	if parentHeader != nil { // Should always happen, default values to be defensive.
		parentBlockNum = parentHeader.Number()
		parentViewID = parentHeader.ViewID()
		parentEpoch = parentHeader.Epoch()
	}
	return &CrossLink{
		HashF:        header.ParentHash(),
		BlockNumberF: parentBlockNum,
		ViewIDF:      parentViewID,
		SignatureF:   header.LastCommitSignature(),
		BitmapF:      header.LastCommitBitmap(),
		ShardIDF:     header.ShardID(),
		EpochF:       parentEpoch,
	}
}

// ShardID returns shardID
func (cl *CrossLink) ShardID() uint32 {
	return cl.ShardIDF
}

// Number returns blockNum with big.Int format
func (cl *CrossLink) Number() *big.Int {
	return cl.BlockNumberF
}

// ViewID returns viewID with big.Int format
func (cl *CrossLink) ViewID() *big.Int {
	return cl.ViewIDF
}

// Epoch returns epoch with big.Int format
func (cl *CrossLink) Epoch() *big.Int {
	return cl.EpochF
}

// BlockNum returns blockNum
func (cl *CrossLink) BlockNum() uint64 {
	return cl.BlockNumberF.Uint64()
}

// Hash returns hash
func (cl *CrossLink) Hash() common.Hash {
	return cl.HashF
}

// Bitmap returns bitmap
func (cl *CrossLink) Bitmap() []byte {
	return cl.BitmapF
}

// Signature returns aggregated signature
func (cl *CrossLink) Signature() [96]byte {
	return cl.SignatureF
}

// MarshalJSON ..
func (cl *CrossLink) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash        common.Hash `json:"hash"`
		BlockNumber *big.Int    `json:"block-number"`
		ViewID      *big.Int    `json:"view-id"`
		Signature   string      `json:"signature"`
		Bitmap      string      `json:"signature-bitmap"`
		ShardID     uint32      `json:"shard-id"`
		EpochNumber *big.Int    `json:"epoch-number"`
	}{
		cl.HashF,
		cl.BlockNumberF,
		cl.ViewIDF,
		hex.EncodeToString(cl.SignatureF[:]),
		hex.EncodeToString(cl.BitmapF),
		cl.ShardIDF,
		cl.EpochF,
	})
}

// Serialize returns bytes of cross link rlp-encoded content
func (cl *CrossLink) Serialize() []byte {
	bytes, _ := rlp.EncodeToBytes(cl)
	return bytes
}

// DeserializeCrossLink rlp-decode the bytes into cross link object.
func DeserializeCrossLink(bytes []byte) (*CrossLink, error) {
	cl := &CrossLink{}
	err := rlp.DecodeBytes(bytes, cl)
	if err != nil {
		return nil, err
	}
	return cl, err
}

// CrossLinks is a collection of cross links
type CrossLinks []CrossLink

// Sort crosslinks by shardID and then tie break by blockNum then by viewID
func (cls CrossLinks) Sort() {
	sort.Slice(cls, func(i, j int) bool {
		if s1, s2 := cls[i].ShardID(), cls[j].ShardID(); s1 != s2 {
			return s1 < s2
		}
		if s1, s2 := cls[i].Number(), cls[j].Number(); s1.Cmp(s2) != 0 {
			return s1.Cmp(s2) < 0
		}
		return cls[i].ViewID().Cmp(cls[j].ViewID()) < 0
	})
}

// IsSorted checks whether the cross links are sorted
func (cls CrossLinks) IsSorted() bool {
	return sort.SliceIsSorted(cls, func(i, j int) bool {
		if s1, s2 := cls[i].ShardID(), cls[j].ShardID(); s1 != s2 {
			return s1 < s2
		}
		if s1, s2 := cls[i].Number(), cls[j].Number(); s1.Cmp(s2) != 0 {
			return s1.Cmp(s2) < 0
		}
		return cls[i].ViewID().Cmp(cls[j].ViewID()) < 0
	})
}
