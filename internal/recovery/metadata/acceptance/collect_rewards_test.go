package acceptance

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
)

// TestAuditCollectRewards closes the last WS6 directive gap through the REAL
// audit loop: a branch that carries a native CollectRewards AND a 0xfc
// precompile CollectRewards, both actually funded. The pre-target validator
// (created at 22) wins a shard-0 slot in the epoch-3 election, signs blocks
// 37+ (the fixture signs with the full elected committee, its key included),
// and localnet's aggregated reward payout at block 47 (RewardFrequency 16)
// credits both pre-snapshot delegations — the test account's native
// delegation (26) and the precompile EOA's 0xfc delegation (25). The branch
// then collects natively at 49 and through 0xfc at 50.
//
// The audit must re-execute the reward distribution and both collections to
// the stored roots, inventory the native directive and the precompile frame,
// and close reconciliation cleanly (exit 0): reward collection mutates only
// state material (wrapper rewards, balances), never planned metadata.
//
// This test runs in an ISOLATED subprocess (see isolation_test.go): its
// fixture is the only one in the package with an extra pre-snapshot
// delegation (the 0xfc EOA at 25), so its epoch-3 delegation share map
// differs from every other fixture's, and the process-wide reward caches in
// internal/chain would otherwise cross-poison the payout between tests.
func TestAuditCollectRewards(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	if !runIsolatedSubtest(t) {
		return
	}
	const (
		blocks      = 52
		target      = 30
		create      = 22 // validator elected into shard 0 at epoch 3
		fund        = 23 // fund the 0xfc EOA + forwarders (pre-target)
		precDel     = 25 // EOA→0xfc Delegate (pre-snapshot, earns a share)
		delegate    = 26 // native delegate (pre-snapshot, earns a share)
		collect     = 49 // native CollectRewards (after the block-47 payout)
		precCollect = 50 // EOA→0xfc CollectRewards
	)
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(dir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:                     blocks,
		CreateValidatorAt:          create,
		FundPrecompileAt:           fund,
		PrecompileDirectAt:         precDel,
		DelegateAt:                 delegate,
		CollectRewardsAt:           collect,
		PrecompileCollectRewardsAt: precCollect,
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
		t.Fatalf("collect-rewards audit exit %d verdict %s, want 0", code, rep.Verdict)
	}

	// Native census: the branch CollectRewards body entry is inventoried.
	if rep.Staking.NativeByDirective["CollectRewards"] < 1 {
		t.Fatalf("native CollectRewards not inventoried: %+v", rep.Staking.NativeByDirective)
	}
	// Precompile census: the traced 0xfc frame parsed as CollectRewards.
	if rep.Staking.PrecompileByKind["CollectRewards"] < 1 {
		t.Fatalf("precompile CollectRewards not inventoried: %+v", rep.Staking.PrecompileByKind)
	}
	// Both collections happened on the branch and the branch re-executed to
	// its stored roots (a divergent reward payout would be a FATAL insert,
	// and any masked-metadata effect would surface as an anomaly above).
	for _, o := range rep.Pass2.FailedOutcomes {
		if o.Height == collect || o.Height == precCollect {
			t.Fatalf("collection block %d failed validity checks: %v", o.Height, o.ValidityFails)
		}
	}
}
