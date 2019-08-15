package types

import (
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/ethereum/go-ethereum/common"
)

// CrossLink is only used on beacon chain to store the hash links from other shards
type CrossLink struct {
	ChainHeader *Header
}

// NewCrossLink returns a new cross link object
func NewCrossLink(header *Header) CrossLink {
	return CrossLink{header}
}

// Header returns header
func (cl CrossLink) Header() *Header {
	return cl.ChainHeader
}

// ShardID returns shardID
func (cl CrossLink) ShardID() uint32 {
	return cl.ChainHeader.ShardID
}

// BlockNum returns blockNum
func (cl CrossLink) BlockNum() *big.Int {
	return cl.ChainHeader.Number
}

// Hash returns hash
func (cl CrossLink) Hash() common.Hash {
	return cl.ChainHeader.Hash()
}

// StateRoot returns hash of state root
func (cl CrossLink) StateRoot() common.Hash {
	return cl.ChainHeader.Root
}

// CxReceiptsRoot returns hash of cross shard receipts
func (cl CrossLink) OutgoingReceiptsRoot() common.Hash {
	return cl.ChainHeader.OutgoingReceiptHash
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
		return cls[i].ShardID() < cls[j].ShardID() || (cls[i].ShardID() == cls[j].ShardID() && cls[i].BlockNum().Cmp(cls[j].BlockNum()) < 0)
	})
}
