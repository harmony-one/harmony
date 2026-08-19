package chainread

import (
	"math/big"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core/rawdb"
	bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

func testCommittee(epoch int64) *shard.State {
	stake := numeric.NewDec(1)
	return &shard.State{
		Epoch: big.NewInt(epoch),
		Shards: []shard.Committee{{
			ShardID: 0,
			Slots: shard.SlotList{{
				BLSPublicKey:   bls.SerializedPublicKey{0x01},
				EffectiveStake: &stake,
			}},
		}},
	}
}

// TestShardStateWrongEpoch: byte-equality passes (boundary header carries
// the same bytes) but the decoded epoch does not match the anchor - FAIL.
func TestShardStateWrongEpoch(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	a := &anchor.Anchor{Epoch: big.NewInt(3), ShardID: 0}

	wrongEpochState := testCommittee(4) // decodes fine, epoch mismatch
	raw, err := shard.EncodeWrapper(*wrongEpochState, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := rawdb.WriteShardStateBytes(db, big.NewInt(3), raw); err != nil {
		t.Fatal(err)
	}
	boundary := blockfactory.NewFactory(params.LocalnetChainConfig).NewHeader(big.NewInt(2))
	boundary.SetShardState(raw)

	o := &Outcome{BoundaryHeader: boundary, Checks: report.NewChecks()}
	err = o.checkShardState(db, a)
	f, ok := err.(*report.Failure)
	if !ok || !strings.Contains(f.Reason, "epoch") {
		t.Fatalf("want epoch-mismatch failure, got %v", err)
	}
}

// TestShardStateUndecodable: present, byte-equal, but not valid RLP.
func TestShardStateUndecodable(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	a := &anchor.Anchor{Epoch: big.NewInt(3), ShardID: 0}
	raw := []byte{0xde, 0xad}
	if err := rawdb.WriteShardStateBytes(db, big.NewInt(3), raw); err != nil {
		t.Fatal(err)
	}
	boundary := blockfactory.NewFactory(params.LocalnetChainConfig).NewHeader(big.NewInt(2))
	boundary.SetShardState(raw)
	o := &Outcome{BoundaryHeader: boundary, Checks: report.NewChecks()}
	err := o.checkShardState(db, a)
	f, ok := err.(*report.Failure)
	if !ok || !strings.Contains(f.Reason, "decode") {
		t.Fatalf("want decode failure, got %v", err)
	}
}

// TestMinimalChainReaderFailsClosed: every method outside the audited set
// panics with UnexpectedCallError.
func TestMinimalChainReaderFailsClosed(t *testing.T) {
	r := NewMinimalChainReader(params.LocalnetChainConfig, 0, nil, big.NewInt(3), testCommittee(3))

	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			rec := recover()
			uce, ok := rec.(*UnexpectedCallError)
			if !ok {
				t.Fatalf("%s: want UnexpectedCallError panic, got %v", name, rec)
			}
			if uce.Method != name {
				t.Fatalf("panic names %s, want %s", uce.Method, name)
			}
		}()
		fn()
	}
	mustPanic("GetHeaderByNumber", func() { r.GetHeaderByNumber(1) })
	mustPanic("TrieDB", func() { r.TrieDB() })
	mustPanic("CurrentBlock", func() { r.CurrentBlock() })
	mustPanic("ReadCommitSig", func() { _, _ = r.ReadCommitSig(1) })
	mustPanic("Snapshots", func() { r.Snapshots() })

	// Audited set answers.
	if r.Config() != params.LocalnetChainConfig || r.ShardID() != 0 {
		t.Fatal("audited methods broken")
	}
	if _, err := r.ReadShardState(big.NewInt(3)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadShardState(big.NewInt(4)); err == nil {
		t.Fatal("foreign epoch must be refused")
	}
	called := r.CalledMethods()
	for m := range called {
		switch m {
		case "Config", "CurrentHeader", "ShardID", "ReadShardState":
		default:
			t.Fatalf("unexpected recorded method %s", m)
		}
	}
}
