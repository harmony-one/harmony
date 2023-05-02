package leveldb_shard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/comparer"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LeveldbShard struct {
	dbs     []*leveldb.DB
	dbCount uint32
}

var shardIdxKey = []byte("__DB_SHARED_INDEX__")

func NewLeveldbShard(savePath string, diskCount int, diskShards int) (shard *LeveldbShard, err error) {
	shard = &LeveldbShard{
		dbs:     make([]*leveldb.DB, diskCount*diskShards),
		dbCount: uint32(diskCount * diskShards),
	}

	// clean when error
	defer func() {
		if err != nil {
			for _, db := range shard.dbs {
				if db != nil {
					_ = db.Close()
				}
			}

			shard = nil
		}
	}()

	levelDBOptions := &opt.Options{
		OpenFilesCacheCapacity: 128,
		WriteBuffer:            8 << 20,  //8MB, max memory occupyv = 8*2*diskCount*diskShards
		BlockCacheCapacity:     16 << 20, //16MB
		Filter:                 filter.NewBloomFilter(8),
		DisableSeeksCompaction: true,
	}

	// async open
	wg := sync.WaitGroup{}
	for i := 0; i < diskCount; i++ {
		for j := 0; j < diskShards; j++ {
			shardPath := filepath.Join(savePath, fmt.Sprintf("disk%02d", i), fmt.Sprintf("block%02d", j))
			dbIndex := i*diskShards + j
			wg.Add(1)
			go func() {
				defer wg.Done()

				ldb, openErr := leveldb.OpenFile(shardPath, levelDBOptions)
				if openErr != nil {
					err = openErr
					return
				}

				indexByte := make([]byte, 8)
				binary.BigEndian.PutUint64(indexByte, uint64(dbIndex))
				inDBIndex, getErr := ldb.Get(shardIdxKey, nil)
				if getErr != nil {
					if getErr == leveldb.ErrNotFound {
						putErr := ldb.Put(shardIdxKey, indexByte, nil)
						if putErr != nil {
							err = putErr
							return
						}
					} else {
						err = getErr
						return
					}
				} else if bytes.Compare(indexByte, inDBIndex) != 0 {
					err = fmt.Errorf("db shard index error, need %v, got %v", indexByte, inDBIndex)
					return
				}

				shard.dbs[dbIndex] = ldb
			}()
		}
	}

	wg.Wait()

	return shard, err
}

func (l *LeveldbShard) NewBatchWithSize(size int) ethdb.Batch {
	return nil
}

func (l *LeveldbShard) NewSnapshot() (ethdb.Snapshot, error) {
	return nil, nil
}

func (l *LeveldbShard) mapDB(key []byte) *leveldb.DB {
	return l.dbs[mapDBIndex(key, l.dbCount)]
}

// Has retrieves if a key is present in the key-value data store.
func (l *LeveldbShard) Has(key []byte) (bool, error) {
	return l.mapDB(key).Has(key, nil)
}

// Get retrieves the given key if it's present in the key-value data store.
func (l *LeveldbShard) Get(key []byte) ([]byte, error) {
	return l.mapDB(key).Get(key, nil)
}

// Put inserts the given value into the key-value data store.
func (l *LeveldbShard) Put(key []byte, value []byte) error {
	return l.mapDB(key).Put(key, value, nil)
}

// Delete removes the key from the key-value data store.
func (l *LeveldbShard) Delete(key []byte) error {
	return l.mapDB(key).Delete(key, nil)
}

// NewBatch creates a write-only database that buffers changes to its host db
// until a final write is called.
func (l *LeveldbShard) NewBatch() ethdb.Batch {
	return NewLeveldbShardBatch(l)
}

// NewIterator creates a binary-alphabetical iterator over the entire keyspace
// contained within the key-value database.
func (l *LeveldbShard) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return l.iterator(nil)
}

// NewIteratorWithStart creates a binary-alphabetical iterator over a subset of
// database content starting at a particular initial key (or after, if it does
// not exist).
func (l *LeveldbShard) NewIteratorWithStart(start []byte) ethdb.Iterator {
	return l.iterator(&util.Range{Start: start})
}

// NewIteratorWithPrefix creates a binary-alphabetical iterator over a subset
// of database content with a particular key prefix.
func (l *LeveldbShard) NewIteratorWithPrefix(prefix []byte) ethdb.Iterator {
	return l.iterator(util.BytesPrefix(prefix))
}

func (l *LeveldbShard) iterator(slice *util.Range) ethdb.Iterator {
	iters := make([]iterator.Iterator, l.dbCount)

	for i, db := range l.dbs {
		iter := db.NewIterator(slice, nil)
		iters[i] = iter
	}

	return iterator.NewMergedIterator(iters, comparer.DefaultComparer, true)
}

// Stat returns a particular internal stat of the database.
func (l *LeveldbShard) Stat(property string) (string, error) {
	sb := strings.Builder{}

	for i, db := range l.dbs {
		getProperty, err := db.GetProperty(property)
		if err != nil {
			return "", err
		}

		sb.WriteString(fmt.Sprintf("=== shard %02d ===\n", i))
		sb.WriteString(getProperty)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// Compact flattens the underlying data store for the given key range. In essence,
// deleted and overwritten versions are discarded, and the data is rearranged to
// reduce the cost of operations needed to access them.
//
// A nil start is treated as a key before all keys in the data store; a nil limit
// is treated as a key after all keys in the data store. If both is nil then it
// will compact entire data store.
func (l *LeveldbShard) Compact(start []byte, limit []byte) (err error) {
	return parallelRunAndReturnErr(int(l.dbCount), func(i int) error {
		return l.dbs[i].CompactRange(util.Range{Start: start, Limit: limit})
	})
}

// Close all the DB
func (l *LeveldbShard) Close() error {
	for _, db := range l.dbs {
		err := db.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
