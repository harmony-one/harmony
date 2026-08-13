package audit

import (
	"path/filepath"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	ldberrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/ethereum/go-ethereum/ethdb"
)

// openSourceKV writes a small source leveldb and returns a plain
// ethdb.KeyValueStore adapter over it (writable, but the overlay never
// writes to it).
func openSourceKV(t *testing.T, kvs map[string]string) (ethdb.KeyValueStore, func()) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "src")
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range kvs {
		if err := db.Put([]byte(k), []byte(v), nil); err != nil {
			t.Fatal(err)
		}
	}
	return &ldbKV{db}, func() { db.Close() }
}

// ldbKV is a minimal ethdb.KeyValueStore over goleveldb for the overlay's
// source side.
type ldbKV struct{ db *leveldb.DB }

func (k *ldbKV) Get(key []byte) ([]byte, error) {
	v, err := k.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, ldberrors.ErrNotFound
	}
	return v, err
}
func (k *ldbKV) Has(key []byte) (bool, error)      { return k.db.Has(key, nil) }
func (k *ldbKV) Put(key, value []byte) error       { return k.db.Put(key, value, nil) }
func (k *ldbKV) Delete(key []byte) error           { return k.db.Delete(key, nil) }
func (k *ldbKV) Stat(p string) (string, error)     { return "", nil }
func (k *ldbKV) Compact(a, b []byte) error         { return nil }
func (k *ldbKV) Close() error                      { return nil }
func (k *ldbKV) NewBatch() ethdb.Batch             { return nil }
func (k *ldbKV) NewBatchWithSize(int) ethdb.Batch  { return nil }
func (k *ldbKV) NewSnapshot() (ethdb.Snapshot, error) { return nil, nil }
func (k *ldbKV) NewIterator(prefix, start []byte) ethdb.Iterator {
	r := util.BytesPrefix(prefix)
	r.Start = append(r.Start, start...)
	return k.db.NewIterator(r, nil)
}

func TestOverlayMaskAndSeed(t *testing.T) {
	src, closeSrc := openSourceKV(t, map[string]string{
		"keep":    "sourceval",
		"masked":  "shouldhide",
		"rewrite": "oldval",
	})
	defer closeSrc()
	o, err := NewOverlay(filepath.Join(t.TempDir(), "scratch"), src)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()

	o.Mask([]byte("masked"))
	if err := o.SeedPut([]byte("rewrite"), []byte("newval")); err != nil {
		t.Fatal(err)
	}
	o.SealSeed()

	// keep -> source value; masked -> absent; rewrite -> materialized.
	if v, _ := o.Get([]byte("keep")); string(v) != "sourceval" {
		t.Fatalf("keep = %q", v)
	}
	if _, err := o.Get([]byte("masked")); err == nil {
		t.Fatal("masked key must read absent")
	}
	if v, _ := o.Get([]byte("rewrite")); string(v) != "newval" {
		t.Fatalf("rewrite = %q, want materialized newval", v)
	}
	// Seed writes are baseline: the post-barrier log is empty.
	if len(o.Log()) != 0 {
		t.Fatalf("post-barrier log should be empty after seeding, got %v", o.Log())
	}

	// A post-barrier write is logged; the source is never touched.
	if err := o.Put([]byte("branchkey"), []byte("bval")); err != nil {
		t.Fatal(err)
	}
	if entry, ok := o.Log()["branchkey"]; !ok || entry.Puts != 1 {
		t.Fatalf("branch write not logged: %v", o.Log())
	}
	if v, _ := src.Get([]byte("branchkey")); v != nil {
		t.Fatal("overlay must never write to the source")
	}
}

func TestOverlayMergedIterationOrdered(t *testing.T) {
	src, closeSrc := openSourceKV(t, map[string]string{
		"p1": "s1", "p2": "s2", "p3": "s3", "p5": "s5",
	})
	defer closeSrc()
	o, err := NewOverlay(filepath.Join(t.TempDir(), "scratch"), src)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	o.Mask([]byte("p2"))                          // hide a source key
	_ = o.SeedPut([]byte("p3"), []byte("OVER"))   // scratch wins tie
	_ = o.SeedPut([]byte("p4"), []byte("scratch4")) // scratch-only key
	o.SealSeed()

	it := o.NewIterator([]byte("p"), nil)
	defer it.Release()
	var gotKeys []string
	var gotVals []string
	for it.Next() {
		gotKeys = append(gotKeys, string(it.Key()))
		gotVals = append(gotVals, string(it.Value()))
	}
	if err := it.Error(); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"p1", "p3", "p4", "p5"} // p2 masked
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("keys = %v, want %v", gotKeys, wantKeys)
	}
	for i := range wantKeys {
		if gotKeys[i] != wantKeys[i] {
			t.Fatalf("key[%d] = %s, want %s (order broken)", i, gotKeys[i], wantKeys[i])
		}
	}
	// p3 must be the scratch value (scratch wins ties).
	if gotVals[1] != "OVER" {
		t.Fatalf("p3 = %s, want scratch OVER", gotVals[1])
	}
}
