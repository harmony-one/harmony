package explorer

import (
	"path"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	tikvCommon "github.com/harmony-one/harmony/internal/tikv/common"
	"github.com/harmony-one/harmony/internal/tikv/prefix"
	"github.com/harmony-one/harmony/internal/tikv/remote"
)

// database is an adapter for *leveldb.DB
type database interface {
	databaseWriter
	databaseReader
	NewBatch() batch
}

type databaseWriter interface {
	Put(key, val []byte) error
}

type databaseReader interface {
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	NewPrefixIterator(prefix []byte) iterator
	NewSizedIterator(start []byte, size int) iterator
}

type batch interface {
	databaseWriter
	Write() error
	ValueSize() int
}

// explorerDB is the adapter for explorer node
type explorerDB struct {
	db ethdb.KeyValueStore
}

// newExplorerLvlDB new explorer storage using leveldb
func newExplorerLvlDB(dbPath string) (database, error) {
	db, err := leveldb.New(dbPath, 16, 500, "explorer_db", false)
	if err != nil {
		return nil, err
	}
	return &explorerDB{db}, nil
}

// newExplorerTiKv new explorer storage using leveldb
func newExplorerTiKv(pdAddr []string, dbPath string, readOnly bool) (database, error) {
	prefixStr := append([]byte(path.Base(dbPath)), '/')
	db, err := remote.NewRemoteDatabase(pdAddr, readOnly)
	if err != nil {
		return nil, err
	}
	return &explorerDB{
		db: tikvCommon.ToEthKeyValueStore(
			prefix.NewPrefixDatabase(prefixStr, db),
		),
	}, nil
}

func (db *explorerDB) Put(key, val []byte) error {
	return db.db.Put(key, val)
}

func (db *explorerDB) Get(key []byte) ([]byte, error) {
	return db.db.Get(key)
}

func (db *explorerDB) Has(key []byte) (bool, error) {
	return db.db.Has(key)
}

func (db *explorerDB) NewBatch() batch {
	return db.db.NewBatch()
}

func (db *explorerDB) NewPrefixIterator(prefix []byte) iterator {
	it := db.db.NewIterator(prefix, nil)
	return it
}

func (db *explorerDB) NewSizedIterator(start []byte, size int) iterator {
	return db.newSizedIterator(start, size)
}

type sizedIterator struct {
	it        iterator
	curIndex  int
	sizeLimit int
}

func (db *explorerDB) newSizedIterator(start []byte, size int) *sizedIterator {
	it := db.db.NewIterator(nil, start)
	return &sizedIterator{
		it:        it,
		curIndex:  0,
		sizeLimit: size,
	}
}

func (it *sizedIterator) Next() bool {
	if it.curIndex >= it.sizeLimit-1 {
		return false
	}
	it.curIndex++
	return it.it.Next()
}

func (it *sizedIterator) Key() []byte   { return it.it.Key() }
func (it *sizedIterator) Value() []byte { return it.it.Value() }
func (it *sizedIterator) Release()      { it.it.Release() }
func (it *sizedIterator) Error() error  { return it.it.Error() }

type iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Release()
	Error() error
}

// blockChainTxIndexer is the interface to check the loop up entry for transaction.
// Implemented by core.BlockChain
type blockChainTxIndexer interface {
	ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64)
}
