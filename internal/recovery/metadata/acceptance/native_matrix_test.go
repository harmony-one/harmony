package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

// TestAuditNativeDirectiveMatrix extends the WS6 staking coverage beyond
// Delegate/CreateValidator: it drives a branch that also carries a native
// EditValidator and a native Undelegate, plus a 0xfc precompile Delegate
// and a 0xfc precompile Undelegate, and asserts the audit inventories every
// directive in both the native and precompile censuses while the two-pass
// reconciliation still closes cleanly (exit 0).
//
// CollectRewards needs a longer, reward-funded branch and is covered
// end-to-end by TestAuditCollectRewards (collect_rewards_test.go).
func TestAuditNativeDirectiveMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	const (
		blocks    = 46
		target    = 30
		create    = 22
		delegate  = 26
		fund      = 28
		precDel   = 40 // EOA→0xfc Delegate to the pre-target validator
		edit      = 42 // native EditValidator (deployer edits its validator)
		undeleg   = 44 // native Undelegate (test account, from block-26 stake)
		precUndel = 46 // EOA→0xfc Undelegate of its own delegation
	)
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:                 blocks,
		CreateValidatorAt:      create,
		DelegateAt:             delegate,
		FundPrecompileAt:       fund,
		PrecompileDirectAt:     precDel,
		EditValidatorAt:        edit,
		UndelegateAt:           undeleg,
		PrecompileUndelegateAt: precUndel,
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
		t.Fatalf("directive-matrix audit exit %d verdict %s", code, rep.Verdict)
	}
	// Native directive census: EditValidator and Undelegate present.
	for _, d := range []string{"EditValidator", "Undelegate"} {
		if rep.Staking.NativeByDirective[d] < 1 {
			t.Fatalf("native directive %q not inventoried: %+v", d, rep.Staking.NativeByDirective)
		}
	}
	// Precompile census: a Delegate and an Undelegate frame, both parseable.
	if rep.Staking.PrecompileByKind["Delegate"] < 1 || rep.Staking.PrecompileByKind["Undelegate"] != 1 {
		t.Fatalf("precompile kinds unexpected: %+v", rep.Staking.PrecompileByKind)
	}
}
