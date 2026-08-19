package fixture

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recovery/metadata/refexport"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// Kit schedule constants (WS7). Exported so the generator command, the
// acceptance suite and the reproducibility test all pin the same shape.
const (
	KitBlocks     = 48
	KitTarget     = 30
	KitCreateVal  = 22
	KitDelegate   = 26
	KitFundPrec   = 28
	KitPostCreate = 40
	KitPostDeleg  = 42
	KitPostTopUp  = 43
	KitPrecDirect = 45
	KitPrecNested = 46
	KitPrecRevert = 47
	KitPrecTopUp  = 48
)

// GroundTruthSchema identifies the committed ground-truth document.
const GroundTruthSchema = "metadata-kit-ground-truth-v1"

func generateChain(dir, keysDir string, spec Spec) error {
	c, err := Open(dir, keysDir)
	if err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	if err := c.Generate(spec); err != nil {
		return fmt.Errorf("generate %s: %w", dir, err)
	}
	if err := c.Finalize(); err != nil {
		return fmt.Errorf("finalize %s: %w", dir, err)
	}
	if err := Canonicalize(dir); err != nil {
		return fmt.Errorf("canonicalize %s: %w", dir, err)
	}
	return nil
}

// GenerateKit writes the full committed metadata fixture kit under outRoot
// (which must be absolute — the strict opener refuses relative paths). The
// output is byte-reproducible: two runs into different directories produce
// byte-identical trees (the internal export report, which is run evidence,
// is deliberately excluded). Both the `gen` command and the reproducibility
// test call this so there is exactly one generation path.
func GenerateKit(outRoot, keysDir string, logw io.Writer) error {
	if !filepath.IsAbs(outRoot) {
		return fmt.Errorf("fixture: GenerateKit requires an absolute outRoot, got %q", outRoot)
	}
	if err := os.RemoveAll(outRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(outRoot, 0o755); err != nil {
		return err
	}

	// Dirty chain: pre-target create-validator (22) + delegate (26) +
	// precompile-actor funding (28), target 30 (epoch 2), the 36/37 epoch
	// transition and the post-target abandoned-branch staking schedule
	// plus the 0xfc precompile delegation matrix.
	dirty := filepath.Join(outRoot, "harmony_db_0")
	if err := generateChain(dirty, keysDir, Spec{
		Blocks:                KitBlocks,
		CreateValidatorAt:     KitCreateVal,
		DelegateAt:            KitDelegate,
		FundPrecompileAt:      KitFundPrec,
		PostCreateValidatorAt: KitPostCreate,
		PostDelegateAt:        KitPostDeleg,
		PostTopUpAt:           KitPostTopUp,
		PrecompileDirectAt:    KitPrecDirect,
		PrecompileNestedAt:    KitPrecNested,
		PrecompileRevertAt:    KitPrecRevert,
		PrecompileTopUpAt:     KitPrecTopUp,
	}); err != nil {
		return err
	}

	// Clean twin: the same deterministic chain ended at the target.
	clean := filepath.Join(outRoot, "clean", "harmony_db_0")
	if err := os.MkdirAll(filepath.Dir(clean), 0o755); err != nil {
		return err
	}
	if err := generateChain(clean, keysDir, Spec{
		Blocks:            KitTarget,
		CreateValidatorAt: KitCreateVal,
		DelegateAt:        KitDelegate,
		FundPrecompileAt:  KitFundPrec,
	}); err != nil {
		return err
	}

	// Anchor config (no exploit block planted; empty known-bad list).
	anchorPath := filepath.Join(outRoot, "recovery-anchor.localnet.json")
	if err := WriteAnchorConfig(dirty, KitTarget, KitBlocks, nil, anchorPath); err != nil {
		return fmt.Errorf("anchor: %w", err)
	}

	// Golden export artifacts over the dirty DB. Export publishes into
	// <out>/release/ (one atomic rename); the committed kit keeps the flat
	// reference/ layout, so lift the release files up and drop the export
	// run directory.
	refDir := filepath.Join(outRoot, "reference")
	exportDir := filepath.Join(outRoot, ".export-run")
	if code := refexport.Run(context.Background(), refexport.Options{
		DBPath: dirty, AnchorPath: anchorPath, OutDir: exportDir,
	}, logw); code != 0 {
		return fmt.Errorf("export: exit %d", code)
	}
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{
		fmt.Sprintf("metadata-%d.hmr", KitTarget),
		fmt.Sprintf("metadata-%d.reference.json", KitTarget),
		"run-checksums.sha256",
	} {
		if err := os.Rename(
			filepath.Join(exportDir, "release", name),
			filepath.Join(refDir, name),
		); err != nil {
			return fmt.Errorf("lift release artifact %s: %w", name, err)
		}
	}
	// The internal export report is RUN EVIDENCE (absolute paths, inode,
	// timestamps — machine-specific by design, §4.5) and stays out of the
	// committed kit; only the byte-reproducible release artifacts are kept.
	if err := os.RemoveAll(exportDir); err != nil {
		return fmt.Errorf("drop export run dir: %w", err)
	}

	// Ground truth: the target tuple + artifact digests in one document.
	db, err := rawdb.NewLevelDBDatabase(dirty, 16, 64, "", true)
	if err != nil {
		return fmt.Errorf("open for ground truth: %w", err)
	}
	targetHash := rawdb.ReadCanonicalHash(db, KitTarget)
	childHash := rawdb.ReadCanonicalHash(db, KitTarget+1)
	headHash := rawdb.ReadCanonicalHash(db, KitBlocks)
	db.Close()
	hmrBytes, err := os.ReadFile(filepath.Join(refDir, fmt.Sprintf("metadata-%d.hmr", KitTarget)))
	if err != nil {
		return fmt.Errorf("read golden hmr: %w", err)
	}
	refBytes, err := os.ReadFile(filepath.Join(refDir, fmt.Sprintf("metadata-%d.reference.json", KitTarget)))
	if err != nil {
		return fmt.Errorf("read golden reference: %w", err)
	}
	truth := map[string]interface{}{
		"schema":               GroundTruthSchema,
		"network":              "localnet",
		"blocks":               uint64(KitBlocks),
		"target_height":        uint64(KitTarget),
		"target_hash":          targetHash.Hex(),
		"abandoned_child_hash": childHash.Hex(),
		"head_hash":            headHash.Hex(),
		"staking_schedule": map[string]uint64{
			"create_validator": KitCreateVal, "delegate": KitDelegate,
			"fund_precompile":       KitFundPrec,
			"post_create_validator": KitPostCreate, "post_delegate": KitPostDeleg,
			"post_top_up":       KitPostTopUp,
			"precompile_direct": KitPrecDirect, "precompile_nested": KitPrecNested,
			"precompile_reverted": KitPrecRevert, "precompile_top_up": KitPrecTopUp,
		},
		"hmr_sha256":       report.SHA256Hex(hmrBytes),
		"reference_sha256": report.SHA256Hex(refBytes),
	}
	if err := report.WriteJSONAtomic(filepath.Join(outRoot, "ground-truth.json"), truth); err != nil {
		return fmt.Errorf("ground truth: %w", err)
	}
	fmt.Fprintf(logw, "metadata fixture kit written to %s (head %d, target %d, reference digest %s)\n",
		outRoot, KitBlocks, KitTarget, truth["reference_sha256"])
	return nil
}
