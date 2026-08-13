package dbopen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/syndtr/goleveldb/leveldb"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// Options tunes the strict read-only open.
type Options struct {
	Handles      int // open-file cache capacity (default 512)
	BlockCacheMB int // table block cache MiB (default 256)
}

func (o Options) withDefaults() Options {
	if o.Handles <= 0 {
		o.Handles = 512
	}
	if o.BlockCacheMB <= 0 {
		o.BlockCacheMB = 256
	}
	return o
}

// DB is an open strict read-only database handle.
type DB struct {
	ldb  *leveldb.DB
	stor *strictStorage
	path string // resolved absolute path

	mu            sync.Mutex
	writeAttempts int
}

// CheckLayout gates the directory shape (plan WS1): absolute path, no
// symlinked final component, the default single-LevelDB harmony_db_<shard>
// layout only. The sharded harmony_sharddb_* layout is refused — no
// read-only open path exists for it (internal/shardchain/leveldb_shard).
func CheckLayout(path string, shard uint32) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("dbopen: --db must be an absolute path (got %q)", path)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("dbopen: %w", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("dbopen: %s is a symlink; pass the resolved directory itself", path)
	}
	if !fi.IsDir() {
		return fmt.Errorf("dbopen: %s is not a directory", path)
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, "harmony_sharddb_") {
		return fmt.Errorf("dbopen: %s is a sharded (harmony_sharddb_*) layout; no strict read-only path exists for it", path)
	}
	if want := fmt.Sprintf("harmony_db_%d", shard); base != want {
		return fmt.Errorf("dbopen: directory basename %q does not match the expected %q layout", base, want)
	}
	if _, err := os.Stat(filepath.Join(path, "CURRENT")); err != nil {
		return fmt.Errorf("dbopen: %s does not look like a LevelDB (no CURRENT): %w", path, err)
	}
	return nil
}

// OpenStrictReadOnly opens the database directory via the no-create
// flock-guarded storage, ReadOnly+ErrorIfMissing, strict manifest/journal
// handling, and no recovery path of any kind.
func OpenStrictReadOnly(path string, opts Options) (*DB, error) {
	opts = opts.withDefaults()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("dbopen: resolve %s: %w", path, err)
	}
	stor := newStrictStorage(resolved)
	ldb, err := leveldb.Open(stor, &opt.Options{
		ReadOnly:               true,
		ErrorIfMissing:         true,
		OpenFilesCacheCapacity: opts.Handles,
		BlockCacheCapacity:     opts.BlockCacheMB * opt.MiB,
		// Fail closed on manifest/journal corruption instead of the default
		// record-dropping.
		Strict: opt.DefaultStrict | opt.StrictManifest | opt.StrictJournal,
	})
	if err != nil {
		_ = stor.Close()
		return nil, err
	}
	return &DB{ldb: ldb, stor: stor, path: resolved}, nil
}

// Path returns the resolved database directory.
func (d *DB) Path() string { return d.path }

// Close closes the goleveldb session and releases the shared lock.
func (d *DB) Close() error {
	err := d.ldb.Close()
	if cerr := d.stor.Close(); err == nil {
		err = cerr
	}
	return err
}

// WriteAttempts reports how many times a mutating KV method was invoked
// (each was refused; non-zero means a bug in a caller's read pipeline).
func (d *DB) WriteAttempts() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writeAttempts
}

func (d *DB) recordWriteAttempt() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writeAttempts++
}

// KV returns the write-refusing ethdb.KeyValueStore adapter.
func (d *DB) KV() ethdb.KeyValueStore { return &kv{d: d} }

// ClassifyExit maps an open error to the §4.5 exit code: 13 for unsafe
// open/concurrent writer/missing LOCK, 14 for I/O or corruption.
func ClassifyExit(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrConcurrentWriter), errors.Is(err, ErrMissingLock):
		return 13
	default:
		return 14
	}
}

// IsNotFound reports goleveldb key absence.
func IsNotFound(err error) bool { return errors.Is(err, ldberrors.ErrNotFound) }

type kv struct{ d *DB }

var _ ethdb.KeyValueStore = (*kv)(nil)

func (k *kv) Get(key []byte) ([]byte, error)  { return k.d.ldb.Get(key, nil) }
func (k *kv) Has(key []byte) (bool, error)    { return k.d.ldb.Has(key, nil) }
func (k *kv) Put(key, value []byte) error     { k.d.recordWriteAttempt(); return ErrWriteRefused }
func (k *kv) Delete(key []byte) error         { k.d.recordWriteAttempt(); return ErrWriteRefused }
func (k *kv) Compact(start, limit []byte) error {
	k.d.recordWriteAttempt()
	return ErrWriteRefused
}
func (k *kv) Stat(property string) (string, error) { return k.d.ldb.GetProperty(property) }
func (k *kv) NewBatch() ethdb.Batch                { return &refusingBatch{d: k.d} }
func (k *kv) NewBatchWithSize(int) ethdb.Batch     { return &refusingBatch{d: k.d} }
func (k *kv) Close() error                         { return nil } // owner controls lifecycle

func (k *kv) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	r := util.BytesPrefix(prefix)
	r.Start = append(r.Start, start...)
	return &ldbIterator{it: k.d.ldb.NewIterator(r, nil)}
}

func (k *kv) NewSnapshot() (ethdb.Snapshot, error) {
	snap, err := k.d.ldb.GetSnapshot()
	if err != nil {
		return nil, err
	}
	return &roSnapshot{snap: snap}, nil
}

type ldbIterator struct{ it iterator.Iterator }

func (i *ldbIterator) Next() bool    { return i.it.Next() }
func (i *ldbIterator) Key() []byte   { return i.it.Key() }
func (i *ldbIterator) Value() []byte { return i.it.Value() }
func (i *ldbIterator) Release()      { i.it.Release() }
func (i *ldbIterator) Error() error  { return i.it.Error() }

type roSnapshot struct{ snap *leveldb.Snapshot }

func (s *roSnapshot) Has(key []byte) (bool, error)   { return s.snap.Has(key, nil) }
func (s *roSnapshot) Get(key []byte) ([]byte, error) { return s.snap.Get(key, nil) }
func (s *roSnapshot) Release()                       { s.snap.Release() }

type refusingBatch struct{ d *DB }

var _ ethdb.Batch = (*refusingBatch)(nil)

func (b *refusingBatch) Put(key, value []byte) error {
	b.d.recordWriteAttempt()
	return ErrWriteRefused
}
func (b *refusingBatch) Delete(key []byte) error {
	b.d.recordWriteAttempt()
	return ErrWriteRefused
}
func (b *refusingBatch) ValueSize() int { return 0 }
func (b *refusingBatch) Write() error {
	b.d.recordWriteAttempt()
	return ErrWriteRefused
}
func (b *refusingBatch) Reset() {}
func (b *refusingBatch) Replay(w ethdb.KeyValueWriter) error {
	b.d.recordWriteAttempt()
	return ErrWriteRefused
}
