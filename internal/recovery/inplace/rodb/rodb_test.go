package rodb

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	lverrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/storage"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

// newTestDB creates a small stopped LevelDB with deterministic content.
func newTestDB(t *testing.T, n int) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if err := db.Put(testKey(i), testVal(i), nil); err != nil {
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

func testKey(i int) []byte { return []byte(fmt.Sprintf("key-%06d", i)) }
func testVal(i int) []byte { return []byte(fmt.Sprintf("val-%06d", i)) }

func TestLayoutGate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		dir := newTestDB(t, 10)
		if err := CheckLayout(dir, 0); err != nil {
			t.Fatalf("valid layout refused: %v", err)
		}
	})
	t.Run("missing", func(t *testing.T) {
		err := CheckLayout(filepath.Join(t.TempDir(), "nope"), 0)
		var le *LayoutError
		if !errors.As(err, &le) {
			t.Fatalf("want LayoutError, got %v", err)
		}
	})
	t.Run("sharded-name", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "harmony_sharddb_0")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "sharddb") {
			t.Fatalf("sharded layout not refused: %v", err)
		}
	})
	t.Run("pebble-options", func(t *testing.T) {
		dir := newTestDB(t, 1)
		if err := os.WriteFile(filepath.Join(dir, "OPTIONS-000003"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "pebble") {
			t.Fatalf("pebble layout not refused: %v", err)
		}
	})
	t.Run("not-a-leveldb", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "harmony_db_0")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "CURRENT") {
			t.Fatalf("non-leveldb dir not refused: %v", err)
		}
	})
	t.Run("wrong-shard-basename", func(t *testing.T) {
		// A valid LevelDB named harmony_db_1 must be refused for shard 0
		// (wrong-shard DBs would otherwise FAIL confusingly), and accepted
		// when the caller asks for shard 1.
		src := newTestDB(t, 5)
		dir := filepath.Join(filepath.Dir(src), "harmony_db_1")
		if err := os.Rename(src, dir); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "harmony_db_0") {
			t.Fatalf("wrong shard basename not refused: %v", err)
		}
		if err := CheckLayout(dir, 1); err != nil {
			t.Fatalf("harmony_db_1 refused for shard 1: %v", err)
		}
	})
	t.Run("renamed-dir-refused", func(t *testing.T) {
		// Even an otherwise valid LevelDB under an arbitrary basename is
		// refused: --db must point at the node's harmony_db_0 itself.
		src := newTestDB(t, 5)
		dir := filepath.Join(filepath.Dir(src), "db-backup")
		if err := os.Rename(src, dir); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "harmony_db_0") {
			t.Fatalf("renamed dir not refused: %v", err)
		}
	})
	t.Run("hints-at-subdir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "harmony_db_0"), 0o755); err != nil {
			t.Fatal(err)
		}
		err := CheckLayout(dir, 0)
		if err == nil || !strings.Contains(err.Error(), "subdirectory") {
			t.Fatalf("no subdir hint: %v", err)
		}
	})
}

func TestROStorageRefusesWrites(t *testing.T) {
	stor := newROStorage(newTestDB(t, 5))
	if _, err := stor.Create(storage.FileDesc{Type: storage.TypeTable, Num: 99}); err != ErrWriteRefused {
		t.Fatalf("Create: %v", err)
	}
	if err := stor.Remove(storage.FileDesc{Type: storage.TypeTable, Num: 1}); err != ErrWriteRefused {
		t.Fatalf("Remove: %v", err)
	}
	if err := stor.Rename(storage.FileDesc{Type: storage.TypeTable, Num: 1}, storage.FileDesc{Type: storage.TypeTable, Num: 2}); err != ErrWriteRefused {
		t.Fatalf("Rename: %v", err)
	}
	if err := stor.SetMeta(storage.FileDesc{Type: storage.TypeManifest, Num: 9}); err != ErrWriteRefused {
		t.Fatalf("SetMeta: %v", err)
	}
	// GetMeta and List work read-only.
	fd, err := stor.GetMeta()
	if err != nil || fd.Type != storage.TypeManifest {
		t.Fatalf("GetMeta: %v %v", fd, err)
	}
	fds, err := stor.List(storage.TypeAll)
	if err != nil || len(fds) == 0 {
		t.Fatalf("List: %v %v", fds, err)
	}
	// The LOCK file is never part of the descriptor namespace.
	for _, fd := range fds {
		if !storage.FileDescOk(fd) {
			t.Fatalf("bad descriptor %v", fd)
		}
	}
}

func TestAdapterRefusesWrites(t *testing.T) {
	dir := newTestDB(t, 20)
	db, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	latch := &Latch{}
	kv := db.KV(latch)

	if err := kv.Put([]byte("k"), []byte("v")); err != ErrWriteRefused {
		t.Fatalf("Put: %v", err)
	}
	if err := kv.Delete([]byte("k")); err != ErrWriteRefused {
		t.Fatalf("Delete: %v", err)
	}
	if err := kv.Compact(nil, nil); err != ErrWriteRefused {
		t.Fatalf("Compact: %v", err)
	}
	for name, b := range map[string]interface {
		Put(k, v []byte) error
		Delete(k []byte) error
		Write() error
	}{
		"NewBatch":         kv.NewBatch(),
		"NewBatchWithSize": kv.NewBatchWithSize(16),
	} {
		if err := b.Put([]byte("k"), []byte("v")); err != ErrWriteRefused {
			t.Fatalf("%s.Put: %v", name, err)
		}
		if err := b.Delete([]byte("k")); err != ErrWriteRefused {
			t.Fatalf("%s.Delete: %v", name, err)
		}
		if err := b.Write(); err != ErrWriteRefused {
			t.Fatalf("%s.Write: %v", name, err)
		}
	}
	if got := latch.WriteAttempts(); got == 0 {
		t.Fatal("write attempts not recorded")
	}

	// Reads pass through; a missing key is not latched.
	val, err := kv.Get(testKey(3))
	if err != nil || !bytes.Equal(val, testVal(3)) {
		t.Fatalf("Get: %q %v", val, err)
	}
	if _, err := kv.Get([]byte("absent")); !IsNotFound(err) {
		t.Fatalf("absent key: %v", err)
	}
	if latch.First() != nil {
		t.Fatalf("latch dirtied by not-found: %v", latch.First())
	}
	// Iterator and snapshot read paths work.
	it := kv.NewIterator([]byte("key-"), nil)
	count := 0
	for it.Next() {
		count++
	}
	it.Release()
	if count != 20 || it.Error() != nil {
		t.Fatalf("iterator: %d %v", count, it.Error())
	}
	snap, err := kv.NewSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer snap.Release()
	if v, err := snap.Get(testKey(1)); err != nil || !bytes.Equal(v, testVal(1)) {
		t.Fatalf("snapshot get: %q %v", v, err)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Class
	}{
		{"nil", nil, ClassNone},
		{"enoent", &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}, ClassRetryableRace},
		{"corrupt-journal", lverrors.NewErrCorrupted(storage.FileDesc{Type: storage.TypeJournal, Num: 3}, errors.New("torn tail")), ClassRetryableRace},
		{"corrupt-manifest", lverrors.NewErrCorrupted(storage.FileDesc{Type: storage.TypeManifest, Num: 2}, errors.New("bad record")), ClassRetryableRace},
		{"corrupt-table", lverrors.NewErrCorrupted(storage.FileDesc{Type: storage.TypeTable, Num: 7}, errors.New("checksum mismatch")), ClassCorruptTable},
		{"corrupt-unattributed", lverrors.NewErrCorrupted(storage.FileDesc{}, errors.New("mystery")), ClassPersistent},
		{"other", errors.New("permission denied"), ClassPersistent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, detail := Classify(c.err)
			if got != c.want {
				t.Fatalf("Classify(%v) = %v, want %v", c.err, got, c.want)
			}
			if c.want == ClassCorruptTable && !strings.Contains(detail, "000007.ldb") {
				t.Fatalf("corrupt table detail %q does not name the table", detail)
			}
		})
	}
}

func TestRunnerRetryPolicy(t *testing.T) {
	dir := newTestDB(t, 10)

	t.Run("open-race-then-success", func(t *testing.T) {
		r := NewRunner(dir, Options{})
		defer r.Close()
		fails := 1
		r.SetOpenFunc(func() (*DB, error) {
			if fails > 0 {
				fails--
				return nil, &os.PathError{Op: "open", Path: "000001.ldb", Err: os.ErrNotExist}
			}
			return Open(dir, Options{})
		})
		err := r.Stage("test", func(kv *KV) error {
			_, err := kv.Get(testKey(1))
			return err
		})
		if err != nil {
			t.Fatalf("stage: %v", err)
		}
		if r.ReopenCount() != 1 {
			t.Fatalf("reopen count %d, want 1", r.ReopenCount())
		}
	})

	t.Run("latched-race-then-success", func(t *testing.T) {
		r := NewRunner(dir, Options{})
		defer r.Close()
		injected := 1
		err := r.Stage("test", func(kv *KV) error {
			if injected > 0 {
				injected--
				r.Latch().Record(&os.PathError{Op: "read", Path: "000002.ldb", Err: os.ErrNotExist})
				// A missing-key FAIL computed under a dirty latch must NOT
				// surface as a verification failure.
				return report.Failf("target_header", "spurious absence")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("stage: %v", err)
		}
		if r.ReopenCount() != 1 {
			t.Fatalf("reopen count %d, want 1", r.ReopenCount())
		}
	})

	t.Run("verification-failure-clean-latch", func(t *testing.T) {
		r := NewRunner(dir, Options{})
		defer r.Close()
		err := r.Stage("test", func(kv *KV) error {
			return report.Failf("target_header", "genuinely missing")
		})
		var f *report.Failure
		if !errors.As(err, &f) {
			t.Fatalf("want Failure, got %v", err)
		}
		if r.ReopenCount() != 0 {
			t.Fatalf("reopens on clean FAIL: %d", r.ReopenCount())
		}
	})

	t.Run("corrupt-table-zero-retries", func(t *testing.T) {
		r := NewRunner(dir, Options{})
		defer r.Close()
		err := r.Stage("test", func(kv *KV) error {
			return lverrors.NewErrCorrupted(storage.FileDesc{Type: storage.TypeTable, Num: 5}, errors.New("checksum mismatch"))
		})
		var re *ReadError
		if !errors.As(err, &re) {
			t.Fatalf("want ReadError, got %v", err)
		}
		if re.Retries != 0 || r.ReopenCount() != 0 {
			t.Fatalf("corrupt table must not retry: %+v reopens=%d", re, r.ReopenCount())
		}
		if !strings.Contains(re.Detail, "000005.ldb") {
			t.Fatalf("detail %q does not name the table", re.Detail)
		}
	})

	t.Run("persistent-race-exhausts", func(t *testing.T) {
		r := NewRunner(dir, Options{})
		defer r.Close()
		err := r.Stage("test", func(kv *KV) error {
			return &os.PathError{Op: "read", Path: "000009.ldb", Err: os.ErrNotExist}
		})
		var re *ReadError
		if !errors.As(err, &re) {
			t.Fatalf("want ReadError, got %v", err)
		}
		if re.Retries != r.MaxAttempts-1 || r.ReopenCount() != r.MaxAttempts-1 {
			t.Fatalf("retries %d reopens %d, want %d", re.Retries, r.ReopenCount(), r.MaxAttempts-1)
		}
	})
}

// TestLiveWriterCoexistence: a real goleveldb writer holds LOCK_EX and keeps
// writing/compacting; the lock-free reader opens concurrently and reads
// stable keys; the writer never observes an error.
func TestLiveWriterCoexistence(t *testing.T) {
	dir := newTestDB(t, 500)
	wdb, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()

	// The writer holds the flock now; probe must see it.
	if running, known := ProbeLiveWriter(dir); !known || !running {
		t.Fatalf("probe = running:%v known:%v, want running under writer flock", running, known)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var writerErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			if err := wdb.Put([]byte(fmt.Sprintf("live-%06d", i)), testVal(i), nil); err != nil {
				writerErr = err
				return
			}
			if i%512 == 511 {
				if err := wdb.CompactRange(util.Range{}); err != nil {
					writerErr = err
					return
				}
			}
		}
	}()

	// Open lock-free while the writer runs and read the pre-existing keys.
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	ok := false
	for time.Now().Before(deadline) && !ok {
		func() {
			db, err := Open(dir, Options{})
			if err != nil {
				lastErr = err
				time.Sleep(50 * time.Millisecond)
				return
			}
			defer db.Close()
			latch := &Latch{}
			kv := db.KV(latch)
			for i := 0; i < 500; i++ {
				val, err := kv.Get(testKey(i))
				if err != nil || !bytes.Equal(val, testVal(i)) {
					lastErr = fmt.Errorf("key %d: %q %w", i, val, err)
					return
				}
			}
			if latch.First() != nil {
				lastErr = latch.First()
				return
			}
			ok = true
		}()
	}
	close(stop)
	wg.Wait()
	if writerErr != nil {
		t.Fatalf("writer observed an error: %v", writerErr)
	}
	if !ok {
		t.Fatalf("reader never completed cleanly against the live writer: %v", lastErr)
	}
	// The writer still works after our reads.
	if err := wdb.Put([]byte("post"), []byte("ok"), nil); err != nil {
		t.Fatalf("writer put after coexistence: %v", err)
	}
}

// TestIdleWriterDirectoryUntouched: with an idle writer holding the flock,
// a full read pass leaves the directory byte-identical (no LOCK/LOG
// creation, no manifest rewrite - the geth-wrapper RecoverFile hazard).
func TestIdleWriterDirectoryUntouched(t *testing.T) {
	dir := newTestDB(t, 50)
	wdb, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()

	snapshot := func() map[string][]byte {
		out := map[string][]byte{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, ent := range entries {
			data, err := os.ReadFile(filepath.Join(dir, ent.Name()))
			if err != nil {
				t.Fatal(err)
			}
			out[ent.Name()] = data
		}
		return out
	}
	before := snapshot()

	db, err := Open(dir, Options{})
	if err != nil {
		t.Fatalf("lock-free open under held flock: %v", err)
	}
	latch := &Latch{}
	kv := db.KV(latch)
	it := kv.NewIterator(nil, nil)
	for it.Next() {
	}
	it.Release()
	if err := it.Error(); err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Get(testKey(7)); err != nil {
		t.Fatal(err)
	}
	db.Close()

	after := snapshot()
	if len(before) != len(after) {
		t.Fatalf("file set changed: %d -> %d", len(before), len(after))
	}
	for name, data := range before {
		if !bytes.Equal(after[name], data) {
			t.Fatalf("file %s changed", name)
		}
	}
	if latch.First() != nil {
		t.Fatalf("latch: %v", latch.First())
	}
}

// TestMidScanRelocation: a compaction relocates tables between stages; the
// pinned session either keeps serving (open fds) or the runner reopens -
// stage 2 must succeed either way.
func TestMidScanRelocation(t *testing.T) {
	dir := newTestDB(t, 200)
	r := NewRunner(dir, Options{})
	defer r.Close()

	if err := r.Stage("stage1", func(kv *KV) error {
		_, err := kv.Get(testKey(0))
		return err
	}); err != nil {
		t.Fatalf("stage1: %v", err)
	}

	// Relocate: write + compact through a second (writing) handle.
	wdb, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2000; i++ {
		if err := wdb.Put([]byte(fmt.Sprintf("fill-%06d", i)), bytes.Repeat([]byte{0xab}, 128), nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := wdb.CompactRange(util.Range{}); err != nil {
		t.Fatal(err)
	}
	if err := wdb.Close(); err != nil {
		t.Fatal(err)
	}

	if err := r.Stage("stage2", func(kv *KV) error {
		for i := 0; i < 200; i++ {
			if _, err := kv.Get(testKey(i)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("stage2 after relocation: %v (reopens %d)", err, r.ReopenCount())
	}
}

// TestRelocationReopenDeterministic pins the successful-relocation path:
// every table file vanishes mid-stage (the extreme compaction race), the
// runner reopens exactly once, and the retried stage reads every key
// correctly after the files return.
func TestRelocationReopenDeterministic(t *testing.T) {
	dir := newTestDB(t, 300)
	r := NewRunner(dir, Options{})
	defer r.Close()

	stash := t.TempDir()
	moveTables := func(from, to string) []string {
		entries, err := os.ReadDir(from)
		if err != nil {
			t.Fatal(err)
		}
		var moved []string
		for _, ent := range entries {
			if strings.HasSuffix(ent.Name(), ".ldb") || strings.HasSuffix(ent.Name(), ".sst") {
				if err := os.Rename(filepath.Join(from, ent.Name()), filepath.Join(to, ent.Name())); err != nil {
					t.Fatal(err)
				}
				moved = append(moved, ent.Name())
			}
		}
		if len(moved) == 0 {
			t.Fatal("no table files to relocate")
		}
		return moved
	}

	attempt := 0
	err := r.Stage("scan", func(kv *KV) error {
		attempt++
		if attempt == 1 {
			// Tables vanish before the first read of this session (the
			// open-file cache holds nothing yet), producing a genuine
			// ENOENT through goleveldb - the retryable race class.
			moveTables(dir, stash)
			if _, err := kv.Get(testKey(0)); err != nil {
				return err
			}
			return errors.New("read unexpectedly succeeded with tables missing")
		}
		// Retried attempt: the files are back; every key must read
		// correctly through the fresh session.
		moveTables(stash, dir)
		for i := 0; i < 300; i++ {
			val, err := kv.Get(testKey(i))
			if err != nil {
				return err
			}
			if !bytes.Equal(val, testVal(i)) {
				return fmt.Errorf("key %d reads %q after reopen", i, val)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stage after relocation: %v", err)
	}
	if attempt != 2 {
		t.Fatalf("stage ran %d times, want 2", attempt)
	}
	if r.ReopenCount() != 1 {
		t.Fatalf("reopen count = %d, want exactly 1", r.ReopenCount())
	}
}

// TestUnlatchedReaderScopesLatch: read errors through the Unlatched view
// must not dirty the shared latch (they back the informational head sample),
// while the same error through the primary adapter must.
func TestUnlatchedReaderScopesLatch(t *testing.T) {
	dir := newTestDB(t, 5)
	db, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	latch := &Latch{}
	kv := db.KV(latch)
	unlatched := kv.Unlatched()

	// Force a real (non-not-found) read error on both views.
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := unlatched.Get(testKey(0)); err == nil {
		t.Fatal("read on a closed DB must fail")
	}
	if latch.First() != nil || latch.Count() != 0 {
		t.Fatalf("unlatched read dirtied the shared latch: %v", latch.First())
	}
	if _, err := kv.Get(testKey(0)); err == nil {
		t.Fatal("read on a closed DB must fail")
	}
	if latch.First() == nil {
		t.Fatal("latched read did not record the error")
	}
	// The unlatched view still refuses writes.
	if w, ok := unlatched.(interface{ Put(k, v []byte) error }); !ok {
		t.Fatal("unlatched view lost its type")
	} else if err := w.Put([]byte("k"), []byte("v")); err != ErrWriteRefused {
		t.Fatalf("unlatched Put: %v", err)
	}
}

func TestOpenMissingDB(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "empty"), Options{})
	if err == nil {
		t.Fatal("open of a missing DB must fail (ErrorIfMissing)")
	}
}
