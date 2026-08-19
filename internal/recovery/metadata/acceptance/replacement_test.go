package acceptance

import (
	"math/big"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/core"
	coregenesis "github.com/harmony-one/harmony/core/genesis"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
)

// replacement post-target schedule: the SAME validator address is
// re-created and re-delegated-to at heights that differ from the dirty
// branch (40/42), so any reactivated stale index is unambiguous.
const (
	rpPostCreate = 41
	rpPostDeleg  = 43
)

// TestReplacementBranchNoReactivation is B5 bullet (6) in its literal
// form: over the masked overlay of the dirty chain (deletion plan applied,
// post-target records tombstoned, heads rewound), InsertChain a FRESH
// valid successor branch that re-creates the post-target validator address
// and re-delegates to it. Every resulting dvl index must carry the fresh
// branch's BlockNums — the old branch's indexes (created at 40/42 and
// removed by normalization) never reactivate.
func TestReplacementBranchNoReactivation(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dirty := buildFixture(t)

	// The replacement branch: identical pre-target ops (deterministic ⇒
	// byte-identical blocks 1..target), different post-target schedule.
	replDir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(replDir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks:                fxBlocks,
		CreateValidatorAt:     fxCreateVal,
		DelegateAt:            fxDelegate,
		FundPrecompileAt:      fxFundPrec,
		PostCreateValidatorAt: rpPostCreate,
		PostDelegateAt:        rpPostDeleg,
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatal(err)
	}

	// Masked overlay over the dirty chain (the audit's post-apply view).
	anchorPath := writeAnchor(t, dirty, fxTarget)
	res, err := anchor.Resolve(anchorPath)
	if err != nil {
		t.Fatal(err)
	}
	open, err := source.OpenSource(dirty, res, dbopen.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer open.Close()
	srcs, err := open.BuildSources()
	if err != nil {
		t.Fatal(err)
	}
	nres, err := norm.Normalize(open.NormA, srcs)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := audit.NewOverlay(filepath.Join(t.TempDir(), "scratch"), open.KV)
	if err != nil {
		t.Fatal(err)
	}
	defer overlay.Close()
	if _, err := audit.Seed(overlay, nres, kvCanonReader{open}, res.TargetHash, res.Config.TargetHeight, nil, nil); err != nil {
		t.Fatal(err)
	}

	cfg := nodeconfig.GetShardConfig(0).GetNetworkType().ChainConfig()
	cfg.EthCompatibleChainID = big.NewInt(cfg.EthCompatibleShard0ChainID.Int64())
	overlayDB := rawdb.NewDatabase(overlay)
	bc, err := core.NewBlockChainWithOptions(
		overlayDB, nil, nil,
		&core.CacheConfig{Disabled: true, SnapshotLimit: 0},
		&cfg, chain.NewEngine(), vm.Config{}, core.Options{},
	)
	if err != nil {
		t.Fatalf("open masked chain: %v", err)
	}
	if got := bc.CurrentBlock().NumberU64(); got != fxTarget {
		t.Fatalf("masked head %d, want the target %d", got, fxTarget)
	}

	// Feed the replacement branch through the production InsertChain with
	// full header verification (the twin construction guarantees block
	// target+1 parents the target).
	rdb, err := rawdb.NewLevelDBDatabase(replDir, 16, 64, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	if rawdb.ReadCanonicalHash(rdb, fxTarget) != bc.CurrentBlock().Hash() {
		t.Fatal("replacement chain does not share the target block (twin construction broken)")
	}
	for n := uint64(fxTarget + 1); n <= fxBlocks; n++ {
		hash := rawdb.ReadCanonicalHash(rdb, n)
		blk := rawdb.ReadBlock(rdb, hash, n)
		if blk == nil {
			t.Fatalf("replacement block %d unreadable", n)
		}
		sig, err := rawdb.ReadBlockCommitSig(rdb, n)
		if err != nil {
			t.Fatalf("replacement commit sig %d: %v", n, err)
		}
		blk.SetCurrentCommitSig(sig)
		if _, err := bc.InsertChain(types.Blocks{blk}, true); err != nil {
			t.Fatalf("insert replacement block %d over the masked view: %v", n, err)
		}
	}

	// The re-created validator and its delegations carry ONLY fresh
	// BlockNums; the old branch's 40/42 indexes never reactivate.
	postValidator := c.PostValidatorAddr
	deployer := crypto.PubkeyToAddress(coregenesis.ContractDeployerKey.PublicKey)

	selfIdx, err := rawdb.ReadDelegationsByDelegator(overlayDB, postValidator)
	if err != nil {
		t.Fatal(err)
	}
	var selfSeen bool
	for _, di := range selfIdx {
		if di.ValidatorAddress != postValidator {
			continue
		}
		selfSeen = true
		if di.BlockNum.Uint64() != rpPostCreate {
			t.Fatalf("self-delegation index BlockNum %d, want the replacement create height %d (stale reactivation?)",
				di.BlockNum.Uint64(), rpPostCreate)
		}
	}
	if !selfSeen {
		t.Fatal("re-created validator has no self-delegation index after the replacement branch")
	}

	depIdx, err := rawdb.ReadDelegationsByDelegator(overlayDB, deployer)
	if err != nil {
		t.Fatal(err)
	}
	var delegSeen bool
	for _, di := range depIdx {
		if di.ValidatorAddress != postValidator {
			continue
		}
		delegSeen = true
		if got := di.BlockNum.Uint64(); got != rpPostDeleg {
			t.Fatalf("delegation index BlockNum %d, want the replacement delegate height %d (the dirty branch delegated at %d — stale index reactivated)",
				got, rpPostDeleg, fxPostDeleg)
		}
	}
	if !delegSeen {
		t.Fatal("replacement delegation produced no dvl index")
	}
}
