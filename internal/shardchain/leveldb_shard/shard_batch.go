package leveldb_shard

import (
	"runtime"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
)

var batchesPool = sync.Pool{
	New: func() interface{} {
		return &leveldb.Batch{}
	},
}

type LeveldbShardBatch struct {
	shard        *LeveldbShard
	batches      []*leveldb.Batch
	batchesCount uint32
}

func NewLeveldbShardBatch(shard *LeveldbShard) *LeveldbShardBatch {
	shardBatch := &LeveldbShardBatch{
		batches:      make([]*leveldb.Batch, shard.dbCount),
		batchesCount: shard.dbCount,
		shard:        shard,
	}

	for i := uint32(0); i < shard.dbCount; i++ {
		shardBatch.batches[i] = batchesPool.Get().(*leveldb.Batch)
	}

	runtime.SetFinalizer(shardBatch, func(o *LeveldbShardBatch) {
		for _, batch := range o.batches {
			batch.Reset()
			batchesPool.Put(batch)
		}

		o.batches = nil
	})

	return shardBatch
}

func (l *LeveldbShardBatch) mapBatch(key []byte) *leveldb.Batch {
	return l.batches[mapDBIndex(key, l.batchesCount)]
}

// Put inserts the given value into the key-value data store.
func (l *LeveldbShardBatch) Put(key []byte, value []byte) error {
	l.mapBatch(key).Put(key, value)
	return nil
}

// Delete removes the key from the key-value data store.
func (l *LeveldbShardBatch) Delete(key []byte) error {
	l.mapBatch(key).Delete(key)
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (l *LeveldbShardBatch) ValueSize() int {
	size := 0
	for _, batch := range l.batches {
		size += batch.Len()
	}
	return size
}

// Write flushes any accumulated data to disk.
func (l *LeveldbShardBatch) Write() (err error) {
	return parallelRunAndReturnErr(int(l.batchesCount), func(i int) error {
		return l.shard.dbs[i].Write(l.batches[i], nil)
	})
}

// Reset resets the batch for reuse.
func (l *LeveldbShardBatch) Reset() {
	for _, batch := range l.batches {
		batch.Reset()
	}
}

// Replay replays the batch contents.
func (l *LeveldbShardBatch) Replay(w ethdb.KeyValueWriter) error {
	for _, batch := range l.batches {
		err := batch.Replay(&replayer{writer: w})
		if err != nil {
			return err
		}
	}

	return nil
}

// replayer is a small wrapper to implement the correct replay methods.
type replayer struct {
	writer  ethdb.KeyValueWriter
	failure error
}

// Put inserts the given value into the key-value data store.
func (r *replayer) Put(key, value []byte) {
	// If the replay already failed, stop executing ops
	if r.failure != nil {
		return
	}
	r.failure = r.writer.Put(key, value)
}

// Delete removes the key from the key-value data store.
func (r *replayer) Delete(key []byte) {
	// If the replay already failed, stop executing ops
	if r.failure != nil {
		return
	}
	r.failure = r.writer.Delete(key)
}
