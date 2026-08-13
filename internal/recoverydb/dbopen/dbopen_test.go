package dbopen

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// fingerprint hashes every file (name + content) under dir.
func fingerprint(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, f := range files {
		h.Write([]byte(f))
		fd, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(h, fd); err != nil {
			t.Fatal(err)
		}
		fd.Close()
	}
	return hex.EncodeToString(h.Sum(nil))
}

func makeDB(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	ldb, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := byte(0); i < 100; i++ {
		if err := ldb.Put([]byte{'k', i}, []byte{i}, nil); err != nil {
			t.Fatal(err)
		}
	}
	// Force SST creation so the truncated-SST fixture has a file to damage.
	if err := ldb.CompactRange(util.Range{}); err != nil {
		t.Fatal(err)
	}
	if err := ldb.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRefusals(t *testing.T) {
	dir := makeDB(t)

	// Relative paths refused (plan §4 absolute-paths-only).
	if _, err := OpenReadOnly("relative/path"); err == nil {
		t.Fatal("relative path must refuse")
	}

	// Sharded layout refused (plan §2.2.3).
	shardedRoot := filepath.Join(t.TempDir(), "harmony_sharddb_0")
	if err := os.MkdirAll(shardedRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenReadOnly(shardedRoot); !errors.Is(err, ErrShardedLayout) {
		t.Fatalf("sharded name must refuse, got %v", err)
	}
	// A root of numbered subdirectories each holding a LevelDB is the
	// merged multi-LDB layout.
	multiRoot := filepath.Join(t.TempDir(), "harmony_db_0")
	sub := filepath.Join(multiRoot, "0")
	ldb, err := leveldb.OpenFile(sub, nil)
	if err != nil {
		t.Fatal(err)
	}
	ldb.Close()
	if _, err := OpenReadOnly(multiRoot); !errors.Is(err, ErrShardedLayout) {
		t.Fatalf("multi-LDB layout must refuse, got %v", err)
	}

	// Live-held directory refused (writable holder owns the lock).
	holder, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := OpenReadOnly(dir); err == nil {
		t.Fatal("live-held directory must refuse")
	}
}

// TestStrictOpenerCorruption: corrupted MANIFEST and truncated SST fixtures
// each return a fatal corruption error with byte-identical directory
// fingerprints before and after — no recovery, no writes (plan WS1, round 6
// finding 1). The stock geth wrapper demonstrably mutates the same fixture
// (regression guard for the pinned-version RecoverFile behavior).
func TestStrictOpenerCorruption(t *testing.T) {
	corrupt := func(t *testing.T, glob string, truncate bool) string {
		dir := makeDB(t)
		matches, err := filepath.Glob(filepath.Join(dir, glob))
		if err != nil || len(matches) == 0 {
			t.Fatalf("no %s in fixture: %v", glob, err)
		}
		target := matches[0]
		if truncate {
			if err := os.Truncate(target, 10); err != nil {
				t.Fatal(err)
			}
		} else {
			raw, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			for i := range raw {
				raw[i] ^= 0xff
			}
			if err := os.WriteFile(target, raw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}

	// A corrupted MANIFEST fails at open; a truncated SST is admitted by
	// goleveldb's lazy open and fails on first read. Both must surface a
	// fatal error with the directory byte-identical (never RecoverFile).
	t.Run("corruptedManifest", func(t *testing.T) {
		dir := corrupt(t, "MANIFEST-*", false)
		before := fingerprint(t, dir)
		if _, err := OpenReadOnly(dir); err == nil {
			t.Fatal("corrupted MANIFEST must refuse at open")
		}
		if after := fingerprint(t, dir); before != after {
			t.Fatalf("strict opener mutated a corrupted directory")
		}
	})
	t.Run("truncatedSST", func(t *testing.T) {
		dir := corrupt(t, "*.ldb", true)
		before := fingerprint(t, dir)
		ro, err := OpenReadOnly(dir)
		if err == nil {
			// Lazy open admitted the handle; reads must fail fatally.
			readErr := func() error {
				for i := byte(0); i < 100; i++ {
					if _, err := ro.Get([]byte{'k', i}); err != nil {
						return err
					}
				}
				it := ro.NewIterator(nil, nil)
				defer it.Release()
				for it.Next() {
				}
				return it.Error()
			}()
			ro.Close()
			if readErr == nil {
				t.Fatal("reads through a truncated SST must fail")
			}
		}
		if after := fingerprint(t, dir); before != after {
			t.Fatalf("strict opener mutated a corrupted directory on the read path")
		}
	})

	// Regression guard: the stock geth-v1.11.2 wrapper runs writable
	// RecoverFile on a corrupted MANIFEST even with readonly=true and
	// mutates the directory (plan §2.1; why every source open goes through
	// the strict opener).
	t.Run("stockWrapperMutates", func(t *testing.T) {
		dir := corrupt(t, "MANIFEST-*", false)
		before := fingerprint(t, dir)
		db, err := rawdb.NewLevelDBDatabase(dir, 16, 16, "", true /* readonly */)
		if err == nil {
			db.Close()
		}
		after := fingerprint(t, dir)
		if before == after {
			t.Fatalf("expected the stock readonly open to mutate the corrupted directory (RecoverFile); pinned-version behavior changed?")
		}
	})
}

// TestMutationRefusal exercises every write-shaped method of the read-only
// adapter (plan WS1, round 7 finding 4 + round 8 nit).
func TestMutationRefusal(t *testing.T) {
	dir := makeDB(t)
	before := fingerprint(t, dir)
	ro, err := OpenReadOnly(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ro.Put([]byte("k"), []byte("v")); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Put: %v", err)
	}
	if err := ro.Delete([]byte("k")); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete: %v", err)
	}
	if err := ro.Compact(nil, nil); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Compact: %v", err)
	}
	for name, b := range map[string]interface {
		Put([]byte, []byte) error
		Delete([]byte) error
		Write() error
		ValueSize() int
		Reset()
	}{
		"NewBatch":         ro.NewBatch(),
		"NewBatchWithSize": ro.NewBatchWithSize(16),
	} {
		if err := b.Put([]byte("k"), []byte("v")); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("%s.Put: %v", name, err)
		}
		if err := b.Delete([]byte("k")); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("%s.Delete: %v", name, err)
		}
		if err := b.Write(); !errors.Is(err, ErrReadOnly) {
			t.Fatalf("%s.Write: %v", name, err)
		}
		if b.ValueSize() != 0 {
			t.Fatalf("%s.ValueSize must be 0 (nothing ever accumulates)", name)
		}
		b.Reset() // must be a no-op, not panic
	}
	batch := ro.NewBatch()
	sink, err := OpenDestination(filepath.Join(t.TempDir(), "sink"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close()
	if err := batch.Replay(sink); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Replay must refuse (it could mutate an external writer): %v", err)
	}

	// Reads still work; directory untouched.
	if v, err := ro.Get([]byte{'k', 1}); err != nil || len(v) != 1 {
		t.Fatalf("Get through RO handle: %v %x", err, v)
	}
	if err := ro.Close(); err != nil {
		t.Fatal(err)
	}
	if after := fingerprint(t, dir); after != before {
		t.Fatalf("read-only adapter mutated the directory")
	}
}

// TestReadOnlyIsSharedLock: read-only opens take compatible shared locks —
// two RO handles coexist, a writable open fails while one is held (plan WS7
// acceptance nit, round 9).
func TestReadOnlyIsSharedLock(t *testing.T) {
	dir := makeDB(t)
	ro1, err := OpenReadOnly(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ro1.Close()
	ro2, err := OpenReadOnly(dir)
	if err != nil {
		t.Fatalf("second read-only open must succeed (shared lock): %v", err)
	}
	ro2.Close()
	if _, err := leveldb.OpenFile(dir, &opt.Options{ErrorIfMissing: true}); err == nil {
		t.Fatal("writable open must fail while the read-only storage lock is held")
	}
}
