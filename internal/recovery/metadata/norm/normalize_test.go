package norm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/internal/recovery/report"
	staking "github.com/harmony-one/harmony/staking/types"
)

// cleanBuilder assembles a passing two-validator fixture at the target.
// v1 = addr(1) created at the snapshot base; v2 = addr(2) created at the
// boundary (dual-snapshot edge). Both self-delegate; an external delegator
// addr(9) delegates to v1.
func cleanBuilder(t *testing.T) *builder {
	b := newBuilder(t)
	v1, v2, ext := addr(1), addr(2), addr(9)
	w1 := b.wrapper(v1, tSnapshotBase, ext)
	w2 := b.wrapper(v2, tBoundary)
	b.commit()

	b.writeList([]common.Address{v1, v2})
	// dvl: v1 self (idx 0), ext -> v1 (idx 1), v2 self (idx 0).
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	b.writeDVL(ext, staking.DelegationIndexes{idx(v1, 1, tTarget-1)})
	b.writeDVL(v2, staking.DelegationIndexes{idx(v2, 0, tBoundary)})
	// target-epoch snapshots.
	b.writeSnapshotOf(v1, tEpoch, w1)
	b.writeSnapshotOf(v2, tEpoch, w2)
	// v2 dual-snapshot edge: also a prior-epoch (2) record, preserved.
	b.writeSnapshotOf(v2, tEpoch-1, w2)
	// ss<target-epoch> byte-equals the boundary header ShardState.
	b.writeSS(tEpoch, b.ssBytes)
	// reward accumulator at the target.
	b.writeBlkRwd(tTarget, big.NewInt(12345).Bytes())
	return b
}

func TestCleanFixturePasses(t *testing.T) {
	b := cleanBuilder(t)
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode() != report.ExitOK {
		t.Fatalf("clean fixture exit %d, findings: %+v", res.ExitCode(), res.Findings)
	}
	if res.NormalizedListLength != 2 {
		t.Fatalf("normalized list length %d, want 2", res.NormalizedListLength)
	}
	if len(res.Normalized.Snapshots) != 2 {
		t.Fatalf("retained %d target snapshots, want 2", len(res.Normalized.Snapshots))
	}
	if res.Normalized.RewardAccumulator.Value == nil {
		t.Fatal("reward accumulator must be included (§8 Q5)")
	}
	// Stats never appear in the plan or the normalized set.
	for _, d := range res.Deletions.Deletions() {
		if classifyAssertNS(d.Key) == "validator-stats" {
			t.Fatal("stats key must never be in the deletion plan")
		}
	}
}

func TestDeterminismByteIdentical(t *testing.T) {
	b := cleanBuilder(t)
	r1, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := report.CanonicalJSON(r1.Digests)
	d2, _ := report.CanonicalJSON(r2.Digests)
	if string(d1) != string(d2) {
		t.Fatalf("digests differ across runs:\n%s\n%s", d1, d2)
	}
	p1, _ := report.CanonicalJSON(r1.Deletions)
	p2, _ := report.CanonicalJSON(r2.Deletions)
	if string(p1) != string(p2) {
		t.Fatal("deletion plans differ across runs")
	}
}

func TestPostTargetSnapshotRemoved(t *testing.T) {
	b := cleanBuilder(t)
	// A future-epoch snapshot for v1 and a post-target-created validator's
	// target-epoch snapshot (addr 7, absent from the list).
	w1 := staking.ValidatorWrapper{}
	_ = w1
	b.writeSnapshotRaw(snapshotKey(addr(1), big.NewInt(int64(tEpoch+1))), mustEnc(t, staketestWrapper(addr(1))))
	b.writeSnapshotRaw(snapshotKey(addr(7), big.NewInt(int64(tEpoch))), mustEnc(t, staketestWrapper(addr(7))))
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	var future, postTarget int
	for _, d := range res.Deletions.Deletions() {
		if classifyAssertNS(d.Key) == "validator-snapshot" {
			switch d.Reason {
			case "future-epoch":
				future++
			case "post-target-created":
				postTarget++
			}
		}
	}
	if future != 1 || postTarget != 1 {
		t.Fatalf("snapshot deletions: future=%d post-target=%d (want 1,1)", future, postTarget)
	}
	// Still a clean exit — removals are not fatal.
	if res.ExitCode() != report.ExitOK {
		t.Fatalf("exit %d, findings %+v", res.ExitCode(), res.Findings)
	}
}

func TestDVLFilterBoundary(t *testing.T) {
	// BlockNum == target retained; target+1 removed. Add an external
	// delegator whose two indexes straddle the boundary — but only the
	// retained one may point at a real wrapper slot.
	b := newBuilder(t)
	v1, ext := addr(1), addr(9)
	w1 := b.wrapper(v1, tSnapshotBase, ext)
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	// ext: retained index at target (valid slot 1), plus a post-target
	// index at target+1 that must be filtered.
	b.writeDVL(ext, staking.DelegationIndexes{
		idx(v1, 1, tTarget),
		idx(addr(50), 0, tTarget+1), // post-target -> filtered before validation
	})
	b.writeSnapshotOf(v1, tEpoch, w1)
	b.writeSS(tEpoch, b.ssBytes)
	b.writeBlkRwd(tTarget, big.NewInt(1).Bytes())

	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode() != report.ExitOK {
		t.Fatalf("exit %d findings %+v", res.ExitCode(), res.Findings)
	}
	if len(res.RemovedDVLEntries) != 1 || res.RemovedDVLEntries[0].BlockNum != tTarget+1 {
		t.Fatalf("expected exactly one removed dvl entry at target+1, got %+v", res.RemovedDVLEntries)
	}
	// The retained ext record keeps only the target-block index.
	var extRecord []byte
	for _, r := range res.Normalized.DVL {
		if len(r.Key) == len(dvlKey(ext)) && string(r.Key) == string(dvlKey(ext)) {
			extRecord = r.Value
		}
	}
	if extRecord == nil {
		t.Fatal("ext dvl record missing from normalized set")
	}
	var kept staking.DelegationIndexes
	if err := rlp.DecodeBytes(extRecord, &kept); err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].BlockNum.Uint64() != tTarget {
		t.Fatalf("kept indexes = %+v, want single target-block index", kept)
	}
}

func TestNoncanonicalSnapshotKeyFatal(t *testing.T) {
	b := cleanBuilder(t)
	// Leading-zero alias suffix for epoch 3 (0x03 -> 0x0003).
	alias := append(append([]byte("validator-snapshot"), addr(1).Bytes()...), 0x00, 0x03)
	b.writeSnapshotRaw(alias, mustEnc(t, staketestWrapper(addr(1))))
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	f := findFinding(res, "noncanonical-epoch-suffix")
	if f == nil || !f.fatal || f.class != string(report.ClassNoncanonicalKey) {
		t.Fatalf("expected fatal NoncanonicalKey finding, got %+v", f)
	}
	// Canonical + alias for one logical record => also a duplicate fatal.
	if findFinding(res, "duplicate-logical-record") == nil {
		t.Fatal("expected duplicate-logical-record finding for canonical+alias pair")
	}
	if res.ExitCode() != report.ExitInvalidRetained {
		t.Fatalf("exit %d, want 21", res.ExitCode())
	}
}

func TestMissingTargetSnapshotFallback(t *testing.T) {
	// Rebuild without v1's target-epoch snapshot.
	b2 := newBuilder(t)
	v1 := addr(1)
	w1 := b2.wrapper(v1, tSnapshotBase)
	b2.commit()
	b2.writeList([]common.Address{v1})
	b2.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	// No snapshot for v1.
	b2.writeSS(tEpoch, b2.ssBytes)
	b2.writeBlkRwd(tTarget, big.NewInt(1).Bytes())
	_ = w1
	res, err := Normalize(testAnchor(), b2.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "snapshot-missing-for-listed") == nil {
		t.Fatal("expected snapshot-missing-for-listed fallback finding")
	}
	if res.ExitCode() != report.ExitMissingRequired {
		t.Fatalf("exit %d, want 20", res.ExitCode())
	}
}

func TestMissingSSFallback(t *testing.T) {
	b := newBuilder(t)
	v1 := addr(1)
	w1 := b.wrapper(v1, tSnapshotBase)
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	b.writeSnapshotOf(v1, tEpoch, w1)
	// No ss<target-epoch>.
	b.writeBlkRwd(tTarget, big.NewInt(1).Bytes())
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "shard-state-missing") == nil {
		t.Fatal("expected shard-state-missing fallback")
	}
	if res.ExitCode() != report.ExitMissingRequired {
		t.Fatalf("exit %d, want 20", res.ExitCode())
	}
}

func TestMissingBlkRwdFallback(t *testing.T) {
	b := newBuilder(t)
	v1 := addr(1)
	w1 := b.wrapper(v1, tSnapshotBase)
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	b.writeSnapshotOf(v1, tEpoch, w1)
	b.writeSS(tEpoch, b.ssBytes)
	// No blk-rwd-target.
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "blk-rwd-target-missing") == nil {
		t.Fatal("expected blk-rwd-target-missing fallback")
	}
	if res.ExitCode() != report.ExitMissingRequired {
		t.Fatalf("exit %d, want 20", res.ExitCode())
	}
}

func TestMissingDVLIndexFallback(t *testing.T) {
	b := newBuilder(t)
	v1, ext := addr(1), addr(9)
	w1 := b.wrapper(v1, tSnapshotBase, ext) // wrapper has ext delegation...
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	// ...but no reverse dvl entry for ext -> v1.
	b.writeSnapshotOf(v1, tEpoch, w1)
	b.writeSS(tEpoch, b.ssBytes)
	b.writeBlkRwd(tTarget, big.NewInt(1).Bytes())
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "dvl-missing-required-index") == nil {
		t.Fatal("expected dvl-missing-required-index fallback")
	}
	if res.ExitCode() != report.ExitMissingRequired {
		t.Fatalf("exit %d, want 20", res.ExitCode())
	}
}

func TestInvalidListDecodeFatal(t *testing.T) {
	b := newBuilder(t)
	b.commit()
	b.writeRawList([]byte{0xde, 0xad, 0xbe, 0xef})
	b.writeSS(tEpoch, b.ssBytes)
	b.writeBlkRwd(tTarget, big.NewInt(1).Bytes())
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "validator-list-undecodable") == nil {
		t.Fatal("expected validator-list-undecodable fatal")
	}
	if res.ExitCode() != report.ExitInvalidRetained {
		t.Fatalf("exit %d, want 21", res.ExitCode())
	}
}

func TestDVLPointerMismatchFatal(t *testing.T) {
	b := newBuilder(t)
	v1, ext := addr(1), addr(9)
	w1 := b.wrapper(v1, tSnapshotBase, ext)
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	// ext points at slot 0, which belongs to v1 (self), not ext.
	b.writeDVL(ext, staking.DelegationIndexes{idx(v1, 0, tTarget-1)})
	b.writeSnapshotOf(v1, tEpoch, w1)
	b.writeSS(tEpoch, b.ssBytes)
	b.writeBlkRwd(tTarget, big.NewInt(1).Bytes())
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if findFinding(res, "dvl-pointer-mismatch") == nil {
		t.Fatal("expected dvl-pointer-mismatch fatal")
	}
	if res.ExitCode() != report.ExitInvalidRetained {
		t.Fatalf("exit %d, want 21", res.ExitCode())
	}
}

func TestMixedExitPrecedence(t *testing.T) {
	// MISSING_REQUIRED + INVALID_RETAINED -> 21 (corruption outranks the
	// fallback signal).
	b := newBuilder(t)
	v1 := addr(1)
	w1 := b.wrapper(v1, tSnapshotBase)
	b.commit()
	b.writeList([]common.Address{v1})
	b.writeDVL(v1, staking.DelegationIndexes{idx(v1, 0, tSnapshotBase)})
	b.writeSnapshotOf(v1, tEpoch, w1)
	// Corrupt: noncanonical ss alias (INVALID) AND missing blk-rwd
	// (MISSING).
	b.writeSS(tEpoch, b.ssBytes)
	b.writeSnapshotRaw([]byte("ss\x00\x03"), b.ssBytes) // noncanonical ss<3> alias
	res, err := Normalize(testAnchor(), b.sources())
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode() != report.ExitInvalidRetained {
		t.Fatalf("mixed exit %d, want 21", res.ExitCode())
	}
}

func mustEnc(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := rlp.EncodeToBytes(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func staketestWrapper(a common.Address) *staking.ValidatorWrapper {
	w := staketestWrapperValue(a)
	return &w
}
