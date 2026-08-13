package anchor

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

func validMainnetManifest() *Manifest {
	return &Manifest{
		SchemaVersion:        SchemaVersionV1,
		Network:              "mainnet",
		ShardID:              0,
		TargetHeight:         MainnetTargetHeight,
		TargetHash:           MainnetTargetHash,
		TargetParentHash:     MainnetTargetParentHash,
		TargetEpoch:          3002,
		BaselineHeight:       MainnetPresumedBaselineHeight,
		AbandonedChildHeight: MainnetAbandonedChildHeight,
		AbandonedChildHash:   MainnetAbandonedChildHash,
		RejectedShard1Height: MainnetRejectedShard1Height,
		RejectedShard1Hash:   MainnetRejectedShard1Hash,
	}
}

func writeManifest(t *testing.T, m interface{}) string {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "anchor.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRoundTrip(t *testing.T) {
	path := writeManifest(t, validMainnetManifest())
	m, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.TargetHash != MainnetTargetHash {
		t.Fatalf("target hash mangled")
	}
	if err := m.RequireTargetHeight(MainnetTargetHeight); err != nil {
		t.Fatal(err)
	}
	if err := m.RequireTargetHeight(MainnetTargetHeight + 1); err == nil {
		t.Fatal("CLI/manifest height disagreement must refuse")
	}
}

func TestRejectionVectors(t *testing.T) {
	// Unknown fields rejected (strict JSON, all-or-nothing).
	m := validMainnetManifest()
	raw, _ := json.Marshal(m)
	var loose map[string]interface{}
	json.Unmarshal(raw, &loose)
	loose["surprise_field"] = 1
	path := writeManifest(t, loose)
	if _, err := Load(path); err == nil {
		t.Fatal("unknown field must be rejected")
	}

	// Pinned target hash mismatch refusal (plan WS1 acceptance).
	bad := validMainnetManifest()
	bad.TargetHash = common.HexToHash("0xdead")
	if _, err := Load(writeManifest(t, bad)); err == nil {
		t.Fatal("pinned target hash mismatch must refuse")
	}
	bad = validMainnetManifest()
	bad.TargetParentHash = common.HexToHash("0xdead")
	if _, err := Load(writeManifest(t, bad)); err == nil {
		t.Fatal("pinned parent hash mismatch must refuse")
	}
	bad = validMainnetManifest()
	bad.AbandonedChildHeight = bad.TargetHeight + 2
	if _, err := Load(writeManifest(t, bad)); err == nil {
		t.Fatal("abandoned child height must be target+1")
	}
	bad = validMainnetManifest()
	bad.RejectedShard1Hash = common.HexToHash("0xdead")
	if _, err := Load(writeManifest(t, bad)); err == nil {
		t.Fatal("pinned rejected shard-1 mismatch must refuse")
	}
	bad = validMainnetManifest()
	bad.BaselineHeight = bad.TargetHeight
	if _, err := Load(writeManifest(t, bad)); err == nil {
		t.Fatal("baseline at/above target must refuse")
	}
}

// TestWindowPinnedValues verifies the plan §1 incident values against the
// real mainnet schedule: Window(MainnetSchedule, 92730034) == [92700671,
// 92730034], epoch 3002, 29364 blocks.
func TestWindowPinnedValues(t *testing.T) {
	w, err := ComputeWindow(shardingconfig.MainnetSchedule, MainnetTargetHeight, 0)
	if err != nil {
		t.Fatal(err)
	}
	if w.RetainFrom != 92700671 || w.Target != 92730034 || w.Epoch != 3002 {
		t.Fatalf("window = %+v, want [92700671, 92730034] epoch 3002", w)
	}
	if w.Blocks() != 29364 {
		t.Fatalf("window size %d, want 29364", w.Blocks())
	}
	// --retain-from-height may only extend retention.
	if _, err := ComputeWindow(shardingconfig.MainnetSchedule, MainnetTargetHeight, w.RetainFrom+1); err == nil {
		t.Fatal("shrinking override must refuse")
	}
	w2, err := ComputeWindow(shardingconfig.MainnetSchedule, MainnetTargetHeight, w.RetainFrom-1000)
	if err != nil {
		t.Fatal(err)
	}
	if w2.RetainFrom != w.RetainFrom-1000 {
		t.Fatalf("extension override not applied")
	}
}

// TestEpochLastBlockProperty property-tests
// CalcEpochNumber(EpochLastBlock(e)) == e across the TwoSeconds boundary for
// mainnet and localnet schedules (plan WS1 acceptance).
func TestEpochLastBlockProperty(t *testing.T) {
	shardingconfig.InitLocalnetConfig(16, 16)
	schedules := map[string]shardingconfig.Schedule{
		"mainnet":  shardingconfig.MainnetSchedule,
		"localnet": shardingconfig.LocalnetSchedule,
	}
	// Mainnet TwoSecondsEpoch = 366 (internal/params); sweep wide ranges
	// around small epochs, the TwoSeconds boundary, and the incident epoch.
	ranges := map[string][][2]uint64{
		"mainnet":  {{1, 40}, {360, 372}, {2996, 3006}},
		"localnet": {{1, 12}},
	}
	for name, sched := range schedules {
		for _, r := range ranges[name] {
			for e := r[0]; e <= r[1]; e++ {
				last := sched.EpochLastBlock(e)
				got := sched.CalcEpochNumber(last)
				if got.Uint64() != e {
					t.Errorf("%s: CalcEpochNumber(EpochLastBlock(%d)=%d) = %d", name, e, last, got.Uint64())
				}
				gotNext := sched.CalcEpochNumber(last + 1)
				if gotNext.Uint64() != e+1 {
					t.Errorf("%s: block after EpochLastBlock(%d) is epoch %d, want %d", name, e, gotNext.Uint64(), e+1)
				}
			}
		}
	}
	_ = big.NewInt(0)
}

// TestBloomCheckpoint pins the advanceable-checkpoint math (round 13
// finding 4) against the real mainnet window and the boundary shapes.
func TestBloomCheckpoint(t *testing.T) {
	// Real mainnet window: retainFrom = 92,700,671, target = 92,730,034.
	// retainFrom is the last block of section 22,631; the checkpoint must
	// store count 22,632 so the indexer's next section (22,632, starting
	// at block 92,700,672) needs no pruned headers, with the section head
	// at 22,632*4096-1 = 92,700,671 == retainFrom (retained).
	count, head, ok := BloomCheckpoint(Window{RetainFrom: 92700671, Target: 92730034})
	if !ok || count != 22632 || head != 92700671 {
		t.Fatalf("mainnet checkpoint = (%d, %d, %v), want (22632, 92700671, true)", count, head, ok)
	}
	if next := count * BloomSectionSize; next < 92700671 {
		t.Fatalf("next section start %d needs pruned headers below retainFrom", next)
	}

	// Section-aligned retainFrom: head block must still be retained.
	count, head, ok = BloomCheckpoint(Window{RetainFrom: 8192, Target: 100000})
	if !ok || count != 3 || head != 12287 {
		t.Fatalf("aligned checkpoint = (%d, %d, %v), want (3, 12287, true)", count, head, ok)
	}

	// Genesis-epoch window: full archival, no checkpoint.
	if _, _, ok := BloomCheckpoint(Window{RetainFrom: 0, Target: 5000}); ok {
		t.Fatal("genesis window must not write a checkpoint")
	}
	// Tiny window (localnet fixtures): no section boundary inside, skip.
	if _, _, ok := BloomCheckpoint(Window{RetainFrom: 19, Target: 22}); ok {
		t.Fatal("tiny window must not write a checkpoint")
	}
}
