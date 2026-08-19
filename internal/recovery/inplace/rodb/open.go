package rodb

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

// Options tunes the read-only open. Zero values pick safe defaults.
type Options struct {
	// Handles caps goleveldb's open-file cache (OpenFilesCacheCapacity).
	Handles int
	// BlockCacheMB caps the table block cache, in MiB.
	BlockCacheMB int
}

const (
	// DefaultHandles is the default open-file cache capacity.
	DefaultHandles = 512
	// DefaultBlockCacheMB is the default block cache size in MiB.
	DefaultBlockCacheMB = 256
)

func (o Options) withDefaults() Options {
	if o.Handles <= 0 {
		o.Handles = DefaultHandles
	}
	if o.BlockCacheMB <= 0 {
		o.BlockCacheMB = DefaultBlockCacheMB
	}
	return o
}

// DB is an open, strictly read-only database handle.
type DB struct {
	ldb  *leveldb.DB
	stor *roStorage
}

// Open opens the database directory read-only via the no-flock storage.
// It never calls leveldb.RecoverFile and never uses geth's leveldb wrapper
// (whose open path calls RecoverFile on ErrCorrupted, dropping ReadOnly and
// rewriting the MANIFEST on disk).
//
// With ReadOnly+ErrorIfMissing: a missing DB errors instead of being
// created, journal recovery is performed in memory only, obsolete-file
// cleanup and compaction are skipped, and every internal write path returns
// an error.
func Open(dir string, opts Options) (*DB, error) {
	opts = opts.withDefaults()
	stor := newROStorage(dir)
	ldb, err := leveldb.Open(stor, &opt.Options{
		ReadOnly:               true,
		ErrorIfMissing:         true,
		OpenFilesCacheCapacity: opts.Handles,
		BlockCacheCapacity:     opts.BlockCacheMB * opt.MiB,
		// Fail closed on manifest/journal corruption instead of goleveldb's
		// default record-dropping (a silently truncated manifest would make
		// missing tables look like key absence - a false FAIL). A cleanly
		// torn live-journal tail still reads as graceful EOF; actual
		// corruption surfaces as ErrCorrupted and is classified by Classify.
		Strict: opt.DefaultStrict | opt.StrictManifest | opt.StrictJournal,
	})
	if err != nil {
		_ = stor.Close()
		return nil, err
	}
	return &DB{ldb: ldb, stor: stor}, nil
}

// KV returns the write-refusing ethdb adapter recording read errors into
// latch.
func (d *DB) KV(latch *Latch) *KV {
	return &KV{ldb: d.ldb, latch: latch}
}

// Close closes the goleveldb session and the storage.
func (d *DB) Close() error {
	err := d.ldb.Close()
	if cerr := d.stor.Close(); err == nil {
		err = cerr
	}
	return err
}
