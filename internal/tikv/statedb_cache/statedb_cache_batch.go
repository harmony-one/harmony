package statedb_cache

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

type StateDBCacheBatch struct {
	db          *StateDBCacheDatabase
	remoteBatch ethdb.Batch
}

func newStateDBCacheBatch(db *StateDBCacheDatabase, batch ethdb.Batch) *StateDBCacheBatch {
	return &StateDBCacheBatch{db: db, remoteBatch: batch}
}

func (b *StateDBCacheBatch) Put(key []byte, value []byte) error {
	return b.remoteBatch.Put(key, value)
}

func (b *StateDBCacheBatch) Delete(key []byte) error {
	return b.remoteBatch.Delete(key)
}

func (b *StateDBCacheBatch) ValueSize() int {
	// In ethdb, the size of each commit is too small, this controls the submission frequency
	return b.remoteBatch.ValueSize() / 40
}

func (b *StateDBCacheBatch) Write() (err error) {
	defer func() {
		if err == nil {
			_ = b.db.cacheWrite(b)
		}
	}()

	return b.remoteBatch.Write()
}

func (b *StateDBCacheBatch) Reset() {
	b.remoteBatch.Reset()
}

func (b *StateDBCacheBatch) Replay(w ethdb.KeyValueWriter) error {
	return b.remoteBatch.Replay(w)
}
