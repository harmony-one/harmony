// Package refexport implements `harmony-recovery metadata export-reference`
// (plan WS5): the run-once reference producer. Any Fatal (incl.
// MissingRequired) finding refuses export; the built-in double-run
// determinism self-check (§4.5, replacing two-donor convergence per §8 Q1)
// derives everything twice over fresh handles and byte-compares both .hmr
// serializations and both reference manifests before writing anything. On
// mismatch: exit 23, no release artifacts, only determinism-diff dumps and
// the export report.
package refexport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/integrity"
	"github.com/harmony-one/harmony/internal/recovery/metadata/hmr"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/scan"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// Schema identifies the export report document.
const Schema = "metadata-export-report-v1"

// Tool is the report tool identifier.
const Tool = "harmony-recovery metadata export-reference"

// Options configures an export run.
type Options struct {
	DBPath     string
	AnchorPath string
	OutDir     string
	Handles    int
	CacheMB    int

	// SkipSelfCheckForTest disables the double-run self-check. It exists
	// only so tests can time nondeterminism faults; the CLI never sets it
	// (the self-check cannot be skipped in a release build, plan WS5).
	SkipSelfCheckForTest bool
}

// Report is the internal export report (run evidence: coverage, per-phase
// deletion counts, timings, created_at — never digested into the
// reference). On a fully clean run the finalized success report is
// published atomically INSIDE release/ together with the artifacts; on a
// failed or refused attempt a failure report is written to the out-dir
// root instead.
type Report struct {
	Tool   string `json:"tool"`
	Schema string `json:"schema"`

	Network string `json:"network"`
	Shard   uint32 `json:"shard"`
	DBPath  string `json:"db_path"`

	AnchorConfigSHA string          `json:"anchor_config_sha256"`
	Anchor          hmr.AnchorTuple `json:"anchor"`

	NormalizedValidatorListLength int `json:"normalized_validator_list_length"`

	Counts     norm.Counts             `json:"counts"`
	Coverage   norm.SnapshotCoverage   `json:"snapshot_coverage"`
	Findings   scan.FindingsSection    `json:"findings"`
	Plan       scan.PlanSection        `json:"deletion_plan"`
	Digests    norm.DigestSet          `json:"digests"`
	Assertions []norm.AbsenceAssertion `json:"absence_assertions"` // carry planned_deletions (run evidence)

	Refused      bool   `json:"refused"`
	RefuseReason string `json:"refuse_reason,omitempty"`

	Determinism struct {
		Ran            bool   `json:"ran"`
		Passed         bool   `json:"passed"`
		FirstDiff      string `json:"first_diff,omitempty"`
		DiffDumpPrefix string `json:"diff_dump_prefix,omitempty"`
	} `json:"determinism_self_check"`

	Artifacts struct {
		// ReleaseDir is the directory (relative to --out-dir) the release
		// set was atomically published into; set only on a fully clean run.
		ReleaseDir    string `json:"release_dir,omitempty"`
		HMRFile       string `json:"hmr_file,omitempty"`
		HMRSHA256     string `json:"hmr_sha256,omitempty"`
		ManifestFile  string `json:"manifest_file,omitempty"`
		ManifestSHA   string `json:"manifest_sha256,omitempty"` // THE reference digest
		ChecksumsFile string `json:"checksums_file,omitempty"`
	} `json:"artifacts"`

	FingerprintBefore *dbopen.Fingerprint `json:"db_fingerprint_before"`
	FingerprintAfter  *dbopen.Fingerprint `json:"db_fingerprint_after"`
	ZeroWriteProof    bool                `json:"zero_write_proof"`
	WriteAttempts     int                 `json:"write_attempts"`

	CreatedAt string  `json:"created_at"`
	DurationS float64 `json:"duration_s"`

	Verdict  string `json:"verdict"`
	ExitCode int    `json:"exit_code"`
}

type derivation struct {
	res      *norm.Result
	hmrBytes []byte
	manifest []byte
}

// TestMutatePassB is a test-only hook that perturbs the second derivation's
// serialized bytes so tests can exercise the determinism self-check. It is
// nil in every non-test build; the self-check itself cannot be skipped in a
// release build (there is no CLI flag for it).
var TestMutatePassB func(hmrBytes, manifest []byte) ([]byte, []byte)

// TestVerifyFault, when non-nil, is consulted for each staged artifact
// during pre-publication verification; a non-nil return simulates a
// verification failure so tests can prove a set that cannot be verified is
// never published under the release name. Never set in a release build.
var TestVerifyFault func(name string) error

// TestPromoteFault, when non-nil, is consulted before the single atomic
// directory rename that publishes the release; a non-nil return simulates
// the rename failing so tests can prove nothing ever appears under the
// release name. Never set in a release build.
var TestPromoteFault func(releaseDir string) error

// TestCleanupFault, when non-nil, is consulted before the best-effort
// removal of the staging directory; a non-nil return simulates that removal
// failing so tests can prove a failed cleanup changes neither the exit code
// nor what is visible under the release name (staging is not
// consumer-facing). Never set in a release build.
var TestCleanupFault func(staging string) error

// TestReportFault, when non-nil, is consulted before writing a report
// document at path (both the staged success report and the root failure
// report); a non-nil return simulates the write failing so tests can prove
// a report failure never publishes a release and never leaves a
// success-shaped report visible. Never set in a release build.
var TestReportFault func(path string) error

// Run executes export-reference. Artifact names follow the release list:
// metadata-<target>.hmr, metadata-<target>.reference.json.
func Run(ctx context.Context, opts Options, stderr io.Writer) int {
	started := time.Now()
	usage := func(format string, args ...interface{}) int {
		fmt.Fprintf(stderr, "invalid invocation: "+format+"\n", args...)
		return report.ExitBadInvocation
	}
	if opts.DBPath == "" || opts.AnchorPath == "" || opts.OutDir == "" {
		return usage("--db, --anchor and --out-dir are required")
	}
	res, err := anchor.Resolve(opts.AnchorPath)
	if err != nil {
		return usage("%v", err)
	}
	if err := dbopen.ValidateOutputPath(opts.OutDir, opts.DBPath); err != nil {
		return usage("%v", err)
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create out dir: %v\n", err)
		return report.ExitIO
	}
	// Run-once producer: refuse an out-dir already holding a release
	// directory (a previous run's output must never be mistaken for this
	// run's; the operator picks a fresh directory). Anything already
	// sitting at the release path — including a stat error that is not
	// IsNotExist — refuses the run: publication is a single rename onto
	// this name and must never race a pre-existing entry.
	if _, serr := os.Lstat(filepath.Join(opts.OutDir, releaseDirName)); serr == nil {
		return usage("out-dir already contains %s/ from a previous run; export-reference is a run-once producer — use a fresh --out-dir", releaseDirName)
	} else if !os.IsNotExist(serr) {
		fmt.Fprintf(stderr, "error: probe %s: %v\n", filepath.Join(opts.OutDir, releaseDirName), serr)
		return report.ExitIO
	}

	fpBefore, err := dbopen.FingerprintDir(opts.DBPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: fingerprint source: %v\n", err)
		return report.ExitIO
	}

	open, err := source.OpenSource(opts.DBPath, res, dbopen.Options{Handles: opts.Handles, BlockCacheMB: opts.CacheMB})
	if err != nil {
		code := dbopen.ClassifyExit(err)
		fmt.Fprintf(stderr, "error: open source: %v\n", err)
		return code
	}
	defer open.Close()

	rep := &Report{
		Tool:            Tool,
		Schema:          Schema,
		Network:         res.Config.Network,
		Shard:           res.Config.Shard,
		DBPath:          open.DB.Path(),
		AnchorConfigSHA: res.ConfigSHAHex(),
		CreatedAt:       started.UTC().Format(time.RFC3339),
	}
	rep.Anchor = hmr.AnchorTuple{
		TargetHeight:       open.NormA.TargetHeight,
		TargetHash:         open.NormA.TargetHash.Hex(),
		TargetRoot:         open.NormA.TargetRoot.Hex(),
		Epoch:              open.NormA.Epoch,
		EpochFirstBlock:    open.NormA.EpochFirst,
		EpochLastBlock:     open.NormA.EpochLast,
		SnapshotBaseHeight: open.NormA.SnapshotBase,
		AbandonedChildHash: open.NormA.AbandonedChildHash.Hex(),
	}

	staging := filepath.Join(opts.OutDir, stagingDirName)
	code := runExport(ctx, opts, res, open, rep, staging, stderr)

	// Source-immutability gate (fingerprint compared before/after,
	// device/inode included): any read failure or mismatch is exit 14 and
	// refuses the staged artifacts.
	rep.FingerprintBefore = fpBefore
	if fpAfter, ferr := dbopen.FingerprintDir(opts.DBPath); ferr != nil {
		fmt.Fprintf(stderr, "error: fingerprint source after run: %v\n", ferr)
		code = report.ResolveExit(code, report.ExitIO)
	} else {
		rep.FingerprintAfter = fpAfter
		rep.ZeroWriteProof = fpBefore.Equal(fpAfter) && open.DB.WriteAttempts() == 0
	}
	rep.WriteAttempts = open.DB.WriteAttempts()
	if rep.WriteAttempts > 0 {
		fmt.Fprintf(stderr, "internal invariant violated: %d write attempts were made (and refused)\n", rep.WriteAttempts)
		code = report.ResolveExit(code, report.ExitIO)
	}
	if rep.FingerprintAfter != nil && !rep.ZeroWriteProof {
		fmt.Fprintf(stderr, "error: source fingerprint changed during the export (zero-write proof failed); artifacts refused\n")
		code = report.ResolveExit(code, report.ExitIO)
	}
	if ctx.Err() != nil {
		code = report.ResolveExit(code, report.InterruptExit(ctx))
	}

	// Publication protocol (one atomic unit): on a fully clean run the
	// finalized SUCCESS report is staged NEXT TO the release artifacts and
	// the whole set — artifacts, checksums AND report — becomes visible
	// through the single atomic directory rename to release/. There is no
	// window in which release/ exists without its success report, or in
	// which a success-shaped report is consumer-visible without release/:
	// before the rename nothing consumer-visible exists (a crash there
	// strands only the non-consumer .staging directory); after it the
	// complete unit exists. On ANY failed or refused attempt — including a
	// failed publication — a separate FAILURE report is written to the
	// out-dir root instead; a success-shaped report only ever exists
	// inside release/.
	publishable := code == report.ExitOK && rep.Artifacts.HMRFile != ""
	finalize := func() {
		rep.DurationS = time.Since(started).Seconds()
		rep.ExitCode = code
		rep.Verdict = scan.Verdict(code)
	}
	if publishable {
		rep.Artifacts.ChecksumsFile = checksumsName
		rep.Artifacts.ReleaseDir = releaseDirName
		finalize()
		if err := writeReport(filepath.Join(staging, reportName), rep); err != nil {
			fmt.Fprintf(stderr, "error: stage export report: %v; nothing was published under %s/\n",
				err, filepath.Join(opts.OutDir, releaseDirName))
			code = report.ResolveExit(code, report.ExitIO)
		} else {
			names := []string{rep.Artifacts.HMRFile, rep.Artifacts.ManifestFile, checksumsName, reportName}
			if pcode := publishRelease(staging, opts.OutDir, names, stderr); pcode != report.ExitOK {
				code = report.ResolveExit(code, pcode)
			} else {
				releaseDir := filepath.Join(opts.OutDir, releaseDirName)
				fmt.Fprintf(stderr, "wrote %s (package sha %s)\n", filepath.Join(releaseDir, rep.Artifacts.HMRFile), rep.Artifacts.HMRSHA256)
				fmt.Fprintf(stderr, "wrote %s (reference digest %s)\n", filepath.Join(releaseDir, rep.Artifacts.ManifestFile), rep.Artifacts.ManifestSHA)
				fmt.Fprintf(stderr, "export report published to %s\n", filepath.Join(releaseDir, reportName))
				// The staging directory was renamed away; nothing to clean.
				return code
			}
		}
	}
	// Failed or refused attempt: nothing was published (release/ is
	// absent). Record the failure report at the out-dir root.
	rep.Artifacts.HMRFile, rep.Artifacts.ManifestFile, rep.Artifacts.ChecksumsFile = "", "", ""
	rep.Artifacts.HMRSHA256, rep.Artifacts.ManifestSHA = "", ""
	rep.Artifacts.ReleaseDir = ""
	finalize()
	reportPath := filepath.Join(opts.OutDir, reportName)
	if err := writeReport(reportPath, rep); err != nil {
		// Even the failure report could not be recorded: the process exit
		// code still carries the failure, and no success-shaped report
		// exists anywhere consumer-visible (it only ever lives inside
		// release/, which was never created).
		fmt.Fprintf(stderr, "error: write export report: %v\n", err)
		code = report.ResolveExit(code, report.ExitIO)
	} else {
		fmt.Fprintf(stderr, "export report written to %s\n", reportPath)
	}
	// Remove staging leftovers. Staging is not a consumer-facing name, so
	// a failed removal here is a warning: it can never expose anything
	// under the release name.
	if err := removeStaging(staging); err != nil {
		fmt.Fprintf(stderr, "warning: remove staging dir %s: %v\n", staging, err)
	}
	return code
}

// writeReport writes one report document (with the test-only report fault
// applied first).
func writeReport(path string, rep *Report) error {
	if TestReportFault != nil {
		if err := TestReportFault(path); err != nil {
			return err
		}
	}
	return report.WriteJSONAtomic(path, rep)
}

// publishRelease publishes the staged release set into outDir/release with
// ONE atomic rename. The protocol makes an incomplete set under the release
// name structurally impossible, on every failure path:
//
//  1. VERIFY the staged set BEFORE anything is visible: the staging
//     directory must contain exactly the expected artifact names — nothing
//     missing, nothing extra, nothing empty. Every error counts as a
//     verification failure (not just IsNotExist). A set that cannot be
//     verified is simply not published; the release name never appears.
//  2. Publish with a single os.Rename(staging → outDir/release). rename(2)
//     is atomic on POSIX (staging lives inside outDir, so no cross-device
//     fallback exists): the release directory either appears fully
//     populated with the verified set, or — on any failure — does not
//     appear at all. There is no partially-promoted state to clean up, so
//     no cleanup step whose own failure could strand a partial set; a
//     leftover staging directory is not a consumer-facing name.
//
// Returns report.ExitOK only when the verified set is published.
func publishRelease(staging, outDir string, names []string, stderr io.Writer) int {
	fail := func(err error) int {
		fmt.Fprintf(stderr, "error: %v; nothing was published under %s/ (the release name appears only with the complete verified set)\n",
			err, filepath.Join(outDir, releaseDirName))
		return report.ExitIO
	}
	// Step 1: verify the staged set is exactly the expected release set.
	entries, err := os.ReadDir(staging)
	if err != nil {
		return fail(fmt.Errorf("verify staged release set: %w", err))
	}
	expected := map[string]bool{}
	for _, name := range names {
		expected[name] = true
	}
	for _, e := range entries {
		if !expected[e.Name()] {
			return fail(fmt.Errorf("verify staged release set: unexpected stray file %q in staging", e.Name()))
		}
	}
	for _, name := range names {
		var err error
		if TestVerifyFault != nil {
			err = TestVerifyFault(name)
		}
		if err == nil {
			var fi os.FileInfo
			if fi, err = os.Stat(filepath.Join(staging, name)); err == nil && fi.Size() == 0 {
				err = errors.New("zero-length artifact")
			}
		}
		if err != nil {
			return fail(fmt.Errorf("verify staged release set: %s: %w", name, err))
		}
	}
	// Step 2: one atomic rename.
	dst := filepath.Join(outDir, releaseDirName)
	if TestPromoteFault != nil {
		if ferr := TestPromoteFault(dst); ferr != nil {
			return fail(fmt.Errorf("publish release directory: %w", ferr))
		}
	}
	if err := os.Rename(staging, dst); err != nil {
		return fail(fmt.Errorf("publish release directory: %w", err))
	}
	return report.ExitOK
}

// removeStaging best-effort-removes the staging directory (with the
// test-only cleanup fault applied first).
func removeStaging(staging string) error {
	if TestCleanupFault != nil {
		if err := TestCleanupFault(staging); err != nil {
			return err
		}
	}
	return os.RemoveAll(staging)
}

// stagingDirName is the private staging area inside --out-dir; the whole
// directory is renamed to releaseDirName in one atomic step on a fully
// clean run.
const stagingDirName = ".staging"

// releaseDirName is the consumer-facing release directory inside --out-dir.
// It exists if and only if a run completed fully clean, and then holds the
// complete verified release set.
const releaseDirName = "release"

// checksumsName is the internal run checksum file (§4.7 tier (a)).
const checksumsName = "run-checksums.sha256"

// reportName is the export report document. On a fully clean run it is
// staged with the release artifacts and published atomically inside
// release/; on a failed or refused attempt a failure report is written to
// the out-dir root instead.
const reportName = "export-report.json"

// derive runs one full derivation over fresh iterators/state handles.
func derive(ctx context.Context, open *source.Open, res *anchor.Resolved) (*derivation, error) {
	srcs, err := open.BuildSources()
	if err != nil {
		return nil, err
	}
	srcs.Ctx = ctx // long raw iterations observe cancellation (SIGINT)
	nres, err := norm.Normalize(open.NormA, srcs)
	if err != nil {
		return nil, err
	}
	d := &derivation{res: nres}
	if nres.HasFatalOrMissing() {
		return d, nil // refusal handled by the caller; no encode possible
	}
	if d.hmrBytes, err = hmr.Encode(nres.Normalized, res.ConfigSHA); err != nil {
		return nil, err
	}
	manifest := hmr.BuildManifest(open.NormA, nres, d.hmrBytes)
	if d.manifest, err = hmr.EncodeManifest(manifest); err != nil {
		return nil, err
	}
	return d, nil
}

func runExport(ctx context.Context, opts Options, res *anchor.Resolved, open *source.Open, rep *Report, staging string, stderr io.Writer) int {
	fillSections := func(r *norm.Result) int {
		rep.NormalizedValidatorListLength = r.NormalizedListLength
		rep.Counts = r.Counts
		rep.Coverage = r.Coverage
		rep.Digests = r.Digests
		rep.Assertions = r.Assertions
		var err error
		if rep.Findings, err = scan.BuildFindingsSection(r.Findings); err != nil {
			fmt.Fprintf(stderr, "error: findings section: %v\n", err)
			return report.ExitIO
		}
		if rep.Plan, err = scan.BuildPlanSection(r.Deletions); err != nil {
			fmt.Fprintf(stderr, "error: plan section: %v\n", err)
			return report.ExitIO
		}
		return 0
	}

	passA, err := derive(ctx, open, res)
	if err != nil {
		return classifyDeriveError(ctx, err, stderr)
	}
	if c := fillSections(passA.res); c != 0 {
		return c
	}
	if passA.res.HasFatalOrMissing() {
		// Fatal or MissingRequired refuses export (ReviewItems allowed,
		// recorded in the diagnostics digest).
		rep.Refused = true
		rep.RefuseReason = "normalization produced fatal findings; export refused (see findings)"
		fmt.Fprintf(stderr, "export refused: fatal findings present\n")
		return passA.res.ExitCode()
	}
	if ctx.Err() != nil {
		return report.InterruptExit(ctx)
	}

	// Determinism self-check: full second derivation over fresh handles.
	if !opts.SkipSelfCheckForTest {
		rep.Determinism.Ran = true
		passB, err := derive(ctx, open, res)
		if err != nil {
			return classifyDeriveError(ctx, err, stderr)
		}
		if TestMutatePassB != nil {
			// Test-only fault injection: perturb the second derivation's
			// serialization to prove the self-check catches nondeterminism.
			// Never set in a release build.
			passB.hmrBytes, passB.manifest = TestMutatePassB(passB.hmrBytes, passB.manifest)
		}
		if diff := firstDifference(passA, passB); diff != "" {
			rep.Determinism.Passed = false
			rep.Determinism.FirstDiff = diff
			prefix := filepath.Join(opts.OutDir, "determinism-diff")
			rep.Determinism.DiffDumpPrefix = prefix
			if err := os.MkdirAll(prefix, 0o755); err == nil {
				_ = report.WriteFileAtomic(filepath.Join(prefix, "pass-a.hmr"), passA.hmrBytes)
				_ = report.WriteFileAtomic(filepath.Join(prefix, "pass-b.hmr"), passB.hmrBytes)
				_ = report.WriteFileAtomic(filepath.Join(prefix, "pass-a.reference.json"), passA.manifest)
				_ = report.WriteFileAtomic(filepath.Join(prefix, "pass-b.reference.json"), passB.manifest)
			}
			fmt.Fprintf(stderr, "DETERMINISM_MISMATCH: %s — no release artifacts emitted; dumps under %s\n", diff, prefix)
			return report.ExitDeterminismMismatch
		}
		rep.Determinism.Passed = true
	}

	// Stage artifacts privately (only after the self-check passed); the
	// caller promotes them together after the source-immutability gate.
	hmrName := fmt.Sprintf("metadata-%d.hmr", open.NormA.TargetHeight)
	refName := fmt.Sprintf("metadata-%d.reference.json", open.NormA.TargetHeight)
	if err := os.RemoveAll(staging); err != nil {
		fmt.Fprintf(stderr, "error: reset staging dir: %v\n", err)
		return report.ExitIO
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: create staging dir: %v\n", err)
		return report.ExitIO
	}
	if err := report.WriteFileAtomic(filepath.Join(staging, hmrName), passA.hmrBytes); err != nil {
		fmt.Fprintf(stderr, "error: stage %s: %v\n", hmrName, err)
		return report.ExitIO
	}
	if err := report.WriteFileAtomic(filepath.Join(staging, refName), passA.manifest); err != nil {
		fmt.Fprintf(stderr, "error: stage %s: %v\n", refName, err)
		return report.ExitIO
	}
	// Internal run checksums (§4.7 tier (a)), staged alongside.
	if err := integrity.WriteSums(filepath.Join(staging, checksumsName), []string{hmrName, refName}); err != nil {
		fmt.Fprintf(stderr, "error: stage run checksums: %v\n", err)
		return report.ExitIO
	}
	rep.Artifacts.HMRFile = hmrName
	rep.Artifacts.HMRSHA256 = report.SHA256Hex(passA.hmrBytes)
	rep.Artifacts.ManifestFile = refName
	rep.Artifacts.ManifestSHA = report.SHA256Hex(passA.manifest)
	return report.ExitOK
}

func classifyDeriveError(ctx context.Context, err error, stderr io.Writer) int {
	var tse *source.TargetStateError
	if errors.As(err, &tse) {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return report.ExitTargetStateUnavailable
	}
	fmt.Fprintf(stderr, "error: %v\n", err)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return report.InterruptExit(ctx)
	}
	return report.ExitIO
}

// firstDifference byte-compares both serializations and reports the first
// differing artifact and offset ("" when identical).
func firstDifference(a, b *derivation) string {
	if !bytes.Equal(a.hmrBytes, b.hmrBytes) {
		return fmt.Sprintf("hmr bytes differ at offset %d (lens %d vs %d)",
			firstDiffOffset(a.hmrBytes, b.hmrBytes), len(a.hmrBytes), len(b.hmrBytes))
	}
	if !bytes.Equal(a.manifest, b.manifest) {
		return fmt.Sprintf("reference manifests differ at offset %d (lens %d vs %d)",
			firstDiffOffset(a.manifest, b.manifest), len(a.manifest), len(b.manifest))
	}
	return ""
}

func firstDiffOffset(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
