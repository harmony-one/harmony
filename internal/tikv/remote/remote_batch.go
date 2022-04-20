package remote

import (
	"bytes"
	"context"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/tikv/common"
	"sync"
)

type RemoteBatch struct {
	db   *RemoteDatabase
	lock sync.Mutex

	size            int
	batchWriteKey   [][]byte
	batchWriteValue [][]byte
	batchDeleteKey  [][]byte
}

func newRemoteBatch(db *RemoteDatabase) *RemoteBatch {
	return &RemoteBatch{db: db}
}

func (b *RemoteBatch) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return common.ErrEmptyKey
	}

	if len(value) == 0 {
		value = EmptyValueStub
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchWriteKey = append(b.batchWriteKey, key)
	b.batchWriteValue = append(b.batchWriteValue, value)
	b.size += len(key) + len(value)
	return nil
}

func (b *RemoteBatch) Delete(key []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchDeleteKey = append(b.batchDeleteKey, key)
	b.size += len(key)
	return nil
}

func (b *RemoteBatch) ValueSize() int {
	return b.size
}

func (b *RemoteBatch) Write() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	if len(b.batchWriteKey) > 0 {
		err := b.db.client.BatchPut(context.Background(), b.batchWriteKey, b.batchWriteValue)
		if err != nil {
			return err
		}
	}

	if len(b.batchDeleteKey) > 0 {
		err := b.db.client.BatchDelete(context.Background(), b.batchDeleteKey)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *RemoteBatch) Reset() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.batchWriteKey = b.batchWriteKey[:0]
	b.batchWriteValue = b.batchWriteValue[:0]
	b.batchDeleteKey = b.batchDeleteKey[:0]
	b.size = 0
}

func (b *RemoteBatch) Replay(w ethdb.KeyValueWriter) error {
	for i, key := range b.batchWriteKey {
		if bytes.Compare(b.batchWriteValue[i], EmptyValueStub) == 0 {
			err := w.Put(key, []byte{})
			if err != nil {
				return err
			}
		} else {
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
