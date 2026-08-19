package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/metadata/scan"
)

// target and staking schedule for the acceptance kit. localnet 16/16:
// epoch1=[5,20], epoch2=[21,36], epoch3=[37,52]. Target 30 (epoch 2) puts
// the 36/37 election and the post-target staking in the audit range.
const (
	fxBlocks     = 48
	fxTarget     = 30
	fxCreateVal  = 22
	fxDelegate   = 26
	fxFundPrec   = 28 // fund the 0xfc EOA + deploy proxy/reverter (pre-target)
	fxPostCreate = 40
	fxPostDeleg  = 42
	fxPostTopUp  = 43 // repeats the 42 delegation: NO new dvl index
	// The 0xfc precompile delegation matrix (WS6): direct, nested
	// (contract→0xfc), reverted (contract→0xfc then REVERT), top-up.
	fxPrecDirect = 45
	fxPrecNested = 46
	fxPrecRevert = 47
	fxPrecTopUp  = 48
)

// buildFixture generates a fresh chain and returns its closed DB dir plus
// the target hash/root read from it.
func buildFixture(t *testing.T) (dir string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	spec := metafixture.Spec{
		Blocks:                fxBlocks,
		CreateValidatorAt:     fxCreateVal,
		DelegateAt:            fxDelegate,
		FundPrecompileAt:      fxFundPrec,
		PostCreateValidatorAt: fxPostCreate,
		PostDelegateAt:        fxPostDeleg,
		PostTopUpAt:           fxPostTopUp,
		PrecompileDirectAt:    fxPrecDirect,
		PrecompileNestedAt:    fxPrecNested,
		PrecompileRevertAt:    fxPrecRevert,
		PrecompileTopUpAt:     fxPrecTopUp,
	}
	if err := c.Generate(spec); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	return dir
}

// writeAnchor builds a localnet anchor config for fxTarget from the chain.
// The clean fixture plants no exploit block, so known_bad_blocks is empty
// (the audit's known-bad gate expects zero validity failures).
func writeAnchor(t *testing.T, dir string, target uint64) string {
	return writeAnchorKnownBad(t, dir, target, nil)
}

// writeAnchorKnownBad is writeAnchor with an explicit known-bad list (the
// known-bad gate tests use it to assert the expected-failure-absent path).
func writeAnchorKnownBad(t *testing.T, dir string, target uint64, knownBad []uint64) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := metafixture.WriteAnchorConfig(dir, target, fxBlocks, knownBad, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func runScan(t *testing.T, dir, anchorPath string) (int, *scan.Report) {
	t.Helper()
	reportPath := filepath.Join(t.TempDir(), "scan-report.json")
	code := scan.Run(context.Background(), scan.Options{
		DBPath: dir, AnchorPath: anchorPath, ReportPath: reportPath,
	}, os.Stderr)
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read scan report: %v", err)
	}
	var rep scan.Report
	if err := json.Unmarshal(raw, &rep); err != nil {
		t.Fatalf("parse scan report: %v", err)
	}
	return code, &rep
}

// TestScanCleanPasses is the B5 read-only + clean-target row: exit 0,
// zero-write proof, the post-target validator removed, reconstruction
// resolved on the archival fixture.
func TestScanCleanPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := buildFixture(t)
	anchorPath := writeAnchor(t, dir, fxTarget)
	code, rep := runScan(t, dir, anchorPath)
	if code != 0 || rep.Verdict != "OK" {
		t.Fatalf("scan exit %d verdict %s, findings %+v", code, rep.Verdict, rep.Findings.Items)
	}
	if rep.NormalizedValidatorListLength != 1 {
		t.Fatalf("normalized list length %d, want 1", rep.NormalizedValidatorListLength)
	}
	if !rep.ZeroWriteProof || rep.WriteAttempts != 0 {
		t.Fatalf("zero-write proof failed: proof=%v attempts=%d", rep.ZeroWriteProof, rep.WriteAttempts)
	}
	if rep.Counts.ValidatorList.Removed != 1 {
		t.Fatalf("expected exactly one removed (post-target) validator, got %d", rep.Counts.ValidatorList.Removed)
	}
	if rep.Counts.RewardAccum.Removed == 0 {
		t.Fatal("expected post-target reward-accumulator records removed")
	}
	_ = fmt.Sprint
}
