package types

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
)

// CrossLink is only used on beacon chain to store the hash links from other shards
// signature and bitmap correspond to |blockNumber|parentHash| byte array
// Captial to enable rlp encoding
// Here we replace header to signatures only, the basic assumption is the committee will not be
// corrupted during one epoch, which is the same as consensus assumption
type CrossLink struct {
	HashF        common.Hash
	BlockNumberF *big.Int
	SignatureF   [96]byte //aggregated signature
	BitmapF      []byte   //corresponding bitmap mask for agg signature
	ShardIDF     uint32   //will be verified with signature on |blockNumber|blockHash| is correct
	EpochF       *big.Int
}

// NewCrossLink returns a new cross link object
// epoch is the parentHeader's epoch
func NewCrossLink(header *block.Header, epoch *big.Int) CrossLink {
	parentBlockNum := big.NewInt(0)
	if header.Number().Uint64() == 0 { // should not happend, just to be defensive
		return CrossLink{header.ParentHash(), parentBlockNum, header.LastCommitSignature(), header.LastCommitBitmap(), header.ShardID(), epoch}
	}
	parentBlockNum.Sub(header.Number(), big.NewInt(1))
	return CrossLink{header.ParentHash(), parentBlockNum, header.LastCommitSignature(), header.LastCommitBitmap(), header.ShardID(), epoch}
}

// ShardID returns shardID
func (cl CrossLink) ShardID() uint32 {
	return cl.ShardIDF
}

// Number returns blockNum with big.Int format
func (cl CrossLink) Number() *big.Int {
	return cl.BlockNumberF
}

// Epoch returns epoch with big.Int format
func (cl CrossLink) Epoch() *big.Int {
	return cl.EpochF
}

// BlockNum returns blockNum
func (cl CrossLink) BlockNum() uint64 {
	return cl.BlockNumberF.Uint64()
}

// Hash returns hash
func (cl CrossLink) Hash() common.Hash {
	return cl.HashF
}

// Bitmap returns bitmap
func (cl CrossLink) Bitmap() []byte {
	return cl.BitmapF
}

// Signature returns aggregated signature
func (cl CrossLink) Signature() [96]byte {
	return cl.SignatureF
}

// Serialize returns bytes of cross link rlp-encoded content
func (cl CrossLink) Serialize() []byte {
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

// Sort crosslinks by shardID and then by blockNum
func (cls CrossLinks) Sort() {
	sort.Slice(cls, func(i, j int) bool {
		return cls[i].ShardID() < cls[j].ShardID() || (cls[i].ShardID() == cls[j].ShardID() && cls[i].Number().Cmp(cls[j].Number()) < 0)
	})
}

// IsSorted checks whether the cross links are sorted
func (cls CrossLinks) IsSorted() bool {
	return sort.SliceIsSorted(cls, func(i, j int) bool {
		return cls[i].ShardID() < cls[j].ShardID() || (cls[i].ShardID() == cls[j].ShardID() && cls[i].Number().Cmp(cls[j].Number()) < 0)
	})
}
