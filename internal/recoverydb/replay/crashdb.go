package replay

import (
	"os"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// midInsertBatchPoint is the deterministic crash point that fires INSIDE a
// single block insert's write sequence (round 14 finding 3). One insert
// commits in several separate leveldb writes (the trie-node batch from
// TrieDB.Commit, WriteBlockWithState's block batch, then the head-pointer
// writes); dying between two of them leaves a genuinely torn block on disk,
// unlike replay.mid-insert which fires on the clean between-blocks boundary.
const midInsertBatchPoint = "replay.mid-insert-batch"

// crashDB wraps the destination handle for the crash matrix. Once armed (at
// insert-loop start, so open/journal setup writes never count), the process
// dies immediately BEFORE the second counted mutation commit — direct
// Put/Delete or Batch.Write — when $RECOVERYDB_CRASHPOINT names the
// mid-batch point. Inert in production: the env var is unset.
type crashDB struct {
	ethdb.Database
	enabled bool
	armed   atomic.Bool
	commits atomic.Int64
}

func newCrashDB(db ethdb.Database) *crashDB {
	return &crashDB{
		Database: db,
		enabled:  os.Getenv(report.CrashPointEnv) == midInsertBatchPoint,
	}
}

func (c *crashDB) arm() { c.armed.Store(true) }

// tick dies before the second armed commit: the first insert's trie-node
// batch lands, its block batch (and everything after) never does.
func (c *crashDB) tick() {
	if !c.enabled || !c.armed.Load() {
		return
	}
	if c.commits.Add(1) == 2 {
		os.Exit(137)
	}
}

func (c *crashDB) Put(key, value []byte) error {
	c.tick()
	return c.Database.Put(key, value)
}

func (c *crashDB) Delete(key []byte) error {
	c.tick()
	return c.Database.Delete(key)
}

func (c *crashDB) NewBatch() ethdb.Batch {
	return &crashBatch{Batch: c.Database.NewBatch(), db: c}
}

func (c *crashDB) NewBatchWithSize(size int) ethdb.Batch {
	return &crashBatch{Batch: c.Database.NewBatchWithSize(size), db: c}
}

type crashBatch struct {
	ethdb.Batch
	db *crashDB
}

func (b *crashBatch) Write() error {
	b.db.tick()
	return b.Batch.Write()
}
