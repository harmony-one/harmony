package ethdb_locker

import (
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
)

var _ ethdb.Batch = &RWLockBatchWrapper{}

type RWLockBatchWrapper struct {
	batch ethdb.Batch
	lock  sync.Locker
}

func NewRWLockBatchWrapper(batch ethdb.Batch, lock sync.Locker) *RWLockBatchWrapper {
	return &RWLockBatchWrapper{
		batch: batch,
		lock:  lock,
	}
}

func (r *RWLockBatchWrapper) Put(key []byte, value []byte) error {
	return r.batch.Put(key, value)
}

func (r *RWLockBatchWrapper) Delete(key []byte) error {
	return r.batch.Delete(key)
}

func (r *RWLockBatchWrapper) ValueSize() int {
	return r.batch.ValueSize()
}

func (r *RWLockBatchWrapper) Write() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.batch.Write()
}

func (r *RWLockBatchWrapper) Reset() {
	r.batch.Reset()
}

func (r *RWLockBatchWrapper) Replay(w ethdb.KeyValueWriter) error {
	return r.batch.Replay(w)
}
