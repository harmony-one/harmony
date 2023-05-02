package common

import (
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type TiKVStore interface {
	ethdb.KeyValueReader
	ethdb.KeyValueWriter
	ethdb.Batcher
	ethdb.Stater
	ethdb.Compacter
	io.Closer

	NewIterator(start, end []byte) ethdb.Iterator
}

// TiKVStoreWrapper simple wrapper to covert to ethdb.KeyValueStore
type TiKVStoreWrapper struct {
	TiKVStore
}

func (t *TiKVStoreWrapper) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return t.TiKVStore.NewIterator(nil, nil)
}

func (t *TiKVStoreWrapper) NewIteratorWithStart(start []byte) ethdb.Iterator {
	return t.TiKVStore.NewIterator(start, nil)
}

func (t *TiKVStoreWrapper) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	bytesPrefix := util.BytesPrefix(prefix)
	return t.TiKVStore.NewIterator(bytesPrefix.Start, bytesPrefix.Limit)
}

func (t *TiKVStoreWrapper) NewSnapshot() (ethdb.Snapshot, error) {
	return nil, errors.New("not supported")
}

func ToEthKeyValueStore(store TiKVStore) ethdb.KeyValueStore {
	return &TiKVStoreWrapper{TiKVStore: store}
}
