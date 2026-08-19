package acceptance

import (
	"bytes"
	"math/big"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
	metafixture "github.com/harmony-one/harmony/internal/recovery/metadata/fixture"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/shard/committee"
)

// kvCanonReader adapts the strict source KV to the seed builder's
// canonical-hash reader (raw "h"+num+"n" key, error-propagating).
type kvCanonReader struct {
	open *source.Open
}

func (r kvCanonReader) CanonicalHash(n uint64) (common.Hash, error) {
	key := make([]byte, 0, 10)
	key = append(key, 'h')
	for i := 7; i >= 0; i-- {
		key = append(key, byte(n>>uint(8*i)))
	}
	key = append(key, 'n')
	has, err := r.open.KV.Has(key)
	if err != nil || !has {
		return common.Hash{}, err
	}
	raw, err := r.open.KV.Get(key)
	if err != nil {
		return common.Hash{}, err
	}
	return common.BytesToHash(raw), nil
}

// TestElectionEqualityCleanVsMasked is the B5 bullet-(5) election check in
// its literal form: committee.WithStakingEnabled.Compute for the next
// epoch, evaluated over (a) the CLEAN twin chain ended at the target and
// (b) the dirty chain seen through the audit's masked overlay (deletion
// plan applied, post-target chain records tombstoned, heads rewound),
// must produce byte-identical encoded shard states — the mask changes no
// election input.
func TestElectionEqualityCleanVsMasked(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	// (a) Clean twin: the same deterministic chain ended at the target.
	cleanDir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := metafixture.Open(cleanDir, metafixture.RepoKeysDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Generate(metafixture.Spec{
		Blocks: fxTarget, CreateValidatorAt: fxCreateVal, DelegateAt: fxDelegate,
		FundPrecompileAt: fxFundPrec, // identical pre-target ops = identical blocks 1..target
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.Finalize(); err != nil {
		t.Fatal(err)
	}

	// (b) Dirty chain through the masked overlay.
	dirtyDir := buildFixture(t)
	anchorPath := writeAnchor(t, dirtyDir, fxTarget)
	res, err := anchor.Resolve(anchorPath)
	if err != nil {
		t.Fatal(err)
	}
	open, err := source.OpenSource(dirtyDir, res, dbopen.Options{})
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

	maskedBc, err := core.NewBlockChainWithOptions(
		rawdb.NewDatabase(overlay), nil, nil,
		&core.CacheConfig{Disabled: true, SnapshotLimit: 0},
		&cfg, chain.NewEngine(), vm.Config{}, core.Options{},
	)
	if err != nil {
		t.Fatalf("open masked chain: %v", err)
	}
	if got := maskedBc.CurrentBlock().NumberU64(); got != fxTarget {
		t.Fatalf("masked head %d, want the target %d", got, fxTarget)
	}

	// Read-write: NewBlockChainWithOptions rewrites head markers on open
	// (the clean twin is a disposable per-test generation, not a source).
	cleanDB, err := rawdb.NewLevelDBDatabase(cleanDir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanDB.Close()
	cleanBc, err := core.NewBlockChainWithOptions(
		cleanDB, nil, nil,
		&core.CacheConfig{Disabled: true, SnapshotLimit: 0},
		&cfg, chain.NewEngine(), vm.Config{}, core.Options{},
	)
	if err != nil {
		t.Fatalf("open clean chain: %v", err)
	}
	if got := cleanBc.CurrentBlock().NumberU64(); got != fxTarget {
		t.Fatalf("clean head %d, want the target %d", got, fxTarget)
	}
	if maskedBc.CurrentBlock().Hash() != cleanBc.CurrentBlock().Hash() {
		t.Fatal("masked and clean heads differ (twin construction broken)")
	}

	// The literal election: next-epoch committee.Compute on both views.
	nextEpoch := new(big.Int).SetUint64(res.Config.Epoch + 1)
	maskedSS, err := committee.WithStakingEnabled.Compute(nextEpoch, maskedBc)
	if err != nil {
		t.Fatalf("masked election: %v", err)
	}
	cleanSS, err := committee.WithStakingEnabled.Compute(nextEpoch, cleanBc)
	if err != nil {
		t.Fatalf("clean election: %v", err)
	}
	maskedEnc, err := rlp.EncodeToBytes(maskedSS)
	if err != nil {
		t.Fatal(err)
	}
	cleanEnc, err := rlp.EncodeToBytes(cleanSS)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(maskedEnc, cleanEnc) {
		t.Fatal("next-epoch election over the masked view differs from the clean twin")
	}
}
