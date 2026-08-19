package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

// The CXReceiptsProof.Copy bug (core/types/cx_receipt.go, present since
// 2019-09-09) makes rawdb.WriteBlock store every incoming-receipt-carrying
// body with CommitBitmap set to a byte-copy of the 96-byte CommitSig — the
// quorum bitmap is simply gone from the stored body. The production fix is
// out of scope for this recovery branch (no consensus/core changes), so the
// fixture reproduces the mainnet on-disk shape NATIVELY: generation goes
// through the stock WriteBlock and stores the corrupted body, while the true
// bitmap survives only in the separately stored source-shard crosslink —
// exactly as on the mainnet beacon chain. The audit restores the bitmap from
// that crosslink and proves the restoration against the header's
// IncomingReceiptHash commitment (audit/legacybitmap.go).

// fixtureSrcShard1Block is the shard-1 source block number the fixture's
// incoming receipt references (fixture.makeShard1IncomingReceipt call site).
const fixtureSrcShard1Block = 5

// assertStoredReceiptBitmapCorrupted pins the mainnet on-disk shape on the
// freshly generated fixture: the stored proof's CommitBitmap must be the
// Copy-bug signature copy. This is a canary — if the production Copy bug is
// ever fixed, newly generated fixtures stop exhibiting the corruption and
// this assertion fails, signalling that the restoration path lost its
// natural fixture coverage and these tests must re-apply the corruption
// manually to keep covering mainnet-era DBs.
func assertStoredReceiptBitmapCorrupted(t *testing.T, dir string, n uint64) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash := rawdb.ReadCanonicalHash(db, n)
	if hash == (common.Hash{}) {
		t.Fatalf("no canonical hash at %d", n)
	}
	body := rawdb.ReadBody(db, hash, n)
	if body == nil {
		t.Fatalf("no body at %d", n)
	}
	cxps := body.IncomingReceipts()
	if len(cxps) == 0 {
		t.Fatalf("block %d carries no incoming receipts", n)
	}
	if len(cxps[0].CommitSig) != 96 || !bytes.Equal(cxps[0].CommitBitmap, cxps[0].CommitSig) {
		t.Fatalf("stored proof at %d does not exhibit the legacy Copy-bug corruption "+
			"(bitmap %d bytes, sig %d bytes) — was CXReceiptsProof.Copy fixed? These tests "+
			"must then corrupt the stored body manually to keep covering mainnet-era DBs",
			n, len(cxps[0].CommitBitmap), len(cxps[0].CommitSig))
	}
}

// mutateStoredReceiptProof flips one byte of the stored proof's merkle-proof
// shard hash at height n: execution (which applies only the receipts) is
// untouched, so the block still re-executes to its stored root, but the
// stored proof no longer verifies and the header commitment breaks.
func mutateStoredReceiptProof(t *testing.T, dir string, n uint64) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash := rawdb.ReadCanonicalHash(db, n)
	body := rawdb.ReadBody(db, hash, n)
	if body == nil {
		t.Fatalf("no body at %d", n)
	}
	cxps := body.IncomingReceipts()
	if len(cxps) == 0 || cxps[0].MerkleProof == nil || len(cxps[0].MerkleProof.CXShardHashes) == 0 {
		t.Fatalf("block %d has no receipt merkle proof to mutate", n)
	}
	cxps[0].MerkleProof.CXShardHashes[0][0] ^= 0xff
	body.SetIncomingReceipts(cxps)
	if err := rawdb.WriteBody(db, hash, n, body); err != nil {
		t.Fatal(err)
	}
}

// removeShard1CrossLink deletes the stored shard-1 crosslink record for the
// fixture's incoming-receipt source block, removing the audit's only
// restoration source.
func removeShard1CrossLink(t *testing.T, dir string) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := rawdb.ReadCrossLinkShardBlock(db, 1, fixtureSrcShard1Block); err != nil {
		t.Fatalf("fixture did not store the shard-1 crosslink to remove: %v", err)
	}
	if err := rawdb.DeleteCrossLinkShardBlock(db, 1, fixtureSrcShard1Block); err != nil {
		t.Fatal(err)
	}
}

// buildSpentFixture generates the incoming-receipt fixture shared by the
// legacy-bitmap tests (same shape as TestAuditSpentMarkerPollution).
func buildSpentFixture(t *testing.T, receiptAt uint64) (dir, anchorPath string) {
	t.Helper()
	const (
		blocks   = 46
		target   = 30
		create   = 22
		delegate = 26
	)
	dir = filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:            blocks,
		CreateValidatorAt: create,
		DelegateAt:        delegate,
		IncomingReceiptAt: receiptAt,
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	anchorPath = filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := metafixture.WriteAnchorConfig(dir, target, blocks, nil, anchorPath); err != nil {
		t.Fatal(err)
	}
	return dir, anchorPath
}

func runAuditOn(t *testing.T, dir, anchorPath, outDir string) int {
	t.Helper()
	return audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir,
		Scratch:                 filepath.Join(t.TempDir(), "scratch"),
		SkipReserveCheckForTest: true,
	}, os.Stderr)
}

// TestAuditLegacyReceiptBitmapRestored proves the audit runs cleanly against
// the MAINNET on-disk shape, produced natively by the fixture: the stored
// body's CommitBitmap is the Copy-bug signature copy, and the matching
// crosslink exists (as it does on the beacon chain for every source-shard
// block). The audit must restore the bitmap from the crosslink, prove the
// restoration against the header's incoming-receipt commitment, and then
// pass the FULL proof verification in pass 2 — exit 0, with the restoration
// inventoried in both pass sections.
func TestAuditLegacyReceiptBitmapRestored(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const receipt = 38
	dir, anchorPath := buildSpentFixture(t, receipt)
	assertStoredReceiptBitmapCorrupted(t, dir, receipt)

	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditOn(t, dir, anchorPath, outDir)
	rep := readAuditReport(t, outDir)
	if code != 0 {
		for _, a := range rep.Reconciliation.Anomalies {
			t.Logf("ANOMALY kind=%s key=%s detail=%s", a.Kind, a.Key, a.Detail)
		}
		for _, o := range rep.Pass2.FailedOutcomes {
			t.Logf("PASS2 FAIL height=%d fails=%v", o.Height, o.ValidityFails)
		}
		t.Fatalf("legacy-bitmap audit exit %d verdict %s, want 0", code, rep.Verdict)
	}
	if rep.Pass1.LegacyBitmapsRestored < 1 || rep.Pass2.LegacyBitmapsRestored < 1 {
		t.Fatalf("legacy bitmap restoration not inventoried: pass1=%d pass2=%d",
			rep.Pass1.LegacyBitmapsRestored, rep.Pass2.LegacyBitmapsRestored)
	}
	// The restored proof passes the FULL verification chain in pass 2 (the
	// pass-1 failure is only the expected spent-marker pollution).
	if passHasFailPrefix(rep.Pass2, receipt, "incoming-receipts:") {
		t.Fatalf("pass-2 incoming-receipts failure remains despite restoration: %+v", failedHeights(rep.Pass2))
	}
}

// TestAuditLegacyReceiptBitmapUnrestorable pins the fail-closed side: the
// same corruption WITHOUT a stored crosslink cannot be verifiably restored,
// so the stored proof must fail validation (the bitmap truly is gone) and
// gate the audit at 24 — restoration never guesses. The block still
// re-executes to its stored root (the corruption is in the proof material,
// not the applied receipts), which is exactly the "mutated stored incoming
// receipts with roots verifying" shape.
func TestAuditLegacyReceiptBitmapUnrestorable(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const receipt = 38
	dir, anchorPath := buildSpentFixture(t, receipt)
	assertStoredReceiptBitmapCorrupted(t, dir, receipt)
	removeShard1CrossLink(t, dir) // restoration source gone

	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditOn(t, dir, anchorPath, outDir)
	rep := readAuditReport(t, outDir)
	if code != 24 {
		t.Fatalf("unrestorable legacy-bitmap audit exit %d, want 24", code)
	}
	if !passHasFailPrefix(rep.Pass2, receipt, "incoming-receipts:") {
		t.Fatalf("expected a pass-2 incoming-receipts failure at %d: %+v", receipt, failedHeights(rep.Pass2))
	}
	assertRootMatchedFailure(t, rep.Pass2, receipt)
	assertUnexpectedValidityAnomaly(t, rep, receipt)
}

// TestAuditMutatedIncomingReceiptDetected is the direct review-finding case:
// a stored incoming receipt whose PROOF material was mutated post-hoc (merkle
// shard hash flipped) while execution — and therefore the state root — is
// untouched. The audit must re-execute the block to its root AND still flag
// the mutation via the failed proof verification, gating at 24. The
// crosslink restoration source is still present, so the audit ATTEMPTS the
// bitmap restoration — but the mutated proof bytes can no longer reproduce
// the header's incoming-receipt commitment, so the substitution is rolled
// back and nothing is silently repaired.
func TestAuditMutatedIncomingReceiptDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const receipt = 38
	dir, anchorPath := buildSpentFixture(t, receipt)
	mutateStoredReceiptProof(t, dir, receipt)

	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditOn(t, dir, anchorPath, outDir)
	rep := readAuditReport(t, outDir)
	if code != 24 {
		t.Fatalf("mutated-receipt audit exit %d, want 24", code)
	}
	if !passHasFailPrefix(rep.Pass2, receipt, "incoming-receipts:") {
		t.Fatalf("expected a pass-2 incoming-receipts failure at %d: %+v", receipt, failedHeights(rep.Pass2))
	}
	assertRootMatchedFailure(t, rep.Pass2, receipt)
	assertUnexpectedValidityAnomaly(t, rep, receipt)
	// The commitment check must have rolled the substitution back: no
	// restoration may be counted for a proof whose bytes were tampered.
	if rep.Pass1.LegacyBitmapsRestored != 0 || rep.Pass2.LegacyBitmapsRestored != 0 {
		t.Fatalf("restoration fired on a tampered proof: pass1=%d pass2=%d",
			rep.Pass1.LegacyBitmapsRestored, rep.Pass2.LegacyBitmapsRestored)
	}
}

// assertRootMatchedFailure requires the failed outcome at height to have
// re-executed to its stored root — the failure is proof-material-only.
func assertRootMatchedFailure(t *testing.T, p *audit.PassSection, height uint64) {
	t.Helper()
	for _, o := range p.FailedOutcomes {
		if o.Height == height {
			if !o.Executed || !o.RootMatched {
				t.Fatalf("outcome at %d executed=%v root_matched=%v, want both true (roots must verify)", height, o.Executed, o.RootMatched)
			}
			return
		}
	}
	t.Fatalf("no failed outcome at %d: %+v", height, failedHeights(p))
}

func assertUnexpectedValidityAnomaly(t *testing.T, rep *audit.Report, height uint64) {
	t.Helper()
	want := fmt.Sprintf("%d", height)
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "unexpected-validity-failure" && a.Key == want {
			return
		}
	}
	t.Fatalf("no unexpected-validity-failure anomaly at %d: %+v", height, rep.Reconciliation.Anomalies)
}
