package core

import (
	mapset "github.com/deckarep/golang-set"

	"github.com/ethereum/go-ethereum/common"
)

const (
	CxPoolSize = 50
)

type CxPool struct {
	pool    mapset.Set
	maxSize int
}

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
func (cxPool *CxPool) Add(hash common.Hash) bool {
	if cxPool.Size() > cxPool.maxSize {
		return false
	}
	cxPool.pool.Add(hash)
	return true
}

// Clear empty the pool
func (cxPool *CxPool) Clear() {
	cxPool.pool.Clear()
}
