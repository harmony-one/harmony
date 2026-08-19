package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/internal/recovery/report"
	staking "github.com/harmony-one/harmony/staking/types"
)

// Tool is the report tool identifier.
const Tool = "harmony-recovery metadata audit-branch"

// Schema identifies the audit report document.
const Schema = "abandoned-branch-audit-v1"

// Options configures an audit run.
type Options struct {
	DBPath     string
	AnchorPath string
	OutDir     string
	Scratch    string
	EndHeight  uint64 // 0 = anchor audit_end_height
	KeepScratch bool
	SinglePass  bool // debug only; output marked non-authoritative

	// TrustedShard1Pointer optionally names a pre-incident pointer per
	// shard: "<shardID>:<blockNum>". Accepted only if it satisfies the
	// §4.4 invariants.
	TrustedShard1Pointer string
	TrustedProvenance    string

	// ReferencePath optionally names the exported reference manifest
	// (metadata-<target>.reference.json); the audit cross-checks it
	// against the resolved anchor and binds it into the report hash chain.
	ReferencePath string

	ScratchReserveGB int // pre-run free-space check (default 200; must be >= 0)

	// SkipReserveCheckForTest disables the mandatory scratch free-space
	// gate. It exists only for tests on small CI filesystems; the CLI
	// never sets it (there is no flag for it and negative
	// --scratch-reserve-gb values are rejected).
	SkipReserveCheckForTest bool

	Handles int
	CacheMB int
}

// blockOutcome is one height's record-mode + execution result.
type blockOutcome struct {
	Height        uint64   `json:"height"`
	Hash          string   `json:"hash"`
	Executed      bool     `json:"executed"`
	RootMatched   bool     `json:"root_matched"`
	ValidityFails []string `json:"validity_failures,omitempty"`
}

// NativeOp is one staking transaction from a branch block body.
type NativeOp struct {
	Block     uint64 `json:"block"`
	Directive string `json:"directive"`
	Address   string `json:"address,omitempty"`   // validator (create/edit) or delegator
	Validator string `json:"validator,omitempty"` // delegate/undelegate target
	Amount    string `json:"amount,omitempty"`
	TxHash    string `json:"tx_hash"`
}

// passResult carries one pass's outputs.
type passResult struct {
	Outcomes    []blockOutcome
	Findings    []report.Finding
	FCOps       []FCOp
	NativeOps   []NativeOp
	Log         map[string]WriteLogEntry
	PointerEnd  map[uint32][]byte // final pointer values in the overlay per shard
	SeedSpec    *SeedSpec
	Fatal       bool
	FatalHeight uint64
	FatalReason string
	// LegacyBitmapsRestored counts incoming-receipt proofs whose Copy-bug
	// corrupted CommitBitmap was verifiably restored (legacybitmap.go).
	LegacyBitmapsRestored int
	overlay               *Overlay
}

// Run executes the audit. See run() for the pipeline; this wrapper owns
// scratch lifecycle.
//
// NOTE (process isolation): internal/chain keeps process-wide reward-payout
// caches keyed only by (epoch, shard) / (epoch, validator) — not by chain
// identity. The production harmony-recovery binary opens exactly one chain
// per process, so the caches are always coherent there. Test code that opens
// MORE than one chain in a process (several generated fixtures) must isolate
// reward-sensitive runs in a subprocess instead; see
// acceptance.runIsolatedSubtest. Purging those caches from here was rejected
// in review as an out-of-scope internal/chain change.
func Run(ctx context.Context, opts Options, stderr io.Writer) int {
	code, scratchUsed := run(ctx, opts, stderr)
	if scratchUsed != "" && !opts.KeepScratch && code == report.ExitOK {
		if err := os.RemoveAll(scratchUsed); err != nil {
			fmt.Fprintf(stderr, "warning: could not remove scratch %s: %v\n", scratchUsed, err)
		}
	} else if scratchUsed != "" {
		fmt.Fprintf(stderr, "scratch retained at %s\n", scratchUsed)
	}
	return code
}

func run(ctx context.Context, opts Options, stderr io.Writer) (int, string) {
	started := time.Now()
	usage := func(format string, args ...interface{}) (int, string) {
		fmt.Fprintf(stderr, "invalid invocation: "+format+"\n", args...)
		return report.ExitBadInvocation, ""
	}
	if opts.DBPath == "" || opts.AnchorPath == "" || opts.OutDir == "" || opts.Scratch == "" {
		return usage("--db, --anchor, --out-dir and --scratch are required")
	}
	if opts.ScratchReserveGB < 0 {
		return usage("--scratch-reserve-gb must be >= 0 (the reserve gate cannot be bypassed from the CLI)")
	}
	res, err := anchor.Resolve(opts.AnchorPath)
	if err != nil {
		return usage("%v", err)
	}
	for _, p := range []string{opts.OutDir, opts.Scratch} {
		if err := dbopen.ValidateOutputPath(p, opts.DBPath); err != nil {
			return usage("%v", err)
		}
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create out dir: %v\n", err)
		return report.ExitIO, ""
	}
	if err := os.MkdirAll(opts.Scratch, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create scratch dir: %v\n", err)
		return report.ExitIO, ""
	}
	reserve := opts.ScratchReserveGB
	if reserve == 0 {
		reserve = 200
	}
	if !opts.SkipReserveCheckForTest {
		// The reserve gate FAILS CLOSED: if free space cannot be
		// established the run is refused (exit 14), never allowed through.
		free, ferr := dbopen.FreeBytes(opts.Scratch)
		if ferr != nil {
			fmt.Fprintf(stderr, "error: cannot determine free space on the scratch filesystem: %v\n", ferr)
			return report.ExitIO, ""
		}
		if free < uint64(reserve)<<30 {
			fmt.Fprintf(stderr, "error: scratch filesystem has %d GiB free, %d GiB reserve required (--scratch-reserve-gb)\n", free>>30, reserve)
			return report.ExitBadInvocation, ""
		}
	}

	// Source-immutability evidence: fingerprint before the strict open,
	// re-fingerprint after the audit; any change or read failure is exit
	// 14 (the audit writes only to scratch — the source must be inert).
	fpBefore, err := dbopen.FingerprintDir(opts.DBPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: fingerprint source: %v\n", err)
		return report.ExitIO, ""
	}

	// Open the source strictly and normalize (the audit's own derivation:
	// the mask is built from it).
	open, err := source.OpenSource(opts.DBPath, res, dbopen.Options{Handles: opts.Handles, BlockCacheMB: opts.CacheMB})
	if err != nil {
		fmt.Fprintf(stderr, "error: open source: %v\n", err)
		return dbopen.ClassifyExit(err), ""
	}
	defer open.Close()
	srcs, err := open.BuildSources()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return report.ExitTargetStateUnavailable, ""
	}
	srcs.Ctx = ctx // long raw iterations observe cancellation (SIGINT)
	nres, err := norm.Normalize(open.NormA, srcs)
	if err != nil {
		fmt.Fprintf(stderr, "error: normalize: %v\n", err)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return report.InterruptExit(ctx), ""
		}
		return report.ExitIO, ""
	}
	if nres.HasFatalOrMissing() {
		// The mask would be built from an unreliable plan; the §4.5 codes
		// route the operator to the right investigation first.
		fmt.Fprintf(stderr, "audit refused: normalization produced fatal findings (run metadata scan for the report)\n")
		return nres.ExitCode(), ""
	}

	sd := newSide(open.KV)
	endHeight := opts.EndHeight
	if endHeight == 0 {
		endHeight = res.Config.AuditEndHeight
	}

	// Preconditions (plan §4.6).
	if code := checkPreconditions(sd, res, endHeight, stderr); code != 0 {
		return code, ""
	}

	rep := newReport(res, open, opts, endHeight, started)
	rep.FingerprintBefore = fpBefore

	// Optional reference cross-check (§4.6): the exported reference
	// manifest must bind to the same anchor; mismatch is an invocation
	// error (wrong reference for this anchor).
	if opts.ReferencePath != "" {
		if code := loadReference(rep, opts.ReferencePath, res, open.NormA, nres, stderr); code != 0 {
			return code, ""
		}
	}

	// PASS 1 — mask discovery: branch crosslink/spent records unmasked;
	// affected findings classified pollution-suspect.
	pass1Scratch := filepath.Join(opts.Scratch, "pass-1")
	pass1, code := runPass(ctx, passConfig{
		label: "pass-1", scratch: pass1Scratch, side: sd, open: open, res: nres,
		anchorRes: res, endHeight: endHeight, stderr: stderr,
	})
	defer closeOverlay(pass1)
	if code != 0 {
		writeFatalReport(rep, pass1, nil, opts, stderr, code)
		return code, opts.Scratch
	}

	// Branch-written crosslink/spent subsets from the pass-1 write log.
	subsets := extractShardSubsets(pass1.Log)

	// Pointer solving per shard (§4.4 invariant solver).
	pointer, code := solvePointers(sd, nres, subsets, opts, rep, stderr)
	if code != 0 && code != report.ExitAuditAnomaly {
		return code, opts.Scratch
	}
	pointerAmbiguous := code == report.ExitAuditAnomaly

	var pass2 *passResult
	if !opts.SinglePass {
		// PASS 2 — authoritative: scratch reset, pass-1 subsets join the
		// mask, derived pointers seeded.
		pass2Scratch := filepath.Join(opts.Scratch, "pass-2")
		pass2, code = runPass(ctx, passConfig{
			label: "pass-2", scratch: pass2Scratch, side: sd, open: open, res: nres,
			anchorRes: res, endHeight: endHeight, stderr: stderr,
			extraMask: subsets.allKeys(), pointerSeeds: pointer.seeds,
		})
		defer closeOverlay(pass2)
		if code != 0 {
			writeFatalReport(rep, pass1, pass2, opts, stderr, code)
			return code, opts.Scratch
		}
		// State roots must be identical across passes.
		if code := crossPassRootCheck(pass1, pass2, rep, stderr); code != 0 {
			writeFatalReport(rep, pass1, pass2, opts, stderr, code)
			return code, opts.Scratch
		}
	}

	authoritative := pass2
	if opts.SinglePass {
		authoritative = pass1
		rep.NonAuthoritative = true
	}

	// Reconciliation + report assembly over the authoritative pass.
	exit := assembleReport(rep, nres, sd, open, pass1, authoritative, subsets, pointer, pointerAmbiguous)

	// Bind the reference into the report hash chain: anchor config →
	// reference digest → per-pass outcome digests.
	if rep.Reference != nil {
		chain := rep.AnchorConfigSHA + rep.Reference.ManifestSHA
		if rep.Pass1 != nil {
			chain += rep.Pass1.OutcomesSHA256
		}
		if rep.Pass2 != nil {
			chain += rep.Pass2.OutcomesSHA256
		}
		rep.Reference.HashChain = report.SHA256Hex([]byte(chain))
	}

	// Source-immutability gate: re-fingerprint and require equality
	// (device/inode included). Failure is exit 14, never an anomaly.
	if fpAfter, ferr := dbopen.FingerprintDir(opts.DBPath); ferr != nil {
		fmt.Fprintf(stderr, "error: fingerprint source after audit: %v\n", ferr)
		exit = report.ResolveExit(exit, report.ExitIO)
	} else {
		rep.FingerprintAfter = fpAfter
		rep.SourceUnchanged = fpBefore.Equal(fpAfter)
		if !rep.SourceUnchanged {
			fmt.Fprintf(stderr, "error: source fingerprint changed during the audit\n")
			exit = report.ResolveExit(exit, report.ExitIO)
		}
	}

	rep.DurationS = time.Since(started).Seconds()
	rep.ExitCode = exit
	reportPath := filepath.Join(opts.OutDir, "abandoned-branch-audit.json")
	if err := report.WriteJSONAtomic(reportPath, rep); err != nil {
		fmt.Fprintf(stderr, "error: write audit report: %v\n", err)
		return report.ResolveExit(exit, report.ExitIO), opts.Scratch
	}
	fmt.Fprintf(stderr, "audit report written to %s\n", reportPath)
	return exit, opts.Scratch
}

func checkPreconditions(sd *side, res *anchor.Resolved, endHeight uint64, stderr io.Writer) int {
	child, err := sd.Header(res.Config.TargetHeight + 1)
	if err != nil {
		// A read/decode failure is an I/O error (14), never a "missing
		// header" invocation error: the header may well exist.
		fmt.Fprintf(stderr, "error: abandoned child header: %v\n", err)
		return report.ExitIO
	}
	if child == nil {
		fmt.Fprintf(stderr, "error: no canonical header at %d (abandoned child) in the source\n", res.Config.TargetHeight+1)
		return report.ExitBadInvocation
	}
	if child.Hash() != res.ChildHash {
		fmt.Fprintf(stderr, "error: canonical(%d) = %s, anchor abandoned_child_hash = %s\n",
			res.Config.TargetHeight+1, child.Hash().Hex(), res.Config.AbandonedChildHash)
		return report.ExitBadInvocation
	}
	if child.ParentHash() != res.TargetHash {
		fmt.Fprintf(stderr, "error: abandoned child %d parent %s is not the target %s\n",
			res.Config.TargetHeight+1, child.ParentHash().Hex(), res.Config.TargetHash)
		return report.ExitBadInvocation
	}
	head, err := sd.HeadHeight()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return report.ExitIO
	}
	if head < endHeight {
		fmt.Fprintf(stderr, "error: source head %d is below --end-height %d\n", head, endHeight)
		return report.ExitBadInvocation
	}
	return 0
}

type passConfig struct {
	label        string
	scratch      string
	side         *side
	open         *source.Open
	res          *norm.Result
	anchorRes    *anchor.Resolved
	endHeight    uint64
	stderr       io.Writer
	extraMask    [][]byte
	pointerSeeds map[string][]byte
}

// runPass executes one full masked re-execution pass.
func runPass(ctx context.Context, pc passConfig) (*passResult, int) {
	if err := os.RemoveAll(pc.scratch); err != nil {
		fmt.Fprintf(pc.stderr, "error: reset scratch: %v\n", err)
		return nil, report.ExitIO
	}
	overlay, err := NewOverlay(pc.scratch, pc.open.KV)
	if err != nil {
		fmt.Fprintf(pc.stderr, "error: open scratch overlay: %v\n", err)
		return nil, report.ExitIO
	}
	pr := &passResult{overlay: overlay}
	// The overlay is NOT closed here: reconciliation reads final values
	// from the authoritative pass's scratch after runPass returns. The
	// caller (run) owns overlay lifetime and closes both passes' overlays.

	spec, err := Seed(overlay, pc.res, pc.side, pc.anchorRes.TargetHash, pc.anchorRes.Config.TargetHeight,
		pc.extraMask, pc.pointerSeeds)
	if err != nil {
		fmt.Fprintf(pc.stderr, "error: seed overlay: %v\n", err)
		return nil, report.ExitIO
	}
	pr.SeedSpec = spec

	// Harness (plan §4.6): archival cache config over the overlay,
	// self-beacon shard 0, stock engine. The chain never initializes
	// networking, RPC, txpool, consensus services, or BLS signing.
	chainConfig := chainConfigFor(pc.anchorRes)
	db := rawdb.NewDatabase(overlay)
	bc, err := core.NewBlockChainWithOptions(
		db, nil, nil,
		&core.CacheConfig{Disabled: true, Preimages: false, SnapshotLimit: 0},
		chainConfig, chain.NewEngine(), vm.Config{}, core.Options{},
	)
	if err != nil {
		fmt.Fprintf(pc.stderr, "error: open overlay chain: %v\n", err)
		return nil, report.ExitIO
	}
	if got := bc.CurrentBlock().Hash(); got != pc.anchorRes.TargetHash {
		fmt.Fprintf(pc.stderr, "error: overlay head is %s, want the target %s (seed failure)\n",
			got.Hex(), pc.anchorRes.Config.TargetHash)
		return nil, report.ExitIO
	}

	tracer := newFCTracer()
	target := pc.anchorRes.Config.TargetHeight
	prevHash := pc.anchorRes.TargetHash
	for n := target + 1; n <= pc.endHeight; n++ {
		if ctx.Err() != nil {
			return pr, report.InterruptExit(ctx)
		}
		blk, err := pc.side.Block(n)
		// Source-identity failures collected for this height (recorded
		// ahead of the record-mode checks): a canonical mapping whose
		// record decodes to a different hash/height is a MANDATORY
		// validity failure — a redirected mapping over otherwise-valid
		// bytes passes ancestry, cryptographic and execution checks, so
		// without this finding it could exit 0. Validation continues over
		// the decoded content to classify any accompanying tamper too.
		var srcFails []string
		if err != nil {
			var ide *identityError
			if !errors.As(err, &ide) || ide.block == nil {
				fmt.Fprintf(pc.stderr, "error: %v\n", err)
				return pr, report.ExitIO
			}
			blk = ide.block
			srcFails = append(srcFails, recordSourceIdentityFail(ide, n, pc.stderr, &pr.Findings))
		}
		out := blockOutcome{Height: n, Hash: blk.Hash().Hex()}

		// Abandoned-parent ancestry (first parent == target hash; block
		// target+1's hash == anchor AbandonedChildHash).
		if blk.ParentHash() != prevHash {
			pr.Fatal, pr.FatalHeight = true, n
			pr.FatalReason = fmt.Sprintf("branch ancestry broken at %d: parent %s, want %s", n, blk.ParentHash().Hex(), prevHash.Hex())
			fmt.Fprintf(pc.stderr, "fatal: %s\n", pr.FatalReason)
			pr.Outcomes = append(pr.Outcomes, out)
			return pr, report.ExitBadInvocation
		}
		if n == target+1 && blk.Hash() != pc.anchorRes.ChildHash {
			pr.Fatal, pr.FatalHeight = true, n
			pr.FatalReason = fmt.Sprintf("block %d hash %s is not the anchored abandoned child %s", n, blk.Hash().Hex(), pc.anchorRes.Config.AbandonedChildHash)
			fmt.Fprintf(pc.stderr, "fatal: %s\n", pr.FatalReason)
			pr.Outcomes = append(pr.Outcomes, out)
			return pr, report.ExitBadInvocation
		}
		prevHash = blk.Hash()

		// Commit signature: child header or exact block-sig key. A child
		// record failing identity validation still yields its material
		// (the cryptographic checks below convict tampered material), but
		// the identity mismatch itself is a mandatory validity failure —
		// even material that verifies (a redirected mapping over a copy of
		// the true child) must gate the audit.
		sigAndBitmap, sigIdent, err := pc.side.CommitSigFor(n)
		if err != nil {
			fmt.Fprintf(pc.stderr, "error: %v\n", err)
			return pr, report.ExitIO
		}
		if sigIdent != nil {
			srcFails = append(srcFails, recordSourceIdentityFail(sigIdent, n, pc.stderr, &pr.Findings))
		}
		blk.SetCurrentCommitSig(sigAndBitmap)

		// Repair legacy Copy-bug CommitBitmap corruption on stored incoming
		// receipt proofs (legacybitmap.go) before validation: mainnet bodies
		// stored the aggregate signature where the quorum bitmap belongs, so
		// without a verified restoration every receipt-carrying block would
		// fail validation for a storage artifact, not a chain defect.
		restored, restoreNotes := restoreLegacyReceiptBitmaps(pc.side, blk)
		if restored > 0 {
			pr.LegacyBitmapsRestored += restored
			pr.Findings = append(pr.Findings, report.Finding{
				Severity: report.SeverityInfo,
				Class:    report.ClassDiagnostic,
				Code:     "legacy-receipt-bitmap-restored",
				Key:      fmt.Sprintf("%d", n),
				Detail:   fmt.Sprintf("restored %d Copy-bug-corrupted incoming-receipt bitmap(s) from stored crosslinks; header incoming-receipt commitment reproduced", restored),
			})
		}
		for _, note := range restoreNotes {
			pr.Findings = append(pr.Findings, report.Finding{
				Severity: report.SeverityReviewItem,
				Class:    report.ClassDiagnostic,
				Code:     "legacy-receipt-bitmap-unrestored",
				Key:      fmt.Sprintf("%d", n),
				Detail:   note,
			})
		}

		// Record-mode validity checks: each failure is a Finding, not an
		// abort (§4.6). Exploit blocks are EXPECTED to fail patched
		// receipt validation while re-executing to their header roots.
		// Source-identity failures recorded above lead the list.
		out.ValidityFails = append(srcFails, recordModeChecks(bc, blk, sigAndBitmap, pc.label, &pr.Findings)...)

		// Native staking-op inventory from the body.
		pr.NativeOps = append(pr.NativeOps, nativeOps(blk)...)

		// Traced execution: the traced Process result lands in the
		// processor result cache; the immediately following InsertChain
		// consumes it verbatim (§2.3) — the traced execution IS the
		// committed execution.
		parentState, err := bc.StateAt(bc.CurrentBlock().Root())
		if err != nil {
			pr.Fatal, pr.FatalHeight, pr.FatalReason = true, n, fmt.Sprintf("parent state unavailable at %d: %v", n, err)
			fmt.Fprintf(pc.stderr, "fatal: %s\n", pr.FatalReason)
			pr.Outcomes = append(pr.Outcomes, out)
			return pr, report.ExitAuditAnomaly
		}
		tracer.BeginBlock(n)
		_, _, _, _, _, _, _, err = bc.Processor().Process(blk, parentState, vm.Config{Debug: true, Tracer: tracer}, false)
		if err != nil {
			pr.Fatal, pr.FatalHeight, pr.FatalReason = true, n, fmt.Sprintf("branch execution failed at %d: %v", n, err)
			fmt.Fprintf(pc.stderr, "fatal: %s\n", pr.FatalReason)
			pr.Outcomes = append(pr.Outcomes, out)
			return pr, report.ExitAuditAnomaly
		}
		out.Executed = true
		if _, err := bc.InsertChain(types.Blocks{blk}, false); err != nil {
			// ValidateState failures (root/receipt/gas/bloom divergence)
			// land here: state-root divergence on the branch would be a
			// major discovery; stop, scratch preserved.
			pr.Fatal, pr.FatalHeight, pr.FatalReason = true, n, fmt.Sprintf("insert failed at %d (root/state validation): %v", n, err)
			fmt.Fprintf(pc.stderr, "fatal: %s\n", pr.FatalReason)
			pr.Outcomes = append(pr.Outcomes, out)
			return pr, report.ExitAuditAnomaly
		}
		out.RootMatched = true
		pr.Outcomes = append(pr.Outcomes, out)
	}

	pr.FCOps = tracer.Ops()
	pr.Log = overlay.Log()
	pr.PointerEnd = readPointerEnds(overlay)
	return pr, 0
}

// chainConfigFor copies the network chain config with beacon-shard chain-id
// semantics (internal/shardchain/shardchains.go:130-133).
func chainConfigFor(res *anchor.Resolved) *params.ChainConfig {
	nt := nodeconfig.NetworkType(res.Config.Network)
	cfg := nt.ChainConfig()
	if res.Config.Shard == 0 {
		cfg.EthCompatibleChainID = big.NewInt(cfg.EthCompatibleShard0ChainID.Int64())
	}
	return &cfg
}

// recordSourceIdentityFail records one canonical-mapping identity mismatch
// (redirected/tampered header or block record) as a validity failure of the
// block being audited: it joins that block's ValidityFails (gating the
// audit to a non-zero exit through the known-bad cross-check — the gate
// excuses only incoming-receipts failures, never source-identity) and is
// itemized as a Finding. Returns the failure label.
func recordSourceIdentityFail(ide *identityError, atBlock uint64, stderr io.Writer, findings *[]report.Finding) string {
	fmt.Fprintf(stderr, "source-identity failure at %d: %v\n", atBlock, ide)
	*findings = append(*findings, report.Finding{
		Severity: report.SeverityReviewItem,
		Class:    report.ClassDiagnostic,
		Code:     "branch-validity-source-identity",
		Key:      fmt.Sprintf("%d", atBlock),
		Detail:   ide.Error(),
	})
	return fmt.Sprintf("source-identity: %v", ide)
}

// recordModeChecks runs the §4.6 per-block validity checks; failures become
// findings (pollution-suspect classification for the crosslink/spent checks
// happens in pass 1 — pass 2 masks the pollution, plan §4.6).
func recordModeChecks(bc core.BlockChain, blk *types.Block, sigAndBitmap []byte, label string, findings *[]report.Finding) []string {
	var fails []string
	addFail := func(check string, err error, pollutable bool) {
		if err == nil {
			return
		}
		fails = append(fails, fmt.Sprintf("%s: %v", check, err))
		class := report.ClassDiagnostic
		if pollutable && label == "pass-1" {
			class = report.ClassPollutionSuspect
		}
		*findings = append(*findings, report.Finding{
			Severity: report.SeverityReviewItem,
			Class:    class,
			Code:     "branch-validity-" + check,
			Key:      fmt.Sprintf("%d", blk.NumberU64()),
			Detail:   err.Error(),
		})
	}
	header := blk.Header()
	engine := bc.Engine()
	// InsertChain(..., false) skips general header verification entirely
	// (§2.3) — VerifyHeader incl. seal is mandatory here.
	addFail("verify-header", engine.VerifyHeader(bc, header, true), false)
	if len(sigAndBitmap) > 96 {
		var sig bls_cosi.SerializedSignature
		copy(sig[:], sigAndBitmap[:96])
		addFail("header-signature", engine.VerifyHeaderSignature(bc, header, sig, sigAndBitmap[96:]), false)
	} else {
		addFail("header-signature", fmt.Errorf("commit signature material too short (%d bytes)", len(sigAndBitmap)), false)
	}
	addFail("vrf", engine.VerifyVRF(bc, header), false)
	addFail("shard-state", engine.VerifyShardState(bc, bc, header), false)
	if len(header.CrossLinks()) > 0 {
		// Rejects any crosslink already in the DB (errAlreadyExist):
		// pass-1 pollution; masked in pass 2.
		addFail("crosslinks", core.VerifyBlockCrossLinks(bc, blk), true)
	}
	// IsSpent sees future markers in pass 1: pollution; masked in pass 2.
	addFail("incoming-receipts", core.VerifyIncomingReceipts(bc, blk), true)
	return fails
}

// nativeOps inventories the staking transactions of one block by
// directive (§4.6 output 2).
func nativeOps(blk *types.Block) []NativeOp {
	var out []NativeOp
	for _, tx := range blk.StakingTransactions() {
		op := NativeOp{
			Block:     blk.NumberU64(),
			Directive: tx.StakingType().String(),
			TxHash:    tx.Hash().Hex(),
		}
		if payload, err := tx.RLPEncodeStakeMsg(); err == nil {
			if decoded, err := staking.RLPDecodeStakeMsg(payload, tx.StakingType()); err == nil {
				switch m := decoded.(type) {
				case *staking.CreateValidator:
					op.Address = m.ValidatorAddress.Hex()
				case *staking.EditValidator:
					op.Address = m.ValidatorAddress.Hex()
				case *staking.Delegate:
					op.Address = m.DelegatorAddress.Hex()
					op.Validator = m.ValidatorAddress.Hex()
					op.Amount = bigString(m.Amount)
				case *staking.Undelegate:
					op.Address = m.DelegatorAddress.Hex()
					op.Validator = m.ValidatorAddress.Hex()
					op.Amount = bigString(m.Amount)
				case *staking.CollectRewards:
					op.Address = m.DelegatorAddress.Hex()
				}
			}
		}
		out = append(out, op)
	}
	return out
}

// readPointerEnds reads the final pointer values from the overlay.
func readPointerEnds(o *Overlay) map[uint32][]byte {
	out := map[uint32][]byte{}
	for sid := uint32(0); sid < 8; sid++ {
		key := append([]byte("cl"), u32be4(sid)...)
		if v, err := o.Get(key); err == nil {
			out[sid] = v
		}
	}
	return out
}

func u32be4(n uint32) []byte {
	return []byte{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

// closeOverlay closes a pass's scratch overlay (owned by run, not runPass,
// because reconciliation reads final values after the pass returns).
func closeOverlay(pr *passResult) {
	if pr != nil && pr.overlay != nil {
		_ = pr.overlay.Close()
	}
}

// crossPassRootCheck: any root difference across passes is Fatal (§4.6).
func crossPassRootCheck(p1, p2 *passResult, rep *Report, stderr io.Writer) int {
	if len(p1.Outcomes) != len(p2.Outcomes) {
		fmt.Fprintf(stderr, "fatal: pass outcome counts differ (%d vs %d)\n", len(p1.Outcomes), len(p2.Outcomes))
		return report.ExitAuditAnomaly
	}
	for i := range p1.Outcomes {
		a, b := p1.Outcomes[i], p2.Outcomes[i]
		if a.Hash != b.Hash || a.RootMatched != b.RootMatched {
			fmt.Fprintf(stderr, "fatal: pass results diverge at height %d\n", a.Height)
			return report.ExitAuditAnomaly
		}
	}
	return 0
}
