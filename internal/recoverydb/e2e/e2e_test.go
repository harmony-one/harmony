// Package e2e runs the full producer pipeline — inspect (both copies +
// agreement) → export (single donor, preflight) → replay → compact (internal
// mode) → verify → package — against an in-process localnet fixture chain
// with real BLS certificates (plan WS8).
package e2e

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/bundle"
	"github.com/harmony-one/harmony/internal/recoverydb/compact"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/fixture"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/inspect"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/release"
	"github.com/harmony-one/harmony/internal/recoverydb/replay"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

const (
	baselineHeight = 18
	targetHeight   = 22
	donorHeight    = 26
	toolVersion    = "harmony-recovery-db/test"
)

// kit is the generated fixture world shared by the E2E stages.
type kit struct {
	root             string
	donorDir         string
	baseA            string // replay working copy
	baseB            string // second Aug-8-style copy
	anchorNoBaseline string // anchor before baseline pinning (not used by replay)
	anchorPath       string

	targetHash  common.Hash
	parentHash  common.Hash
	childHash   common.Hash
	targetEpoch uint64
}

func buildKit(t *testing.T) *kit {
	t.Helper()
	root := t.TempDir()
	k := &kit{root: root}
	k.donorDir = filepath.Join(root, "donor", "harmony_db_0")

	// Generate to the baseline, snapshot two copies, then extend to the
	// donor head (shared history through the target; the donor keeps
	// going past it, mirroring the real donor shape).
	c, err := fixture.Open(k.donorDir, fixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("fixture open: %v", err)
	}
	if err := c.Generate(fixture.Params{Blocks: baselineHeight, TxEvery: 5, DeployContractAt: 6, CreateValidatorAt: 9, DelegateAt: 11}); err != nil {
		t.Fatalf("fixture generate to baseline: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("fixture finalize: %v", err)
	}
	k.baseA = filepath.Join(root, "baseline-a", "harmony_db_0")
	k.baseB = filepath.Join(root, "baseline-b", "harmony_db_0")
	if err := fixture.CopyDir(k.donorDir, k.baseA); err != nil {
		t.Fatalf("copy baseline A: %v", err)
	}
	if err := fixture.CopyDir(k.donorDir, k.baseB); err != nil {
		t.Fatalf("copy baseline B: %v", err)
	}
	c, err = fixture.Open(k.donorDir, fixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("fixture reopen: %v", err)
	}
	if err := c.Generate(fixture.Params{Blocks: donorHeight - baselineHeight, TxEvery: 5}); err != nil {
		t.Fatalf("fixture generate to donor head: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("fixture finalize donor: %v", err)
	}

	// Read the pinned tuple from the donor.
	db, ro, err := dbopen.OpenSourceDatabase(k.donorDir)
	if err != nil {
		t.Fatalf("open donor: %v", err)
	}
	k.targetHash = rawdb.ReadCanonicalHash(db, targetHeight)
	tHdr := rawdb.ReadHeader(db, k.targetHash, targetHeight)
	if tHdr == nil {
		t.Fatalf("donor target header missing")
	}
	k.parentHash = tHdr.ParentHash()
	k.targetEpoch = tHdr.Epoch().Uint64()
	k.childHash = rawdb.ReadCanonicalHash(db, targetHeight+1)
	ro.Close()

	// Anchor manifest for the fixture incident.
	m := &anchor.Manifest{
		SchemaVersion:        anchor.SchemaVersionV1,
		Network:              "localnet",
		ShardID:              0,
		TargetHeight:         targetHeight,
		TargetHash:           k.targetHash,
		TargetParentHash:     k.parentHash,
		TargetEpoch:          k.targetEpoch,
		BaselineHeight:       baselineHeight,
		AbandonedChildHeight: targetHeight + 1,
		AbandonedChildHash:   k.childHash,
	}
	k.anchorPath = filepath.Join(root, "anchor.json")
	writeJSONWithSum(t, k.anchorPath, m)
	return k
}

func writeJSONWithSum(t *testing.T, path string, v interface{}) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if _, err := integrity.WriteChecksumFile(path); err != nil {
		t.Fatalf("checksum %s: %v", path, err)
	}
}

func TestPipelineEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("pipeline E2E is not short")
	}
	k := buildKit(t)

	// ---- inspect both copies + two-copy agreement. ----
	inspectA := filepath.Join(k.root, "inspect-a.json")
	repA, sumA, err := inspect.Run(inspect.Params{
		Network: "localnet", ShardID: 0, DBPath: k.baseA,
		FullState: true, FullOffchain: true, RequirePreimages: true,
		TargetHeight: targetHeight, AnchorPath: k.anchorPath,
		Output: inspectA, ToolVersion: toolVersion,
	})
	if err != nil {
		t.Fatalf("inspect A: %v", err)
	}
	if inspect.Failed(repA) {
		t.Fatalf("inspect A failed checks: %+v", repA.Checks)
	}
	if repA.Heads[0].Height != baselineHeight {
		t.Fatalf("baseline head %d, want %d", repA.Heads[0].Height, baselineHeight)
	}
	_ = sumA
	inspectB := filepath.Join(k.root, "inspect-b.json")
	repB, _, err := inspect.Run(inspect.Params{
		Network: "localnet", ShardID: 0, DBPath: k.baseB,
		FullState: true, FullOffchain: true, RequirePreimages: true,
		TargetHeight: targetHeight, AnchorPath: k.anchorPath,
		Output: inspectB, ToolVersion: toolVersion,
	})
	if err != nil {
		t.Fatalf("inspect B: %v", err)
	}
	if inspect.Failed(repB) {
		t.Fatalf("inspect B failed checks: %+v", repB.Checks)
	}
	agreementPath := filepath.Join(k.root, "agreement.json")
	verdict, err := inspect.Agreement("localnet", 0, toolVersion, inspectA, inspectB, agreementPath)
	if err != nil {
		t.Fatalf("agreement: %v", err)
	}
	if !verdict.Agreed {
		t.Fatalf("two-copy agreement failed: %v", verdict.Differences)
	}

	// ---- export from the single donor (with preflight inside). ----
	if _, err := harness.InitSchedule("localnet"); err != nil {
		t.Fatal(err)
	}
	chainCfg, err := harness.ChainConfig("localnet", 0)
	if err != nil {
		t.Fatal(err)
	}
	donorDB, donorRO, err := dbopen.OpenSourceDatabase(k.donorDir)
	if err != nil {
		t.Fatalf("open donor: %v", err)
	}
	anc, err := anchor.Load(k.anchorPath)
	if err != nil {
		t.Fatal(err)
	}
	bundleDir := filepath.Join(k.root, "bundle")
	manifest, err := bundle.Export(donorDB, bundle.ExportConfig{
		Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
		FromHeight: baselineHeight + 1, ToHeight: targetHeight, CertChildHeight: targetHeight + 1,
		BaselineHeight: baselineHeight, BaselineHash: common.HexToHash(repA.Heads[0].Hash),
		Anchor: anc, OutputDir: bundleDir, ChunkBytes: 4096, /* force multiple chunks */
		Donor: "fixture-donor", ToolVersion: toolVersion,
	})
	donorRO.Close()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if manifest.RecordCount != targetHeight-baselineHeight {
		t.Fatalf("bundle has %d records, want %d", manifest.RecordCount, targetHeight-baselineHeight)
	}
	if len(manifest.Chunks) < 2 {
		t.Fatalf("expected multiple chunks with tiny chunk-bytes, got %d", len(manifest.Chunks))
	}

	// ---- replay into baseline A. ----
	replayOut := filepath.Join(k.root, "replay.json")
	replayRep, err := replay.Run(replay.Config{
		Network: "localnet", ShardID: 0,
		DestinationDB:         k.baseA,
		AnchorPath:            k.anchorPath,
		InspectReportPath:     inspectA,
		BaselineAgreementPath: agreementPath,
		BundleDir:             bundleDir,
		TargetHeight:          targetHeight,
		ToolVersion:           toolVersion,
		OutputPath:            replayOut,
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if replayRep.BlocksReplayed != targetHeight-baselineHeight {
		t.Fatalf("replayed %d blocks, want %d", replayRep.BlocksReplayed, targetHeight-baselineHeight)
	}
	if !replayRep.Gate.Passed {
		t.Fatalf("replay gate failed: %+v", replayRep.Gate.Checks)
	}
	// Rerunning refuses (journal, v1 no-resume).
	if _, err := replay.Run(replay.Config{
		Network: "localnet", ShardID: 0, DestinationDB: k.baseA,
		AnchorPath: k.anchorPath, InspectReportPath: inspectA,
		BaselineAgreementPath: agreementPath, BundleDir: bundleDir,
		TargetHeight: targetHeight, ToolVersion: toolVersion, OutputPath: replayOut + ".rerun",
	}); err == nil || !strings.Contains(err.Error(), "journal") {
		t.Fatalf("rerun should refuse on existing journal, got %v", err)
	}

	// ---- compact (internal mode). ----
	sched, _ := harness.Schedule("localnet")
	window, err := anchor.ComputeWindow(sched, targetHeight, 0)
	if err != nil {
		t.Fatal(err)
	}
	compactDir := filepath.Join(k.root, "compact", "harmony_db_0")
	compactOut := filepath.Join(k.root, "compact.json")
	compactRep, err := compact.Run(compact.Config{
		Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
		SourceDB: k.baseA, DestinationDB: compactDir,
		AnchorPath: k.anchorPath, SourceReferencePath: replayOut,
		TargetHeight: targetHeight, ToolVersion: toolVersion, OutputPath: compactOut,
	}, window)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if compactRep.JournalState != report.StateCompleteVerified {
		t.Fatalf("compact journal state %s", compactRep.JournalState)
	}
	if compactRep.Mode != report.ModeInternal || compactRep.MetadataReferenceDigest != verify.MetadataReferenceInternalNone {
		t.Fatalf("compact mode/reference wrong: %s / %s", compactRep.Mode, compactRep.MetadataReferenceDigest)
	}

	// ---- verify the compact artifact. ----
	roDB, ro, err := dbopen.OpenSourceDatabase(compactDir)
	if err != nil {
		t.Fatalf("open compact: %v", err)
	}
	result, err := verify.Run(roDB, verify.Params{
		Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
		Anchor: anc, AnchorSHA256: mustFileSHA(t, k.anchorPath),
		Compact:           compactRep,
		Window:            window,
		TargetIsEpochLast: sched.EpochLastBlock(window.Epoch) == window.Target,
		TempDir:           k.root,
	})
	ro.Close()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !result.Passed {
		for _, c := range result.Checks {
			if !c.OK {
				t.Errorf("verify check failed: %s: %s", c.ID, c.Detail)
			}
		}
		t.Fatalf("verify-db failed")
	}
	if result.CertificatesVerified != window.Blocks() {
		t.Fatalf("verified %d certificates, want %d", result.CertificatesVerified, window.Blocks())
	}

	// The fixture carries REAL staking metadata (round 13 finding 9): the
	// verified compact artifact must have a populated validator list, the
	// delegation index, and the elected validator's snapshot — pinning
	// that the staking cross-checks above did not pass vacuously.
	{
		sdb, sro, err := dbopen.OpenSourceDatabase(compactDir)
		if err != nil {
			t.Fatal(err)
		}
		vals, err := rawdb.ReadValidatorList(sdb)
		if err != nil || len(vals) == 0 {
			t.Fatalf("compact artifact must carry a non-empty validator list: %v %v", vals, err)
		}
		testAddr := common.HexToAddress("0xA5241513DA9F4463F1d4874b548dFBAC29D91f34")
		delegations, err := rawdb.ReadDelegationsByDelegator(sdb, testAddr)
		if err != nil || len(delegations) == 0 {
			t.Fatalf("compact artifact must carry the fixture delegation index: %v %v", delegations, err)
		}
		snap, err := rawdb.ReadValidatorSnapshot(sdb, vals[0], new(big.Int).SetUint64(k.targetEpoch))
		if err != nil || snap == nil {
			t.Fatalf("compact artifact must carry the validator snapshot for epoch %d: %v", k.targetEpoch, err)
		}
		sro.Close()
	}

	// ---- package (single invocation). ----
	verifOut := filepath.Join(k.root, "verification.json")
	verifRep := &report.VerificationReport{
		DBPath: compactDir, Mode: compactRep.Mode,
		Checks: result.Checks, Passed: result.Passed,
		DigestSet:               result.DigestSet,
		LogicalKVDigest:         result.Logical.Total.SHA256,
		NormalizedOutputDigest:  result.NormalizedOutput,
		MetadataReferenceDigest: compactRep.MetadataReferenceDigest,
		CertificatesVerified:    result.CertificatesVerified,
		JournalState:            report.StateCompleteVerified,
	}
	meta, err := report.NewMeta(report.VerificationSchemaV1, "verify-db", "localnet", 0, toolVersion,
		[]integrity.InputRef{{Name: "anchor-manifest", Path: k.anchorPath, SHA256: mustFileSHA(t, k.anchorPath)}})
	if err != nil {
		t.Fatal(err)
	}
	verifRep.Meta = meta
	if _, err := report.WriteJSON(verifOut, verifRep); err != nil {
		t.Fatal(err)
	}

	releaseRoot := filepath.Join(k.root, "release")
	if err := os.MkdirAll(releaseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	pkgRep, finalDir, err := release.Run(release.Config{
		Network: "localnet", ShardID: 0,
		DBPath: compactDir, AnchorPath: k.anchorPath, TargetHeight: targetHeight,
		VerificationReportPath: verifOut,
		ReleaseRoot:            releaseRoot,
		ToolVersion:            toolVersion,
	})
	if err != nil {
		t.Fatalf("package: %v", err)
	}
	// Seal semantics: READY content == release ID; SHA256SUMS re-verifies;
	// release.json listed in SHA256SUMS (non-circularity).
	ready, err := os.ReadFile(filepath.Join(finalDir, "READY"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(ready)) != pkgRep.ReleaseID {
		t.Fatalf("READY %q != release ID %s", ready, pkgRep.ReleaseID)
	}
	sums, err := integrity.ReadSums(filepath.Join(finalDir, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	sawReleaseJSON, sawInstall := false, false
	for _, e := range sums {
		if e.Name == "release.json" {
			sawReleaseJSON = true
		}
		if e.Name == "INSTALL.md" {
			sawInstall = true
		}
		if e.Name == "SHA256SUMS" || e.Name == "READY" {
			t.Fatalf("SHA256SUMS must not list %s", e.Name)
		}
		if err := integrity.VerifyRecorded(filepath.Join(finalDir, e.Name), e.SHA256); err != nil {
			t.Fatalf("sealed entry %s: %v", e.Name, err)
		}
	}
	if !sawReleaseJSON || !sawInstall {
		t.Fatalf("SHA256SUMS must cover release.json and INSTALL.md")
	}
	var rj report.ReleaseJSON
	if err := report.ReadJSONStrict(filepath.Join(finalDir, "release.json"), &rj); err != nil {
		t.Fatal(err)
	}
	if rj.TargetHash != k.targetHash.Hex() || rj.RecoveryHarmonyBinarySHA256 != release.FieldAbsent {
		t.Fatalf("release.json fields wrong: %+v", rj)
	}

	// Byte-identical rebuild against a sealed release refuses.
	if _, _, err := release.Run(release.Config{
		Network: "localnet", ShardID: 0,
		DBPath: compactDir, AnchorPath: k.anchorPath, TargetHeight: targetHeight,
		VerificationReportPath: verifOut, ReleaseRoot: releaseRoot, ToolVersion: toolVersion,
	}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("sealed rebuild should refuse as already-existing, got: %v", err)
	}

	// ---- INSTALL.md-style install + offline reopen smoke: copy the
	// payload to a fresh location and confirm the head tuple via a strict
	// read-only open (the stock-binary boot smoke lives in
	// scripts/recovery/e2e-localnet.sh). ----
	installed := filepath.Join(k.root, "installed", "harmony_db_0")
	if err := fixture.CopyDir(filepath.Join(finalDir, "payload", "harmony_db_0"), installed); err != nil {
		t.Fatal(err)
	}
	instDB, instRO, err := dbopen.OpenSourceDatabase(installed)
	if err != nil {
		t.Fatalf("open installed payload: %v", err)
	}
	for _, hk := range [][]byte{[]byte("LastBlock"), []byte("LastHeader"), []byte("LastFast"), []byte("LastFinalized")} {
		val, err := instDB.Get(hk)
		if err != nil {
			t.Fatalf("installed head %s: %v", hk, err)
		}
		if common.BytesToHash(val) != k.targetHash {
			t.Fatalf("installed head %s = %x, want %s", hk, val, k.targetHash.Hex())
		}
	}
	marker, err := verify.ReadMarker(instDB)
	if err != nil {
		t.Fatalf("installed marker: %v", err)
	}
	if marker.LogicalKVDigest != compactRep.LogicalKVDigest {
		t.Fatalf("installed marker digest mismatch")
	}
	instRO.Close()

	// Offline boot smoke: open the installed payload through the STOCK chain
	// open path (core.NewBlockChainWithOptions via the harness) — the same
	// code a stock harmony binary runs at startup: loadLastState,
	// buildLeaderRotationMeta over the retained headers, snapshot config.
	// Assert it boots to exactly the target tuple with the marker inert.
	// (A throwaway writable copy, never the sealed artifact.)
	bootDir := filepath.Join(k.root, "boot", "harmony_db_0")
	if err := fixture.CopyDir(filepath.Join(finalDir, "payload", "harmony_db_0"), bootDir); err != nil {
		t.Fatal(err)
	}
	bootDB, err := dbopen.OpenDestination(bootDir, false)
	if err != nil {
		t.Fatalf("open boot copy: %v", err)
	}
	bc, err := harness.OpenChain(bootDB, "localnet", 0, harness.ModeReplay)
	if err != nil {
		bootDB.Close()
		t.Fatalf("stock offline boot failed: %v", err)
	}
	if h := bc.CurrentBlock(); h.Hash() != k.targetHash || h.NumberU64() != targetHeight {
		bootDB.Close()
		t.Fatalf("booted head %s@%d, want %s@%d", h.Hash().Hex(), h.NumberU64(), k.targetHash.Hex(), targetHeight)
	}
	bootDB.Close()

	t.Logf("E2E complete: release %s at %s (%d certs verified; stock offline boot reached target)", pkgRep.ReleaseID, finalDir, result.CertificatesVerified)
}

func mustFileSHA(t *testing.T, path string) string {
	t.Helper()
	sum, err := integrity.FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}
