package remote

import (
	"bytes"
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/tikv/common"
)

type operate int

const (
	_ operate = iota
	operatePut
	operateDelete
)

type valueInfo struct {
	operate  operate
	key, val []byte
}

type RemoteBatch struct {
	lock sync.Mutex

	db *RemoteDatabase

	size   int
	valMap map[string]*valueInfo
}

func newRemoteBatch(db *RemoteDatabase) *RemoteBatch {
	return &RemoteBatch{
		db:     db,
		valMap: make(map[string]*valueInfo),
	}
}

func (b *RemoteBatch) copy(val []byte) []byte {
	tmp := make([]byte, len(val))
	copy(tmp, val)
	return tmp
}

// Put inserts the given value into the key-value data store.
func (b *RemoteBatch) Put(key []byte, value []byte) error {
	if len(key) == 0 {
		return common.ErrEmptyKey
	}

	if len(value) == 0 {
		value = EmptyValueStub
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.valMap[string(key)] = &valueInfo{
		operate: operatePut,
		key:     b.copy(key),
		val:     b.copy(value),
	}

	b.size += len(key) + len(value)
	return nil
}

// Delete removes the key from the key-value data store.
func (b *RemoteBatch) Delete(key []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.valMap[string(key)] = &valueInfo{
		operate: operateDelete,
		key:     b.copy(key),
	}

	b.size += len(key)
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *RemoteBatch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to disk.
func (b *RemoteBatch) Write() error {
	b.lock.Lock()
	defer b.lock.Unlock()

	batchWriteKey := make([][]byte, 0)
	batchWriteValue := make([][]byte, 0)
	batchDeleteKey := make([][]byte, 0)

	for _, info := range b.valMap {
		switch info.operate {
		case operatePut:
			batchWriteKey = append(batchWriteKey, info.key)
			batchWriteValue = append(batchWriteValue, info.val)
		case operateDelete:
			batchDeleteKey = append(batchDeleteKey, info.key)
		}
	}

	if len(batchWriteKey) > 0 {
		err := b.db.client.BatchPut(context.Background(), batchWriteKey, batchWriteValue)
		if err != nil {
			return err
		}
	}

	if len(batchDeleteKey) > 0 {
		err := b.db.client.BatchDelete(context.Background(), batchDeleteKey)
		if err != nil {
			return err
		}
	}

	return nil
}

// Reset resets the batch for reuse.
func (b *RemoteBatch) Reset() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.valMap = make(map[string]*valueInfo)
	b.size = 0
}

// Replay replays the batch contents.
func (b *RemoteBatch) Replay(w ethdb.KeyValueWriter) error {
	var err error
	for _, info := range b.valMap {
		switch info.operate {
		case operatePut:
			if bytes.Compare(info.val, EmptyValueStub) == 0 {
				err = w.Put(info.key, []byte{})
			} else {
				err = w.Put(info.key, info.val)
			}
		case operateDelete:
			err = w.Delete(info.key)
		}

		if err != nil {
			return err
		}
	}

	return nil
}
