package acceptance

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	staking "github.com/harmony-one/harmony/staking/types"
)

// ---- cold-DB raw tamper helpers (WS6 injected-fault matrix; be8 lives
// in scan_codes_test.go) ----

// tamperBlockSig flips a byte inside the raw block-sig-N record (the
// aggregate commit signature): seal verification of block N must fail as a
// recorded finding while everything else stays intact.
func tamperBlockSig(t *testing.T, dir string, n uint64) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	key := append([]byte("block-sig-"), be8(n)...)
	v, err := db.Get(key)
	if err != nil {
		t.Fatalf("read block-sig-%d: %v", n, err)
	}
	v = append([]byte(nil), v...)
	v[0] ^= 0xff
	if err := db.Put(key, v); err != nil {
		t.Fatal(err)
	}
}

// tamperHeaderRoot rewrites branch header N in place (same h+num+hash key)
// with a corrupted state root: re-execution to the original root must
// diverge from the tampered header, a Fatal abort naming the height.
func tamperHeaderRoot(t *testing.T, dir string, n uint64) {
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
	hdr := rawdb.ReadHeader(db, hash, n)
	if hdr == nil {
		t.Fatalf("no header at %d", n)
	}
	hdr.SetRoot(common.HexToHash("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
	enc, err := rlp.EncodeToBytes(hdr)
	if err != nil {
		t.Fatal(err)
	}
	key := append(append([]byte("h"), be8(n)...), hash.Bytes()...)
	if err := db.Put(key, enc); err != nil {
		t.Fatal(err)
	}
}

// tamperIncomingReceiptHash rewrites header N in place with a nonzero
// IncomingReceiptHash while the block carries no incoming receipts: the
// patched incoming-receipts validation must fail as a recorded finding
// (this is the exploit signature the known-bad gate keys on).
func tamperIncomingReceiptHash(t *testing.T, dir string, n uint64) {
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
	hdr := rawdb.ReadHeader(db, hash, n)
	if hdr == nil {
		t.Fatalf("no header at %d", n)
	}
	hdr.SetIncomingReceiptHash(common.HexToHash("0xbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbad01"))
	enc, err := rlp.EncodeToBytes(hdr)
	if err != nil {
		t.Fatal(err)
	}
	key := append(append([]byte("h"), be8(n)...), hash.Bytes()...)
	if err := db.Put(key, enc); err != nil {
		t.Fatal(err)
	}
}

// tamperLastCommit corrupts header N's embedded LastCommitSignature in
// place: engine.VerifyHeader(seal=true) — the mandatory general header
// verification that InsertChain(..., false) skips — must fail as a recorded
// verify-header finding.
func tamperLastCommit(t *testing.T, dir string, n uint64) {
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
	hdr := rawdb.ReadHeader(db, hash, n)
	if hdr == nil {
		t.Fatalf("no header at %d", n)
	}
	sig := hdr.LastCommitSignature()
	sig[0] ^= 0xff
	hdr.SetLastCommitSignature(sig)
	enc, err := rlp.EncodeToBytes(hdr)
	if err != nil {
		t.Fatal(err)
	}
	key := append(append([]byte("h"), be8(n)...), hash.Bytes()...)
	if err := db.Put(key, enc); err != nil {
		t.Fatal(err)
	}
}

// redirectCanonicalMapping repoints canonical(n) at a WRONG hash and moves
// the true header/body/receipt records under that wrong key (the originals
// are deleted — the true records exist ONLY under the wrong key). Every
// record remains individually valid: ancestry holds, execution reproduces
// the true roots, and every signature verifies over the true content. The
// ONLY defect is the mapping/record identity mismatch, so this fixture
// isolates the mandatory source-identity validation.
func redirectCanonicalMapping(t *testing.T, dir string, n uint64) {
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
	wrong := hash
	wrong[31] ^= 0xff
	for _, prefix := range []byte{'h', 'b', 'r'} {
		key := append(append([]byte{prefix}, be8(n)...), hash.Bytes()...)
		v, err := db.Get(key)
		if err != nil {
			if prefix == 'r' {
				continue // receipts are optional at this height
			}
			t.Fatalf("read %c record at %d: %v", prefix, n, err)
		}
		if err := db.Put(append(append([]byte{prefix}, be8(n)...), wrong.Bytes()...), v); err != nil {
			t.Fatal(err)
		}
		if err := db.Delete(key); err != nil {
			t.Fatal(err)
		}
	}
	canonical := append(append([]byte("h"), be8(n)...), 'n')
	if err := db.Put(canonical, wrong.Bytes()); err != nil {
		t.Fatal(err)
	}
}

// plantFutureSS copies the epoch-2 shard state to the never-written key
// ss<5>: a planned future-epoch deletion the branch can never reproduce.
func plantFutureSS(t *testing.T, dir string) {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v, err := db.Get(append([]byte("ss"), 0x02))
	if err != nil {
		t.Fatalf("read ss<2>: %v", err)
	}
	if err := db.Put(append([]byte("ss"), 0x05), v); err != nil {
		t.Fatal(err)
	}
}

func readAuditReport(t *testing.T, outDir string) *audit.Report {
	t.Helper()
	var rep audit.Report
	raw, err := os.ReadFile(filepath.Join(outDir, "abandoned-branch-audit.json"))
	if err != nil {
		t.Fatalf("read audit report: %v", err)
	}
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	return &rep
}

// TestAuditTamperedSealFinding is the WS6 header-validation fixture: a
// raw-tampered aggregate commit signature (block-sig of the head, the only
// height whose seal source is the exact key rather than the child header)
// must surface as a recorded seal-verification finding at that height —
// the InsertChain(..., false)-skips-headers regression — and, being a
// validity failure outside the anchored known-bad list, gate the audit to
// exit 24.
func TestAuditTamperedSealFinding(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	tamperBlockSig(t, dir, fxBlocks)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("tampered-seal audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	var sealFail bool
	for _, o := range rep.Pass2.FailedOutcomes {
		if o.Height != fxBlocks {
			t.Fatalf("unexpected validity failure at %d: %+v", o.Height, o)
		}
		for _, f := range o.ValidityFails {
			if strings.Contains(f, "seal") || strings.Contains(f, "signature") {
				sealFail = true
			}
		}
	}
	if !sealFail {
		t.Fatalf("expected a seal/signature validity failure at %d: %+v", fxBlocks, rep.Pass2.FailedOutcomes)
	}
	// Execution itself is unaffected: every root still matched.
	if rep.Pass2.RootsMatched != rep.Pass2.ExecutedBlocks {
		t.Fatal("a seal tamper must not affect execution roots")
	}
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "unexpected-validity-failure" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an unexpected-validity-failure anomaly, got %+v", rep.Reconciliation.Anomalies)
	}
}

// TestAuditRootMismatchFatal is the WS6 root-mismatch fixture: a branch
// header whose state root is raw-tampered in place must abort the audit as
// Fatal naming the height (state-root divergence on the branch would be a
// major discovery; the report is still written).
func TestAuditRootMismatchFatal(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const tamperAt = fxPostCreate // any branch height works
	dir := buildFixture(t)
	tamperHeaderRoot(t, dir, tamperAt)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code == 0 {
		t.Fatal("root-mismatch audit must not exit 0")
	}
	rep := readAuditReport(t, outDir)
	if rep.Pass1 == nil || !rep.Pass1.Fatal {
		t.Fatalf("expected a Fatal pass-1 abort, got %+v", rep.Pass1)
	}
	if rep.Pass1.FatalHeight != tamperAt {
		t.Fatalf("fatal at height %d, want %d", rep.Pass1.FatalHeight, tamperAt)
	}
}

// TestAuditTamperedHeaderVerifyHeaderFinding pins that the mandatory
// engine.VerifyHeader(seal=true) call — the general header verification
// InsertChain(..., false) skips entirely — runs per branch block and
// records failures: a corrupted embedded LastCommitSignature on the head
// header must surface as a verify-header validity finding at that height
// and, being outside the (empty) known-bad list, gate the audit to 24.
func TestAuditTamperedHeaderVerifyHeaderFinding(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	tamperLastCommit(t, dir, fxBlocks)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("tampered-header audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	var verifyHeaderFail bool
	for _, o := range rep.Pass2.FailedOutcomes {
		// The tampered head header is ALSO block N-1's commit-sig source
		// (CommitSigFor reads the child header), so a collateral
		// header-signature failure at N-1 is expected alongside the
		// verify-header failure at N.
		if o.Height != fxBlocks && o.Height != fxBlocks-1 {
			t.Fatalf("unexpected validity failure at %d: %+v", o.Height, o)
		}
		if o.Height != fxBlocks {
			continue
		}
		for _, f := range o.ValidityFails {
			if strings.HasPrefix(f, "verify-header:") {
				verifyHeaderFail = true
			}
		}
	}
	if !verifyHeaderFail {
		t.Fatalf("expected a verify-header validity failure at %d: %+v", fxBlocks, rep.Pass2.FailedOutcomes)
	}
}

// TestAuditRedirectedCanonicalMappingCannotExitZero is the source-identity
// end-to-end fixture: canonical(n) is redirected to a wrong hash whose
// records are the TRUE block's bytes (stored only under the wrong key).
// Ancestry holds, execution reproduces the true roots, and every signature
// verifies over the true content — no ancestry/cryptographic/execution
// check can convict the redirect. The audit must still record the identity
// mismatch as a mandatory source-identity validity failure at n (plus the
// collateral commit-sig-source finding at n-1, whose extracted material
// still verifies) and gate to exit 24 — such a DB can never exit 0.
func TestAuditRedirectedCanonicalMappingCannotExitZero(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const redirectAt = fxPostCreate // any branch height works
	dir := buildFixture(t)
	redirectCanonicalMapping(t, dir, redirectAt)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("redirected-mapping audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	var sawBlockIdentity, sawSigIdentity bool
	for _, o := range rep.Pass2.FailedOutcomes {
		if o.Height != redirectAt && o.Height != redirectAt-1 {
			t.Fatalf("unexpected validity failure at %d: %+v", o.Height, o)
		}
		for _, f := range o.ValidityFails {
			if !strings.HasPrefix(f, "source-identity:") {
				t.Fatalf("only source-identity failures expected (everything else verifies), got at %d: %s", o.Height, f)
			}
			if o.Height == redirectAt {
				sawBlockIdentity = true
			} else {
				sawSigIdentity = true
			}
		}
	}
	if !sawBlockIdentity || !sawSigIdentity {
		t.Fatalf("want source-identity failures at %d (block) and %d (commit-sig source), got %+v",
			redirectAt, redirectAt-1, rep.Pass2.FailedOutcomes)
	}
	// The redirect gates through the known-bad cross-check (empty list →
	// unexpected-validity-failure anomalies).
	var anom bool
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "unexpected-validity-failure" {
			anom = true
		}
	}
	if !anom {
		t.Fatalf("expected unexpected-validity-failure anomalies, got %+v", rep.Reconciliation.Anomalies)
	}
	// Execution itself is untouched: the content is the true block, so
	// every executed root still matched.
	if rep.Pass2.RootsMatched != rep.Pass2.ExecutedBlocks {
		t.Fatal("a pure mapping redirect must not affect execution roots")
	}
}

// TestAuditKnownBadExtraFailureAnomalous is the F1 round-2 pin: a known-bad
// height reproduces the expected incoming-receipts exploit failure (so the
// exploit signature IS cross-checked) but the same tamper also breaks the
// header commit signature. The anchor excuses ONLY the receipt failure, so
// the collateral header-signature defect must surface as a
// known-bad-extra-failure anomaly and gate the audit to 24 — the known-bad
// entry is not a blanket amnesty for every failure at the height.
//
// Note: tampering IncomingReceiptHash necessarily also invalidates the commit
// signature over the header, and a body-level incoming-receipt tamper is
// applied by re-execution and diverges the state root into a FATAL insert.
// A clean receipt-ONLY validity failure is therefore not producible on a
// single-shard localnet fixture; the gate-satisfied (exit 0) path is proven
// deterministically by the unit test audit.TestCrossCheckKnownBad
// ("receipt_only_at_known_bad_satisfies_gate").
func TestAuditKnownBadExtraFailureAnomalous(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	tamperIncomingReceiptHash(t, dir, fxBlocks)
	anchorPath := writeAnchorKnownBad(t, dir, fxTarget, []uint64{fxBlocks})
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		rep := readAuditReport(t, outDir)
		for _, a := range rep.Reconciliation.Anomalies {
			t.Logf("ANOMALY kind=%s key=%s detail=%s", a.Kind, a.Key, a.Detail)
		}
		t.Fatalf("known-bad extra-failure audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	// The exploit signature (incoming-receipts) was reproduced ...
	if !rep.KnownBadCrossChecked {
		t.Fatal("known_bad_cross_checked must be true when the known-bad block reproduces the incoming-receipts failure")
	}
	var sawReceipts, sawSig bool
	for _, o := range rep.Pass2.FailedOutcomes {
		// The tampered head header is ALSO block N-1's commit-sig source:
		// its in-place rewrite fails child-record identity validation, so a
		// collateral source-identity failure at N-1 is expected alongside
		// the failures at N.
		if o.Height != fxBlocks && o.Height != fxBlocks-1 {
			t.Fatalf("unexpected validity failure at %d", o.Height)
		}
		if o.Height != fxBlocks {
			continue
		}
		for _, f := range o.ValidityFails {
			if strings.HasPrefix(f, "incoming-receipts:") {
				sawReceipts = true
			}
			if strings.HasPrefix(f, "header-signature:") {
				sawSig = true
			}
		}
	}
	if !sawReceipts || !sawSig {
		t.Fatalf("expected both incoming-receipts and header-signature failures at %d: %+v", fxBlocks, rep.Pass2.FailedOutcomes)
	}
	// ... but the collateral header-signature defect is NOT excused: a
	// known-bad-extra-failure anomaly at the height gates the audit.
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "known-bad-extra-failure" && a.Key == "48" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a known-bad-extra-failure anomaly at 48, got %+v", rep.Reconciliation.Anomalies)
	}
}

// TestAuditKnownBadWrongFailure is the NEGATIVE known-bad-gate case (F1): a
// seal/signature failure at the anchored known-bad height is NOT the
// exploit signature. The gate must reject it (known_bad_cross_checked
// false), raise a known-bad-wrong-failure anomaly, and exit 24 — a
// non-incoming-receipts defect must never satisfy the exploit detector.
func TestAuditKnownBadWrongFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	tamperBlockSig(t, dir, fxBlocks)
	anchorPath := writeAnchorKnownBad(t, dir, fxTarget, []uint64{fxBlocks})
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("seal failure at the known-bad height must exit 24 (wrong defect), got %d", code)
	}
	rep := readAuditReport(t, outDir)
	if rep.KnownBadCrossChecked {
		t.Fatal("a seal/signature failure must NOT satisfy the incoming-receipts exploit gate")
	}
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "known-bad-wrong-failure" && a.Key == "48" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a known-bad-wrong-failure anomaly at 48, got %+v", rep.Reconciliation.Anomalies)
	}
}

// plantSuppressedDVLAppend plants a dvl reverse-index key whose single entry
// claims a BRANCH delegation (BlockNum fxPostDeleg, a real branch height) by
// a delegator that appears in NO branch block. Normalization classifies the
// key post-target-only and schedules its deletion — i.e. the plan expects
// the branch re-execution to REWRITE it (the way it rewrites the genuine
// fxPostDeleg dvl append). Re-execution never writes this delegator's key,
// so its reproduction is SUPPRESSED. Returns the planted key.
func plantSuppressedDVLAppend(t *testing.T, dir string) []byte {
	t.Helper()
	delegator := common.HexToAddress("0x00000000000000000000000000000000dEaDD0d0")
	validator := common.HexToAddress("0x00000000000000000000000000000000dEaDbEEF")
	value, err := rlp.EncodeToBytes(staking.DelegationIndexes{{
		ValidatorAddress: validator,
		Index:            1,
		BlockNum:         big.NewInt(fxPostDeleg),
	}})
	if err != nil {
		t.Fatal(err)
	}
	key := append([]byte("dvl"), delegator.Bytes()...)
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Put(key, value); err != nil {
		t.Fatal(err)
	}
	return key
}

// TestAuditSuppressedBranchRewriteAnomaly is the review-round pin on plan
// reconciliation (finding 2): unlike the planted FUTURE key below (ss<5>,
// a key nothing could ever write), this plants a key in a namespace the
// branch genuinely rewrites (a dvl append attributed to the fxPostDeleg
// branch height) whose reproduction is then suppressed because no branch
// block carries that delegator's op. The bidirectional reconciliation must
// flag the missing rewrite (plan-key-not-reproduced naming the exact key)
// and gate the audit at 24 — while the REAL branch dvl rewrites still
// reproduce.
func TestAuditSuppressedBranchRewriteAnomaly(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	key := plantSuppressedDVLAppend(t, dir)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("suppressed-rewrite audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "plan-key-not-reproduced" && a.Key == fmt.Sprintf("%x", key) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected plan-key-not-reproduced naming the planted dvl key %x, got %+v",
			key, rep.Reconciliation.Anomalies)
	}
	// The genuine branch metadata rewrites still reproduce: the suppression
	// finding is specific to the planted key, not a blanket failure.
	if rep.Reconciliation.Reproduced < 1 || rep.Reconciliation.ByteEqual < 1 {
		t.Fatalf("real branch rewrites did not reproduce: %+v", rep.Reconciliation)
	}
}

// TestAuditPlantedKeyAnomaly is WS6 injected-anomaly (a): a planted
// post-target metadata key (ss<5>, an epoch the branch never reaches) is a
// planned deletion the re-execution can never reproduce — exit 24 naming
// the key.
func TestAuditPlantedKeyAnomaly(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	plantFutureSS(t, dir)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	code := runAuditForSeal(t, dir, anchorPath, outDir, filepath.Join(t.TempDir(), "scratch"))
	if code != 24 {
		t.Fatalf("planted-key audit exit %d, want 24", code)
	}
	rep := readAuditReport(t, outDir)
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "plan-key-not-reproduced" && a.Key == "737305" { // "ss\x05"
			found = true
		}
	}
	if !found {
		t.Fatalf("expected plan-key-not-reproduced naming ss<5> (737305), got %+v", rep.Reconciliation.Anomalies)
	}
}
