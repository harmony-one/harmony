package local_cache

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
)

type LocalCacheBatch struct {
	db   *LocalCacheDatabase
	lock sync.Mutex

	size            int
	batchWriteKey   [][]byte
	batchWriteValue [][]byte
	batchDeleteKey  [][]byte
}

func newLocalCacheBatch(db *LocalCacheDatabase) *LocalCacheBatch {
	return &LocalCacheBatch{db: db}
}

func (b *LocalCacheBatch) Put(key []byte, value []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchWriteKey = append(b.batchWriteKey, key)
	b.batchWriteValue = append(b.batchWriteValue, value)
	b.size += len(key) + len(value)
	return nil
}

func (b *LocalCacheBatch) Delete(key []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchDeleteKey = append(b.batchDeleteKey, key)
	b.size += len(key)
	return nil
}

func (b *LocalCacheBatch) ValueSize() int {
	return b.size
}

func (b *LocalCacheBatch) Write() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	return b.db.batchWrite(b)
}

func (b *LocalCacheBatch) Reset() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchWriteKey = b.batchWriteKey[:0]
	b.batchWriteValue = b.batchWriteValue[:0]
	b.batchDeleteKey = b.batchDeleteKey[:0]
	b.size = 0
}

func (b *LocalCacheBatch) Replay(w ethdb.KeyValueWriter) error {
	if len(b.batchWriteKey) > 0 {
		for i, key := range b.batchWriteKey {
			err := w.Put(key, b.batchWriteValue[i])
			if err != nil {
				return err
			}
		}
	}

	if len(b.batchDeleteKey) > 0 {
		for _, key := range b.batchDeleteKey {
			err := w.Delete(key)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
