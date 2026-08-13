package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

// TestAuditShard1CrossLinkPollution drives the shard-1 crosslink subset and
// the pass-two pollution-clearing path through the REAL two-pass audit loop
// (WS6, review finding 2). The fixture proposes a genuine, validly-signed
// shard-1 crosslink (signed by shard-1's localnet dev committee) at a
// post-target branch block, so the crosslink marker is written to the DB by
// block insertion. On the abandoned branch:
//
//   - PASS 1 (branch crosslink records unmasked) re-runs VerifyBlockCrossLinks
//     and sees the marker already in the DB — an errAlreadyExist pollution
//     finding at that height, classified pollution-suspect.
//   - The pass-1 write log yields a shard-1 crosslink SUBSET (crosslink_block_nums).
//   - PASS 2 masks that crosslink key (seed extra_masked_keys) so
//     VerifyBlockCrossLinks no longer sees a duplicate; the still-embedded
//     crosslink now re-verifies against shard-1's committee and PASSES.
//
// The audit therefore closes cleanly (exit 0): the pollution existed in pass 1
// and was cleared in pass 2. This is the end-to-end coverage the prior rounds
// only had via unit vectors (audit.TestSolver*/TestOverlay*).
func TestAuditShard1CrossLinkPollution(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const (
		blocks   = 46
		target   = 30
		create   = 22
		delegate = 26
		preXlink = 24 // pre-target block carrying shard-1 crosslink (block 3), epoch 2
		xlink    = 40 // post-target branch block carrying shard-1 crosslink (block 4), epoch 3
	)
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:               blocks,
		CreateValidatorAt:    create,
		DelegateAt:           delegate,
		PreCrossLinkShard1At: preXlink,
		CrossLinkShard1At:    xlink,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	anchorPath := filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := metafixture.WriteAnchorConfig(dir, target, blocks, nil, anchorPath); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	code := audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir,
		Scratch:                 filepath.Join(t.TempDir(), "scratch"),
		SkipReserveCheckForTest: true,
	}, os.Stderr)
	rep := readAuditReport(t, outDir)
	if code != 0 {
		for _, a := range rep.Reconciliation.Anomalies {
			t.Logf("ANOMALY kind=%s key=%s detail=%s", a.Kind, a.Key, a.Detail)
		}
		for _, o := range rep.Pass2.FailedOutcomes {
			t.Logf("PASS2 FAIL height=%d fails=%v", o.Height, o.ValidityFails)
		}
		t.Fatalf("shard-1 crosslink audit exit %d verdict %s, want 0", code, rep.Verdict)
	}

	// Subset: shard 1 must appear with the crosslink block number the branch wrote.
	var shard1 *audit.ShardSubset
	for i := range rep.ShardSubsets {
		if rep.ShardSubsets[i].ShardID == 1 {
			shard1 = &rep.ShardSubsets[i]
		}
	}
	if shard1 == nil {
		t.Fatalf("no shard-1 subset extracted from the branch write log: %+v", rep.ShardSubsets)
	}
	// The branch (post-target) wrote shard-1 crosslink block 4.
	if !containsU64(shard1.CrossLinkNums, 4) {
		t.Fatalf("shard-1 subset missing branch crosslink block 4: %+v", shard1.CrossLinkNums)
	}

	// The invariant solver derived the pre-target continuity pointer (3)
	// uniquely from the stored pointer (4) and the pre/branch record sets.
	var solved bool
	for _, p := range rep.Pointers {
		if p.ShardID != 1 {
			continue
		}
		solved = true
		if p.Ambiguous {
			t.Fatalf("shard-1 pointer came out ambiguous: %+v", p)
		}
		if !p.Derived || p.DerivedBlockNum != 3 {
			t.Fatalf("shard-1 pointer derived=%v block=%d, want derived pre-target pointer 3", p.Derived, p.DerivedBlockNum)
		}
	}
	if !solved {
		t.Fatalf("pointer solver did not run for shard 1: %+v", rep.Pointers)
	}

	// Pass 2 masked the crosslink key (pollution seed).
	if rep.Pass2 == nil || rep.Pass2.Seed == nil || rep.Pass2.Seed.ExtraMaskedKeys < 1 {
		t.Fatalf("pass 2 did not mask any branch crosslink/spent key: %+v", rep.Pass2)
	}

	// Pollution appeared in pass 1 and was CLEARED in pass 2: a crosslinks
	// validity failure at the crosslink height in pass 1, none in pass 2.
	if !passHasCrossLinkFail(rep.Pass1, xlink) {
		t.Fatalf("expected a pass-1 crosslinks pollution failure at %d: %+v", xlink, failedHeights(rep.Pass1))
	}
	if passHasCrossLinkFail(rep.Pass2, xlink) {
		t.Fatalf("pass-2 still reports a crosslinks failure at %d (pollution not cleared): %+v", xlink, failedHeights(rep.Pass2))
	}

	// The crosslink write is inventoried, not flagged anomalous.
	if rep.Reconciliation.Writes.CrossLinkSubset < 1 {
		t.Fatalf("crosslink write not inventoried in the reconciliation census: %+v", rep.Reconciliation.Writes)
	}
}

// TestAuditSpentMarkerPollution is the spent-marker analog of the crosslink
// test (WS6, review finding 2): a post-target branch block carries a GENUINE
// incoming cross-shard receipt from shard 1 — the CXReceiptsProof's source
// header is signed by shard-1's dev committee and the merkle proof verifies —
// and the receipt is applied at proposal time, so the stored root includes the
// credit and the audit re-executes the branch to its roots. Block insertion
// wrote the spent marker, so on the abandoned branch:
//
//   - PASS 1 re-runs VerifyIncomingReceipts and IsSpent sees the marker —
//     an errDoubleSpent pollution finding at that height;
//   - the pass-1 write log yields a shard-1 SPENT subset (spent_block_nums);
//   - PASS 2 masks the spent key; the stored receipt then re-verifies through
//     the FULL ValidateCXReceiptsProof chain (trie root, outgoing hash,
//     source-header hash and shard-1 commit signature) and PASSES.
//
// Exit 0: stored incoming receipts re-executed with roots verifying, spent
// pollution cleared in pass 2, spent write inventoried — end-to-end.
func TestAuditSpentMarkerPollution(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const (
		blocks   = 46
		target   = 30
		create   = 22
		delegate = 26
		receipt  = 38 // post-target branch block carrying the incoming receipt (epoch 3)
	)
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:            blocks,
		CreateValidatorAt: create,
		DelegateAt:        delegate,
		IncomingReceiptAt: receipt,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	anchorPath := filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := metafixture.WriteAnchorConfig(dir, target, blocks, nil, anchorPath); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "out")
	code := audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir,
		Scratch:                 filepath.Join(t.TempDir(), "scratch"),
		SkipReserveCheckForTest: true,
	}, os.Stderr)
	rep := readAuditReport(t, outDir)
	if code != 0 {
		for _, a := range rep.Reconciliation.Anomalies {
			t.Logf("ANOMALY kind=%s key=%s detail=%s", a.Kind, a.Key, a.Detail)
		}
		for _, o := range rep.Pass2.FailedOutcomes {
			t.Logf("PASS2 FAIL height=%d fails=%v", o.Height, o.ValidityFails)
		}
		t.Fatalf("spent-marker audit exit %d verdict %s, want 0", code, rep.Verdict)
	}

	// Subset: shard 1 must appear with the spent source block number (5).
	var shard1 *audit.ShardSubset
	for i := range rep.ShardSubsets {
		if rep.ShardSubsets[i].ShardID == 1 {
			shard1 = &rep.ShardSubsets[i]
		}
	}
	if shard1 == nil {
		t.Fatalf("no shard-1 subset extracted from the branch write log: %+v", rep.ShardSubsets)
	}
	if !containsU64(shard1.SpentNums, 5) {
		t.Fatalf("shard-1 subset missing spent marker for source block 5: %+v", shard1.SpentNums)
	}

	// Pass 2 masked the spent key.
	if rep.Pass2 == nil || rep.Pass2.Seed == nil || rep.Pass2.Seed.ExtraMaskedKeys < 1 {
		t.Fatalf("pass 2 did not mask the branch spent key: %+v", rep.Pass2)
	}

	// Double-spent pollution in pass 1, cleared in pass 2.
	if !passHasFailPrefix(rep.Pass1, receipt, "incoming-receipts:") {
		t.Fatalf("expected a pass-1 incoming-receipts pollution failure at %d: %+v", receipt, failedHeights(rep.Pass1))
	}
	if passHasFailPrefix(rep.Pass2, receipt, "incoming-receipts:") {
		t.Fatalf("pass-2 still reports an incoming-receipts failure at %d (pollution not cleared): %+v", receipt, failedHeights(rep.Pass2))
	}

	// The spent write is inventoried, not flagged anomalous.
	if rep.Reconciliation.Writes.SpentSubset < 1 {
		t.Fatalf("spent write not inventoried in the reconciliation census: %+v", rep.Reconciliation.Writes)
	}
}

func containsU64(xs []uint64, v uint64) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func passHasCrossLinkFail(p *audit.PassSection, height uint64) bool {
	return passHasFailPrefix(p, height, "crosslinks:")
}

func passHasFailPrefix(p *audit.PassSection, height uint64, prefix string) bool {
	if p == nil {
		return false
	}
	for _, o := range p.FailedOutcomes {
		if o.Height != height {
			continue
		}
		for _, f := range o.ValidityFails {
			if strings.HasPrefix(f, prefix) {
				return true
			}
		}
	}
	return false
}

func failedHeights(p *audit.PassSection) []uint64 {
	if p == nil {
		return nil
	}
	var out []uint64
	for _, o := range p.FailedOutcomes {
		out = append(out, o.Height)
	}
	return out
}
