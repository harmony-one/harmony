// Package dbopen provides the strict database open helpers used by every
// harmony-recovery-db command.
//
// All *source* opens go through OpenReadOnly: goleveldb opened directly with
// ReadOnly+ErrorIfMissing so corruption is returned as a fatal error with the
// directory untouched. The stock geth path is corruption-UNSAFE here: go.mod
// pins go-ethereum v1.11.2, whose ethdb/leveldb constructor calls writable
// leveldb.RecoverFile when the open reports ErrCorrupted, even with
// readonly=true (plan §2.1, in-place handoff §C1). Never use it for sources.
//
// The open acquires goleveldb's OS-level storage lock on LOCK; failed
// acquisition means a live user and the open is refused. Callers that must
// guarantee byte-stability (packaging stage-and-hash) retain the handle for
// the whole pass. Merely statting LOCK is never a lock.
package dbopen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/syndtr/goleveldb/leveldb"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// ErrReadOnly is returned deterministically by every write-shaped method of
// the read-only adapter.
var ErrReadOnly = errors.New("recoverydb: database opened read-only; writes are refused")

// ErrCorrupted wraps a goleveldb corruption report. The directory is
// guaranteed untouched: no recovery is ever attempted.
var ErrCorrupted = errors.New("recoverydb: source database is corrupted (no recovery attempted, directory untouched)")

// ErrShardedLayout is returned for harmony_sharddb_* / multi-part layouts,
// which have no read-only open path (plan §2.2.3; contingency parked).
var ErrShardedLayout = errors.New("recoverydb: sharded database layout (harmony_sharddb) is not supported; only the standard harmony_db_N LevelDB layout is")

// ErrLocked is returned when the storage lock cannot be acquired, meaning a
// live process has the database open.
var ErrLocked = errors.New("recoverydb: database storage lock is held by a live process; refusing to open")

// RequireAbsolute enforces the global absolute-paths-only command rule.
func RequireAbsolute(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("recoverydb: path %q must be absolute", path)
	}
	return nil
}

// DetectLayout classifies a database directory. It returns nil for a
// standard single-LevelDB layout and ErrShardedLayout for the merged
// multi-LDB layout (root with per-disk numeric subdirectories, or a
// harmony_sharddb_* path name). TiKV/elastic layouts have no local
// directory and are implicitly refused by the CURRENT-file check.
func DetectLayout(dir string) error {
	base := filepath.Base(dir)
	if strings.HasPrefix(base, "harmony_sharddb") {
		return ErrShardedLayout
	}
	if _, err := os.Stat(filepath.Join(dir, "CURRENT")); err == nil {
		return nil // standard LevelDB directory
	}
	// No CURRENT at the root: a sharded root holds numbered subdirectories
	// each containing its own LevelDB. Detect and refuse explicitly.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("recoverydb: cannot read database directory %s: %w", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, e.Name(), "CURRENT")); err == nil {
				return ErrShardedLayout
			}
		}
	}
	return fmt.Errorf("recoverydb: %s does not look like a LevelDB database (no CURRENT file)", dir)
}

// ReadOnlyDB is a complete read-only ethdb.KeyValueStore over a directly
// opened goleveldb handle. Reads delegate; every write-shaped method returns
// ErrReadOnly. Close releases the storage lock.
type ReadOnlyDB struct {
	path string
	ldb  *leveldb.DB
}

// Compile-time interface-completeness pin against the vendored geth version
// (plan WS1, round 7 finding 4).
var _ ethdb.KeyValueStore = (*ReadOnlyDB)(nil)

// OpenReadOnly opens dir strictly read-only. It never invokes goleveldb
// recovery; corruption is reported fatally via ErrCorrupted.
func OpenReadOnly(dir string) (*ReadOnlyDB, error) {
	if err := RequireAbsolute(dir); err != nil {
		return nil, err
	}
	if err := DetectLayout(dir); err != nil {
		return nil, err
	}
	ldb, err := leveldb.OpenFile(dir, &opt.Options{
		ReadOnly:       true,
		ErrorIfMissing: true,
	})
	if err != nil {
		if ldberrors.IsCorrupted(err) {
			return nil, fmt.Errorf("%w: %s: %v", ErrCorrupted, dir, err)
		}
		if isLockError(err) {
			return nil, fmt.Errorf("%w: %s: %v", ErrLocked, dir, err)
		}
		return nil, fmt.Errorf("recoverydb: open %s read-only: %w", dir, err)
	}
	return &ReadOnlyDB{path: dir, ldb: ldb}, nil
}

func isLockError(err error) bool {
	// goleveldb reports a held storage lock as an os-level "resource
	// temporarily unavailable" flock failure wrapped in ErrLocked text.
	msg := err.Error()
	return strings.Contains(msg, "resource temporarily unavailable") ||
		strings.Contains(msg, "already locked") ||
		strings.Contains(msg, "lock")
}

// Path returns the database directory.
func (db *ReadOnlyDB) Path() string { return db.path }

// Has retrieves if a key is present.
func (db *ReadOnlyDB) Has(key []byte) (bool, error) { return db.ldb.Has(key, nil) }

// Get retrieves the given key if present.
func (db *ReadOnlyDB) Get(key []byte) ([]byte, error) { return db.ldb.Get(key, nil) }

// Put refuses: read-only.
func (db *ReadOnlyDB) Put(key []byte, value []byte) error { return ErrReadOnly }

// Delete refuses: read-only.
func (db *ReadOnlyDB) Delete(key []byte) error { return ErrReadOnly }

// Stat delegates to goleveldb properties (mirrors geth's mapping).
func (db *ReadOnlyDB) Stat(property string) (string, error) {
	if property == "" {
		property = "leveldb.stats"
	} else if !strings.HasPrefix(property, "leveldb.") {
		property = "leveldb." + property
	}
	return db.ldb.GetProperty(property)
}

// Compact refuses: it rewrites SSTs.
func (db *ReadOnlyDB) Compact(start []byte, limit []byte) error { return ErrReadOnly }

// NewBatch returns a batch whose every mutating method refuses.
func (db *ReadOnlyDB) NewBatch() ethdb.Batch { return &readOnlyBatch{} }

// NewBatchWithSize returns a batch whose every mutating method refuses.
func (db *ReadOnlyDB) NewBatchWithSize(size int) ethdb.Batch { return &readOnlyBatch{} }

// NewIterator creates a binary-alphabetical iterator over a key subset.
func (db *ReadOnlyDB) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	return db.ldb.NewIterator(bytesPrefixRange(prefix, start), nil)
}

// NewSnapshot creates a read-only snapshot view.
func (db *ReadOnlyDB) NewSnapshot() (ethdb.Snapshot, error) {
	snap, err := db.ldb.GetSnapshot()
	if err != nil {
		return nil, err
	}
	return &roSnapshot{snap: snap}, nil
}

// Close closes the handle and releases the storage lock.
func (db *ReadOnlyDB) Close() error { return db.ldb.Close() }

// bytesPrefixRange returns key range that satisfy
// - the given prefix, and
// - the given seek position
// (same semantics as geth ethdb/leveldb).
func bytesPrefixRange(prefix, start []byte) *util.Range {
	r := util.BytesPrefix(prefix)
	r.Start = append(r.Start, start...)
	return r
}

type roSnapshot struct{ snap *leveldb.Snapshot }

func (s *roSnapshot) Has(key []byte) (bool, error)   { return s.snap.Has(key, nil) }
func (s *roSnapshot) Get(key []byte) ([]byte, error) { return s.snap.Get(key, nil) }
func (s *roSnapshot) Release()                       { s.snap.Release() }

// readOnlyBatch refuses every mutation deterministically (plan WS1: Replay
// could otherwise mutate an external writer; ValueSize returns 0 because
// nothing ever accumulates; Reset is a no-op).
type readOnlyBatch struct{}

var _ ethdb.Batch = (*readOnlyBatch)(nil)

func (b *readOnlyBatch) Put(key, value []byte) error         { return ErrReadOnly }
func (b *readOnlyBatch) Delete(key []byte) error             { return ErrReadOnly }
func (b *readOnlyBatch) ValueSize() int                      { return 0 }
func (b *readOnlyBatch) Write() error                        { return ErrReadOnly }
func (b *readOnlyBatch) Reset()                              {}
func (b *readOnlyBatch) Replay(w ethdb.KeyValueWriter) error { return ErrReadOnly }

// OpenSourceDatabase opens a source directory strictly read-only and wraps
// it in the rawdb accessor layer (freezer-less).
func OpenSourceDatabase(dir string) (ethdb.Database, *ReadOnlyDB, error) {
	ro, err := OpenReadOnly(dir)
	if err != nil {
		return nil, nil, err
	}
	return rawdb.NewDatabase(ro), ro, nil
}

// OpenDestination opens (or creates) a writable destination database via the
// stock path. Destinations are freshly journaled and discarded on corruption,
// so the stock open is acceptable here (plan WS1). failIfNonEmpty enforces
// the --fail-if-destination-nonempty contract: the directory must not exist
// or must be empty.
func OpenDestination(dir string, failIfNonEmpty bool) (ethdb.Database, error) {
	if err := RequireAbsolute(dir); err != nil {
		return nil, err
	}
	if failIfNonEmpty {
		entries, err := os.ReadDir(dir)
		switch {
		case err == nil && len(entries) > 0:
			return nil, fmt.Errorf("recoverydb: destination %s is not empty; refusing (v1 never resumes an unclean destination)", dir)
		case err != nil && !os.IsNotExist(err):
			return nil, fmt.Errorf("recoverydb: stat destination %s: %w", dir, err)
		}
	}
	db, err := rawdb.NewLevelDBDatabase(dir, 256, 1024, "", false)
	if err != nil {
		return nil, fmt.Errorf("recoverydb: open destination %s: %w", dir, err)
	}
	return db, nil
}

// FreeSpace returns the free bytes on the filesystem containing path.
func FreeSpace(path string) (uint64, error) {
	return freeSpace(path)
}
