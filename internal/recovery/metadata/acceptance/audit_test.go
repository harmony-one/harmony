package acceptance

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	"github.com/harmony-one/harmony/internal/recovery/metadata/hmr"
)

func TestAuditTwoPassCloses(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	outDir := filepath.Join(t.TempDir(), "out")
	scratch := filepath.Join(t.TempDir(), "scratch")
	code := audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir, Scratch: scratch,
		SkipReserveCheckForTest: true, // CI filesystems are far below 200 GiB
	}, os.Stderr)

	raw, err := os.ReadFile(filepath.Join(outDir, "abandoned-branch-audit.json"))
	if err != nil {
		t.Fatalf("read audit report: %v", err)
	}
	var rep audit.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatalf("parse audit report: %v", err)
	}
	if code != 0 {
		for _, a := range rep.Reconciliation.Anomalies {
			t.Logf("ANOMALY kind=%s key=%s detail=%s", a.Kind, a.Key, a.Detail)
		}
		t.Logf("writes=%+v", rep.Reconciliation.Writes)
		t.Fatalf("audit exit %d verdict %s; reconciliation anomalies=%d",
			code, rep.Verdict, rep.Reconciliation.AnomalyCount)
	}
	// Both passes ran and executed the whole range.
	if rep.Pass1 == nil || rep.Pass2 == nil {
		t.Fatal("expected both passes in the report")
	}
	if !rep.Pass2.Authoritative {
		t.Fatal("pass 2 must be authoritative")
	}
	wantBlocks := int(rep.RangeEnd - rep.RangeStart + 1)
	if rep.Pass2.RootsMatched != wantBlocks {
		t.Fatalf("pass 2 matched %d roots, want %d (every branch root must match its header)",
			rep.Pass2.RootsMatched, wantBlocks)
	}
	// The post-target created validator is inventoried and reconciled
	// bidirectionally with its normalization removal.
	if len(rep.Staking.CreatedValidators) == 0 {
		t.Fatal("expected at least one post-target CreateValidator in the inventory")
	}
	if len(rep.Staking.RemovedValidators) == 0 {
		t.Fatal("expected at least one removed validator to reconcile against")
	}
	// The epoch transition (36/37) was re-executed: the reproduced
	// next-epoch shard state must byte-equal the source record.
	if !rep.EpochTransition.Observed {
		t.Fatal("expected the epoch transition to be observed in the audit range")
	}
	if !rep.EpochTransition.ShardStateEqual {
		t.Fatal("reproduced next-epoch ss must byte-equal the source's to-be-deleted record")
	}
	// Reconciliation closed with zero anomalies.
	if rep.Reconciliation.AnomalyCount != 0 {
		t.Fatalf("reconciliation anomalies: %+v", rep.Reconciliation.Anomalies)
	}
	// No validity failures on the clean branch, and the (empty) known-bad
	// gate stayed quiet.
	if rep.FirstValidityFailure != 0 || rep.Pass2.ValidityFailures != 0 {
		t.Fatalf("clean branch must have zero validity failures (first=%d count=%d)",
			rep.FirstValidityFailure, rep.Pass2.ValidityFailures)
	}
	// Delegation effects match exactly once (tuple binding): the block-42
	// delegate created the dvl index (metadata-producing); the block-43
	// top-up to the SAME pair produced no index and must NOT be marked
	// producing.
	var producing, topUps int
	for _, d := range rep.Staking.Delegations {
		if d.Block == fxPostDeleg && d.MetadataProducing {
			producing++
		}
		if d.Block == fxPostTopUp {
			topUps++
			if d.MetadataProducing {
				t.Fatalf("top-up delegation at %d falsely marked metadata-producing: %+v", fxPostTopUp, d)
			}
			if !d.Attempted {
				t.Fatalf("top-up delegation at %d must be inventoried as attempted", fxPostTopUp)
			}
		}
	}
	if producing != 1 {
		t.Fatalf("expected exactly one metadata-producing delegation at block %d, got %d", fxPostDeleg, producing)
	}
	if topUps != 1 {
		t.Fatalf("expected the top-up delegation at block %d in the inventory, got %d", fxPostTopUp, topUps)
	}
	// The 0xfc precompile matrix (WS6): four traced Delegate frames, one
	// per classification.
	if got := rep.Staking.PrecompileByKind["Delegate"]; got != 4 {
		t.Fatalf("expected 4 traced 0xfc Delegate frames, got %d (by kind: %+v)", got, rep.Staking.PrecompileByKind)
	}
	prec := map[uint64]*audit.DelegateClass{}
	for i := range rep.Staking.Delegations {
		d := &rep.Staking.Delegations[i]
		if d.Source == "precompile" {
			if prec[d.Block] != nil {
				t.Fatalf("duplicate precompile delegation inventory at block %d", d.Block)
			}
			prec[d.Block] = d
		}
	}
	check := func(block uint64, wantVisible, wantReverted, wantProducing bool, label string) {
		t.Helper()
		d := prec[block]
		if d == nil {
			t.Fatalf("%s 0xfc delegation at block %d missing from the inventory", label, block)
		}
		if d.FrameFailed {
			t.Fatalf("%s 0xfc frame at %d failed: %+v", label, block, d)
		}
		if d.StakeMsgsVisible != wantVisible || d.EnclosingReverted != wantReverted || d.MetadataProducing != wantProducing {
			t.Fatalf("%s 0xfc delegation at %d misclassified: %+v (want visible=%v reverted=%v producing=%v)",
				label, block, d, wantVisible, wantReverted, wantProducing)
		}
	}
	check(fxPrecDirect, true, false, true, "direct")
	check(fxPrecNested, true, false, true, "nested")
	check(fxPrecRevert, true, true, false, "reverted")
	check(fxPrecTopUp, true, false, false, "top-up")
	// Throughput smoke (WS6 regression guard): the two-pass audit must
	// sustain ≥ 20 executed blocks/s on the localnet fixture.
	executed := rep.Pass1.ExecutedBlocks + rep.Pass2.ExecutedBlocks
	if rep.DurationS > 0 && float64(executed)/rep.DurationS < 20 {
		t.Fatalf("audit throughput %.1f blocks/s < 20 (executed %d in %.2fs)",
			float64(executed)/rep.DurationS, executed, rep.DurationS)
	}
	// The per-block outcome evidence is digest-bound for both passes.
	if rep.Pass1.OutcomesSHA256 == "" || rep.Pass2.OutcomesSHA256 == "" ||
		rep.Pass2.OutcomeCount != wantBlocks {
		t.Fatalf("per-block outcome evidence missing: p1=%q p2=%q count=%d",
			rep.Pass1.OutcomesSHA256, rep.Pass2.OutcomesSHA256, rep.Pass2.OutcomeCount)
	}
	// Source immutability across the full audit: real fingerprint proof
	// (before/after captured by the audit itself, device/inode included),
	// cross-checked against an independent post-run fingerprint.
	if rep.FingerprintBefore == nil || rep.FingerprintAfter == nil {
		t.Fatal("audit report must carry both source fingerprints")
	}
	if !rep.SourceUnchanged {
		t.Fatal("source fingerprint changed during the audit")
	}
	post, err := dbopen.FingerprintDir(dir)
	if err != nil {
		t.Fatalf("independent post-run fingerprint: %v", err)
	}
	if !rep.FingerprintBefore.Equal(post) {
		t.Fatal("independent post-run fingerprint differs from the audit's before-fingerprint")
	}
}

// TestAuditKnownBadGate pins the gating known-bad cross-check (§4.6 output
// 1): when the anchor asserts a known-bad height but the branch produces NO
// validity failure there, the audit must exit 24 with the
// known-bad-failure-absent anomaly — the expected exploit failure was not
// reproduced, so the audit did not observe what the anchor asserts.
func TestAuditKnownBadGate(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchorKnownBad(t, dir, fxTarget, []uint64{fxTarget + 1})
	outDir := filepath.Join(t.TempDir(), "out")
	scratch := filepath.Join(t.TempDir(), "scratch")
	code := runAuditForSeal(t, dir, anchorPath, outDir, scratch)
	if code != 24 {
		t.Fatalf("audit exit %d, want 24 (expected known-bad failure absent)", code)
	}
	raw, err := os.ReadFile(filepath.Join(outDir, "abandoned-branch-audit.json"))
	if err != nil {
		t.Fatalf("read audit report: %v", err)
	}
	var rep audit.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.KnownBadCrossChecked {
		t.Fatal("known_bad_cross_checked must be false when no failure occurred")
	}
	found := false
	for _, a := range rep.Reconciliation.Anomalies {
		if a.Kind == "known-bad-failure-absent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a known-bad-failure-absent anomaly, got %+v", rep.Reconciliation.Anomalies)
	}
}

// TestAuditReferenceCrossCheck runs export-reference, then the audit with
// --reference: the manifest must cross-check against the anchor (config
// SHA + anchor tuple) and be sealed into the report hash chain. A
// reference exported under a DIFFERENT anchor config must be refused with
// exit 15.
func TestAuditReferenceCrossCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	exportOut := filepath.Join(t.TempDir(), "export")
	if code := runExportForAudit(t, dir, anchorPath, exportOut); code != 0 {
		t.Fatalf("export exit %d", code)
	}
	refPath := filepath.Join(exportOut, "release", "metadata-30.reference.json")

	outDir := filepath.Join(t.TempDir(), "out")
	scratch := filepath.Join(t.TempDir(), "scratch")
	code := audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: outDir, Scratch: scratch,
		ReferencePath:           refPath,
		SkipReserveCheckForTest: true,
	}, os.Stderr)
	if code != 0 {
		t.Fatalf("audit with matching reference exit %d", code)
	}
	var rep audit.Report
	if err := json.Unmarshal(readFile(t, filepath.Join(outDir, "abandoned-branch-audit.json")), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Reference == nil || !rep.Reference.AnchorConfigOK || !rep.Reference.AnchorTupleOK {
		t.Fatalf("reference cross-check not recorded: %+v", rep.Reference)
	}
	if rep.Reference.HashChain == "" || rep.Reference.ManifestSHA == "" {
		t.Fatalf("reference hash chain missing: %+v", rep.Reference)
	}

	if !rep.Reference.ContentMatch || rep.Reference.ExpectedManifestSHA != rep.Reference.ManifestSHA {
		t.Fatalf("reference content cross-check not recorded: %+v", rep.Reference)
	}

	// A reference bound to a different anchor config is refused (15).
	otherAnchor := writeAnchorKnownBad(t, dir, fxTarget, []uint64{fxTarget + 1})
	code = audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: otherAnchor, OutDir: filepath.Join(t.TempDir(), "out2"),
		Scratch:                 filepath.Join(t.TempDir(), "scratch2"),
		ReferencePath:           refPath,
		SkipReserveCheckForTest: true,
	}, os.Stderr)
	if code != 15 {
		t.Fatalf("mismatched reference must exit 15, got %d", code)
	}

	// A reference whose ANCHOR still matches but whose normalized CONTENT
	// has been forged (a flipped section digest) must be refused (15): the
	// audit rebuilds the expected manifest from its own normalization and
	// byte-compares, so anchor-only agreement is not enough.
	forged := forgeManifestSection(t, readFile(t, refPath))
	forgedPath := filepath.Join(t.TempDir(), "forged.reference.json")
	if err := os.WriteFile(forgedPath, forged, 0o644); err != nil {
		t.Fatal(err)
	}
	code = audit.Run(context.Background(), audit.Options{
		DBPath: dir, AnchorPath: anchorPath, OutDir: filepath.Join(t.TempDir(), "out3"),
		Scratch:                 filepath.Join(t.TempDir(), "scratch3"),
		ReferencePath:           forgedPath,
		SkipReserveCheckForTest: true,
	}, os.Stderr)
	if code != 15 {
		t.Fatalf("content-forged reference must exit 15, got %d", code)
	}
}

// forgeManifestSection decodes a reference manifest, flips one hex character
// of the first section digest and re-encodes it canonically: the anchor
// tuple and config SHA are untouched, but the normalized content no longer
// matches what the audit derives.
func forgeManifestSection(t *testing.T, raw []byte) []byte {
	t.Helper()
	m, err := hmr.DecodeManifest(raw)
	if err != nil {
		t.Fatalf("decode reference for forging: %v", err)
	}
	d := []byte(m.Sections[0].SHA256)
	if d[0] == '0' {
		d[0] = '1'
	} else {
		d[0] = '0'
	}
	m.Sections[0].SHA256 = string(d)
	enc, err := hmr.EncodeManifest(m)
	if err != nil {
		t.Fatalf("re-encode forged reference: %v", err)
	}
	return enc
}

// TestAuditRejectsNegativeReserve pins that the mandatory scratch reserve
// gate cannot be bypassed from the CLI surface (negative values are an
// invocation error; only the test-only option field skips the gate).
func TestAuditRejectsNegativeReserve(t *testing.T) {
	code := audit.Run(context.Background(), audit.Options{
		DBPath: "/nonexistent/harmony_db_0", AnchorPath: "/nonexistent/anchor.json",
		OutDir: t.TempDir(), Scratch: t.TempDir(),
		ScratchReserveGB: -1,
	}, os.Stderr)
	if code != 15 {
		t.Fatalf("negative --scratch-reserve-gb must exit 15, got %d", code)
	}
}
