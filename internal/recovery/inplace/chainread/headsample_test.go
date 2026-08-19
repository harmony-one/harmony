package chainread

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
)

// headFaultCounters tracks which adapter view served the failing head-key
// reads.
type headFaultCounters struct {
	latchedHeadReads   int
	unlatchedHeadReads int
}

func isHeadKey(key []byte) bool {
	return bytes.Equal(key, HeadHeaderKey) || bytes.Equal(key, HeadBlockKey)
}

// headFaultReader wraps the latched rodb adapter and injects a read error
// on the head-pointer keys; its Unlatched view injects the same error but
// counts separately, so the test can pin which view sampleHeads used.
type headFaultReader struct {
	kv       *rodb.KV
	counters *headFaultCounters
}

func (r *headFaultReader) Get(key []byte) ([]byte, error) {
	if isHeadKey(key) {
		r.counters.latchedHeadReads++
		return nil, errors.New("injected head read error")
	}
	return r.kv.Get(key)
}

func (r *headFaultReader) Has(key []byte) (bool, error) {
	if isHeadKey(key) {
		r.counters.latchedHeadReads++
		return false, errors.New("injected head read error")
	}
	return r.kv.Has(key)
}

func (r *headFaultReader) Unlatched() ethdb.KeyValueReader {
	return &headFaultUnlatched{inner: r.kv.Unlatched(), counters: r.counters}
}

type headFaultUnlatched struct {
	inner    ethdb.KeyValueReader
	counters *headFaultCounters
}

func (r *headFaultUnlatched) Get(key []byte) ([]byte, error) {
	if isHeadKey(key) {
		r.counters.unlatchedHeadReads++
		return nil, errors.New("injected head read error")
	}
	return r.inner.Get(key)
}

func (r *headFaultUnlatched) Has(key []byte) (bool, error) {
	if isHeadKey(key) {
		r.counters.unlatchedHeadReads++
		return false, errors.New("injected head read error")
	}
	return r.inner.Has(key)
}

// TestHeadSampleReadErrorsDoNotGate: head-pointer read errors are strictly
// informational - the run completes, nothing reaches the shared read-error
// latch, and the sampling happened through the unlatched view.
func TestHeadSampleReadErrorsDoNotGate(t *testing.T) {
	m, err := fixture.Build(filepath.Join(t.TempDir(), "harmony_db_0"), fixture.VariantBase)
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	db, err := rodb.Open(m.Dir, rodb.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	latch := &rodb.Latch{}
	kv := db.KV(latch)

	a, err := anchor.Resolve("localnet", 0, anchor.Overrides{
		TargetHeight: fixture.TargetHeight,
		TargetHash:   m.TargetHash.Hex(),
	})
	if err != nil {
		t.Fatal(err)
	}

	counters := &headFaultCounters{}
	out, err := RunChecks(&headFaultReader{kv: kv, counters: counters}, a, nil)
	if err != nil {
		t.Fatalf("head read errors gated the run: %v", err)
	}
	if latch.First() != nil {
		t.Fatalf("head read errors dirtied the shared latch: %v", latch.First())
	}
	if counters.latchedHeadReads != 0 {
		t.Fatalf("%d head reads went through the latched view, want 0", counters.latchedHeadReads)
	}
	if counters.unlatchedHeadReads == 0 {
		t.Fatal("head sampling never touched the unlatched view")
	}
	if !bytes.Contains([]byte(out.Head.LastHeader), []byte("read-error")) ||
		!bytes.Contains([]byte(out.Head.LastBlock), []byte("read-error")) {
		t.Fatalf("head sample did not record the errors: %+v", out.Head)
	}
	if out.Head.WalkToTarget != "not-walked: no resolvable head pointer" {
		t.Fatalf("walk = %q", out.Head.WalkToTarget)
	}
	// The gating checks all completed against the intact fixture.
	for _, id := range []string{"target_header", "body", "ancestry_to_boundary", "shard_state"} {
		if out.Checks[id] != "ok" {
			t.Fatalf("check %s = %q", id, out.Checks[id])
		}
	}
}
