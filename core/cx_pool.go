package core

import (
	mapset "github.com/deckarep/golang-set"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// CxPoolSize is the maximum size of the pool
	CxPoolSize = 50
)

// CxEntry represents the egress receipt's blockHash and ToShardID
type CxEntry struct {
	BlockHash common.Hash
	ToShardID uint32
}

// CxPool is to hold a pool of block outgoing receipts to be resend in next round broadcast
// When a user/client doesn't find the destination shard get the money from cross shard tx
// it can send RPC call along with txID to allow the any validator to
// add the corresponding block's receipts to be resent
type CxPool struct {
	pool    mapset.Set
	maxSize int
}

// NewCxPool creates a new CxPool
func NewCxPool(limit int) *CxPool {
	pool := mapset.NewSet()
	cxPool := CxPool{pool: pool, maxSize: limit}
	return &cxPool
}

// Pool returns the pool of blockHashes of missing receipts
func (cxPool *CxPool) Pool() mapset.Set {
	return cxPool.pool
}

// Size return size of the pool
func (cxPool *CxPool) Size() int {
	return cxPool.pool.Cardinality()
}

// Add add element into the pool if not exceed limit
func (cxPool *CxPool) Add(entry CxEntry) bool {
	if cxPool.Size() > cxPool.maxSize {
		return false
	}
	cxPool.pool.Add(entry)
	return true
}

// Clear empty the pool
func (cxPool *CxPool) Clear() {
	cxPool.pool.Clear()
}
