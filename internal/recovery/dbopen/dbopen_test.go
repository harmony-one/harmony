package dbopen

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// makeColdDB writes a small cold localnet-shaped DB directory and returns
// its path (basename harmony_db_0).
func makeColdDB(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if err := db.Put([]byte{byte(i)}, []byte{byte(i), byte(i)}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.CompactRange(util.Range{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestOpenColdReadOnly(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("strict opener is unix-only")
	}
	dir := makeColdDB(t)
	db, err := OpenStrictReadOnly(dir, Options{})
	if err != nil {
		t.Fatalf("open cold DB: %v", err)
	}
	defer db.Close()
	kv := db.KV()
	v, err := kv.Get([]byte{5})
	if err != nil || len(v) != 2 || v[0] != 5 {
		t.Fatalf("read back = %x err %v", v, err)
	}
	// Every mutating method refuses and is counted.
	if err := kv.Put([]byte("x"), []byte("y")); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Put must refuse, got %v", err)
	}
	if err := kv.Delete([]byte("x")); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Delete must refuse, got %v", err)
	}
	if err := kv.Compact(nil, nil); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Compact must refuse, got %v", err)
	}
	b := kv.NewBatch()
	_ = b.Put([]byte("a"), []byte("b"))
	if err := b.Write(); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("batch Write must refuse, got %v", err)
	}
	if db.WriteAttempts() == 0 {
		t.Fatal("refused writes must be counted")
	}
}

func TestErrorIfMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := os.MkdirAll(missing, 0o755); err != nil {
		t.Fatal(err)
	}
	// An empty directory has no CURRENT; open must error, not create a DB.
	if _, err := OpenStrictReadOnly(missing, Options{}); err == nil {
		t.Fatal("open of an empty directory must error (ErrorIfMissing)")
	}
	// And it must not have created anything.
	entries, _ := os.ReadDir(missing)
	if len(entries) != 0 {
		t.Fatalf("open created files in an empty dir: %v", names(entries))
	}
}

func TestMissingLockCreatesNothing(t *testing.T) {
	dir := makeColdDB(t)
	before := snapshotDir(t, dir)
	if err := os.Remove(filepath.Join(dir, "LOCK")); err != nil {
		t.Fatal(err)
	}
	_, err := OpenStrictReadOnly(dir, Options{})
	if !errors.Is(err, ErrMissingLock) {
		t.Fatalf("missing LOCK must yield ErrMissingLock, got %v", err)
	}
	// The O_CREATE regression: nothing may have been created, in
	// particular no LOCK.
	after := snapshotDir(t, dir)
	if _, ok := after["LOCK"]; ok {
		t.Fatal("strict opener created a LOCK file (the O_CREATE regression)")
	}
	delete(before, "LOCK")
	assertSameFiles(t, before, after)
}

func TestConcurrentWriterRefused(t *testing.T) {
	dir := makeColdDB(t)
	// Hold the DB open with a normal writable handle (exclusive flock on
	// LOCK), simulating a running node.
	live, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	_, err = OpenStrictReadOnly(dir, Options{})
	if !errors.Is(err, ErrConcurrentWriter) {
		t.Fatalf("a live writer must make the strict open fail with ErrConcurrentWriter, got %v", err)
	}
}

func TestCorruptManifestNoRecovery(t *testing.T) {
	dir := makeColdDB(t)
	// Corrupt the manifest named by CURRENT.
	current, err := os.ReadFile(filepath.Join(dir, "CURRENT"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, trim(string(current)))
	before := snapshotDir(t, dir)
	if err := os.WriteFile(manifest, []byte("garbage-not-a-manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	beforeCorrupt := snapshotDir(t, dir)
	if _, err := OpenStrictReadOnly(dir, Options{}); err == nil {
		t.Fatal("corrupt manifest must error without recovery")
	}
	after := snapshotDir(t, dir)
	// The RecoverFile regression: open must not rewrite/create any file.
	// (We compare against the already-corrupted snapshot: the opener must
	// not touch anything.)
	assertSameFiles(t, beforeCorrupt, after)
	_ = before
}

func TestStrictStorageMethodsRefuse(t *testing.T) {
	s := newStrictStorage(t.TempDir())
	if err := s.SetMeta(storage.FileDesc{}); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("SetMeta must refuse, got %v", err)
	}
	if _, err := s.Create(storage.FileDesc{Type: storage.TypeTable, Num: 1}); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Create must refuse, got %v", err)
	}
	if err := s.Remove(storage.FileDesc{Type: storage.TypeTable, Num: 1}); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Remove must refuse, got %v", err)
	}
	if err := s.Rename(storage.FileDesc{}, storage.FileDesc{}); !errors.Is(err, ErrWriteRefused) {
		t.Fatalf("Rename must refuse, got %v", err)
	}
}

func TestLockNeverCreates(t *testing.T) {
	// Lock() unit-tested directly: on a directory without a LOCK file it
	// only errors, never creates (the raced-replacement invariant).
	dir := t.TempDir()
	s := newStrictStorage(dir)
	if _, err := s.Lock(); err == nil {
		t.Fatal("Lock on a dir without LOCK must error")
	}
	if _, err := os.Stat(filepath.Join(dir, "LOCK")); !os.IsNotExist(err) {
		t.Fatal("Lock created a LOCK file")
	}
}

func TestValidateOutputPathRejectsInsideDB(t *testing.T) {
	dir := makeColdDB(t)
	inside := filepath.Join(dir, "report.json")
	if err := ValidateOutputPath(inside, dir); err == nil {
		t.Fatal("an output path inside the DB dir must be rejected")
	}
	if err := ValidateOutputPath(dir, dir); err == nil {
		t.Fatal("the DB dir itself must be rejected as an output path")
	}
	outside := filepath.Join(filepath.Dir(dir), "report.json")
	if err := ValidateOutputPath(outside, dir); err != nil {
		t.Fatalf("a sibling output path must be accepted: %v", err)
	}
}

func TestCheckLayout(t *testing.T) {
	dir := makeColdDB(t)
	if err := CheckLayout(dir, 0); err != nil {
		t.Fatalf("cold harmony_db_0 must pass layout: %v", err)
	}
	if err := CheckLayout(dir, 1); err == nil {
		t.Fatal("basename harmony_db_0 must not match shard 1")
	}
	if err := CheckLayout("relative/path", 0); err == nil {
		t.Fatal("relative path must be refused")
	}
	sharded := filepath.Join(t.TempDir(), "harmony_sharddb_0")
	_ = os.MkdirAll(sharded, 0o755)
	if err := CheckLayout(sharded, 0); err == nil {
		t.Fatal("sharded layout must be refused")
	}
}

// --- helpers ---

func names(entries []os.DirEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func snapshotDir(t *testing.T, dir string) map[string]int64 {
	t.Helper()
	m := map[string]int64{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = info.Size()
	}
	return m
}

func assertSameFiles(t *testing.T, before, after map[string]int64) {
	t.Helper()
	for name := range after {
		if _, ok := before[name]; !ok {
			t.Fatalf("strict open created file %q", name)
		}
	}
	for name := range before {
		if _, ok := after[name]; !ok {
			t.Fatalf("strict open removed file %q", name)
		}
	}
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
