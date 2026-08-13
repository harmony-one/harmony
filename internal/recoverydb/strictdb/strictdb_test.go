package strictdb

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
)

// failingBatch wraps a real batch and fails after N puts.
type failingBatch struct {
	ethdb.Batch
	failAfter int
	puts      int
}

func (b *failingBatch) Put(k, v []byte) error {
	b.puts++
	if b.puts > b.failAfter {
		return errors.New("injected put failure")
	}
	return b.Batch.Put(k, v)
}

type failingKV struct {
	ethdb.Database
	failAfter int
}

func (f *failingKV) NewBatch() ethdb.Batch {
	return &failingBatch{Batch: f.Database.NewBatch(), failAfter: f.failAfter}
}

func TestLatchingBatch(t *testing.T) {
	mem := rawdb.NewMemoryDatabase()
	db := &failingKV{Database: mem, failAfter: 2}
	b := NewLatchingBatch(db, 0)
	if err := b.Put([]byte("a"), []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := b.Put([]byte("b"), []byte("2")); err != nil {
		t.Fatal(err)
	}
	if err := b.Put([]byte("c"), []byte("3")); err == nil {
		t.Fatal("injected failure must surface")
	}
	// Once latched, everything returns the same error and Flush refuses —
	// a partial batch can never be committed.
	if err := b.Put([]byte("d"), []byte("4")); err == nil {
		t.Fatal("latched batch must keep failing")
	}
	if err := b.Flush(); err == nil {
		t.Fatal("latched batch must refuse Flush")
	}
	if err := b.Err(); err == nil {
		t.Fatal("latched error must be exposed")
	}
	if ok, _ := mem.Has([]byte("a")); ok {
		t.Fatal("nothing may reach the database after a latch")
	}

	// Self-flush past the limit works when nothing fails.
	b2 := NewLatchingBatch(rawdb.NewMemoryDatabase(), 4)
	for i := byte(0); i < 10; i++ {
		if err := b2.Put([]byte{i}, []byte{i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b2.Flush(); err != nil {
		t.Fatal(err)
	}
	if b2.Count() != 10 {
		t.Fatalf("count %d", b2.Count())
	}
}

func TestWriteRefusing(t *testing.T) {
	db := NewWriteRefusing(rawdb.NewMemoryDatabase())
	if err := db.Put([]byte("k"), []byte("v")); !errors.Is(err, ErrWriteRefused) {
		t.Fatal(err)
	}
	if err := db.Delete([]byte("k")); !errors.Is(err, ErrWriteRefused) {
		t.Fatal(err)
	}
	if err := db.Compact(nil, nil); !errors.Is(err, ErrWriteRefused) {
		t.Fatal(err)
	}
	b := db.NewBatch()
	if err := b.Put([]byte("k"), []byte("v")); !errors.Is(err, ErrWriteRefused) {
		t.Fatal(err)
	}
	if err := b.Write(); !errors.Is(err, ErrWriteRefused) {
		t.Fatal(err)
	}
}

func TestForEachSurfacesIteratorError(t *testing.T) {
	mem := rawdb.NewMemoryDatabase()
	mem.Put([]byte("p1"), []byte("v"))
	// Callback errors abort.
	err := ForEach(mem, []byte("p"), func(k, v []byte) error {
		return errors.New("callback abort")
	})
	if err == nil {
		t.Fatal("callback error must surface")
	}
	// Clean pass.
	n := 0
	if err := ForEach(mem, []byte("p"), func(k, v []byte) error { n++; return nil }); err != nil || n != 1 {
		t.Fatalf("clean pass: %v n=%d", err, n)
	}
}
