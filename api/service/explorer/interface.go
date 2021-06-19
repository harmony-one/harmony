package explorer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	levelutil "github.com/syndtr/goleveldb/leveldb/util"
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

// lvlDB is the adapter for leveldb.Database
type lvlDB struct {
	db *leveldb.DB
}

func newLvlDB(dbPath string) (database, error) {
	// https://github.com/ethereum/go-ethereum/blob/master/ethdb/leveldb/leveldb.go#L98 options.
	// We had 0 for handles and cache params before, so set 0s for all of them. Filter opt is the same.
	options := &opt.Options{
		OpenFilesCacheCapacity: 500,
		BlockCacheCapacity:     8 * 1024 * 1024, // 8 MiB
		WriteBuffer:            4 * 1024 * 1024, // 4 MiB
		Filter:                 filter.NewBloomFilter(10),
	}
	db, err := leveldb.OpenFile(dbPath, options)
	if err != nil {
		return nil, err
	}
	return &lvlDB{db}, nil
}

func (db *lvlDB) Put(key, val []byte) error {
	return db.db.Put(key, val, nil)
}

func (db *lvlDB) Get(key []byte) ([]byte, error) {
	return db.db.Get(key, nil)
}

func (db *lvlDB) Has(key []byte) (bool, error) {
	return db.db.Has(key, nil)
}

func (db *lvlDB) NewBatch() batch {
	batch := new(leveldb.Batch)
	return &lvlBatch{
		batch: batch,
		db:    db.db,
	}
}

func (db *lvlDB) NewPrefixIterator(prefix []byte) iterator {
	rng := levelutil.BytesPrefix(prefix)
	it := db.db.NewIterator(rng, nil)
	return it
}

func (db *lvlDB) NewSizedIterator(start []byte, size int) iterator {
	return db.newSizedIterator(start, size)
}

type sizedIterator struct {
	it        iterator
	curIndex  int
	sizeLimit int
}

func (db *lvlDB) newSizedIterator(start []byte, size int) *sizedIterator {
	rng := &levelutil.Range{Start: start, Limit: nil}
	it := db.db.NewIterator(rng, nil)
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

// Note: lvlBatch is not thread safe
type lvlBatch struct {
	batch     *leveldb.Batch
	db        *leveldb.DB
	valueSize int
}

func (b *lvlBatch) Put(key, val []byte) error {
	b.batch.Put(key, val)
	b.valueSize += len(val)
	return nil
}

func (b *lvlBatch) Write() error {
	if err := b.db.Write(b.batch, nil); err != nil {
		return err
	}
	b.valueSize = 0
	return nil
}

func (b *lvlBatch) ValueSize() int {
	return b.valueSize
}

type iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Release()
	Error() error
}

// blockChainTxIndexer is the interface to check the loop up entry for transaction.
// Implemented by *core.BlockChain
type blockChainTxIndexer interface {
	ReadTxLookupEntry(txID common.Hash) (common.Hash, uint64, uint64)
}
