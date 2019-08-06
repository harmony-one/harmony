package types

import (
	"sort"

	"github.com/ethereum/go-ethereum/common"
)

// CrossLink is only used on beacon chain to store the hash links from other shards
type CrossLink struct {
	shardID  uint32
	blockNum uint64
	hash     common.Hash
}

// ShardID returns shardID
func (cl CrossLink) ShardID() uint32 {
	return cl.shardID
}

// BlockNum returns blockNum
func (cl CrossLink) BlockNum() uint64 {
	return cl.blockNum
}

// Hash returns hash
func (cl CrossLink) Hash() common.Hash {
	return cl.hash
}

// Bytes returns bytes of the hash
func (cl CrossLink) Bytes() []byte {
	return cl.hash[:]
}

// CrossLinks is a collection of cross links
type CrossLinks []CrossLink

// Sort crosslinks by shardID and then by blockNum
func (cls CrossLinks) Sort() {
	sort.Slice(cls, func(i, j int) bool {
		return cls[i].shardID < cls[j].shardID || (cls[i].shardID == cls[j].shardID && cls[i].blockNum < cls[j].blockNum)
	})
}
