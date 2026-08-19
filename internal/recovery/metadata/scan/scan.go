// Package scan implements `harmony-recovery metadata scan` (plan WS4): the
// internal diagnosis/dry-run — strict open, anchor cross-verification,
// target resolution, one Normalize pass, and the full report with bounded
// itemization and overflow digests. It never writes to the source
// (zero-write proof: source fingerprints compared before/after, and the
// handle is the write-refusing wrapper).
package scan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/hmr"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
	"github.com/harmony-one/harmony/internal/recovery/metadata/source"
	"github.com/harmony-one/harmony/internal/recovery/report"
)

// Schema identifies the scan report document.
const Schema = "metadata-scan-report-v1"

// Tool is the report tool identifier.
const Tool = "harmony-recovery metadata scan"

// ItemLimit bounds report itemization; the full lists are digested.
const ItemLimit = 200

// PreflightNote records the §8 Q2 decision in every report.
const PreflightNote = "validator-list completeness (the combined-omission case) rests on the separately-run preflight state traversal; " +
	"the normalized list length above is printed for a manual, informational comparison — no receipt input, no gating"

// Options configures a scan run.
type Options struct {
	DBPath     string
	AnchorPath string
	ReportPath string
	Handles    int
	CacheMB    int
}

// FindingsSection is the bounded findings report.
type FindingsSection struct {
	Total       int              `json:"total"`
	Fatal       int              `json:"fatal"`
	ReviewItems int              `json:"review_items"`
	Info        int              `json:"info"`
	Items       []report.Finding `json:"items"`
	Omitted     int              `json:"omitted"`
	FullListSHA string           `json:"full_list_sha256"`
}

// BuildFindingsSection bounds and digests a finding list.
func BuildFindingsSection(fs []report.Finding) (FindingsSection, error) {
	sec := FindingsSection{Total: len(fs)}
	for _, f := range fs {
		switch f.Severity {
		case report.SeverityFatal:
			sec.Fatal++
		case report.SeverityReviewItem:
			sec.ReviewItems++
		default:
			sec.Info++
		}
	}
	full, err := report.DigestCanonicalJSON(fs)
	if err != nil {
		return sec, err
	}
	sec.FullListSHA = full
	if len(fs) > ItemLimit {
		sec.Items = fs[:ItemLimit]
		sec.Omitted = len(fs) - ItemLimit
	} else {
		sec.Items = fs
	}
	return sec, nil
}

// PhaseSummary is one deletion-plan phase with bounded itemization.
type PhaseSummary struct {
	Name             string                 `json:"name"`
	DeletionCount    int                    `json:"deletion_count"`
	RewriteCount     int                    `json:"rewrite_count"`
	PlaceholderCount int                    `json:"placeholder_count"`
	Deletions        []norm.PlannedDeletion `json:"deletions,omitempty"`
	DeletionsOmitted int                    `json:"deletions_omitted"`
	Rewrites         []norm.PlannedRewrite  `json:"rewrites,omitempty"`
	RewritesOmitted  int                    `json:"rewrites_omitted"`
	Placeholders     []norm.Placeholder     `json:"placeholders,omitempty"`
}

// PlanSection summarizes the deletion plan; apply re-derives, the digest
// authenticates.
type PlanSection struct {
	Phases  []PhaseSummary `json:"phases"`
	PlanSHA string         `json:"plan_sha256"`
}

// BuildPlanSection bounds and digests a deletion plan.
func BuildPlanSection(p *norm.DeletionPlan) (PlanSection, error) {
	sha, err := report.DigestCanonicalJSON(p)
	if err != nil {
		return PlanSection{}, err
	}
	sec := PlanSection{PlanSHA: sha}
	for _, ph := range p.Phases {
		s := PhaseSummary{
			Name:             ph.Name,
			DeletionCount:    len(ph.Deletions),
			RewriteCount:     len(ph.Rewrites),
			PlaceholderCount: len(ph.Placeholders),
			Placeholders:     ph.Placeholders,
		}
		if len(ph.Deletions) > ItemLimit {
			s.Deletions = ph.Deletions[:ItemLimit]
			s.DeletionsOmitted = len(ph.Deletions) - ItemLimit
		} else {
			s.Deletions = ph.Deletions
		}
		if len(ph.Rewrites) > ItemLimit {
			s.Rewrites = ph.Rewrites[:ItemLimit]
			s.RewritesOmitted = len(ph.Rewrites) - ItemLimit
		} else {
			s.Rewrites = ph.Rewrites
		}
		sec.Phases = append(sec.Phases, s)
	}
	return sec, nil
}

// Report is the scan report document.
type Report struct {
	Tool   string `json:"tool"`
	Schema string `json:"schema"`

	Network string `json:"network"`
	Shard   uint32 `json:"shard"`
	DBPath  string `json:"db_path"`

	AnchorConfigSHA string          `json:"anchor_config_sha256"`
	Anchor          hmr.AnchorTuple `json:"anchor"`

	// NormalizedValidatorListLength is printed prominently for the manual
	// preflight comparison (§8 Q2).
	NormalizedValidatorListLength int    `json:"normalized_validator_list_length"`
	PreflightNote                 string `json:"preflight_note"`

	Counts     norm.Counts             `json:"counts"`
	Coverage   norm.SnapshotCoverage   `json:"snapshot_coverage"`
	Findings   FindingsSection         `json:"findings"`
	Plan       PlanSection             `json:"deletion_plan"`
	Digests    norm.DigestSet          `json:"digests"`
	Assertions []norm.AbsenceAssertion `json:"absence_assertions"`
	Inventory  norm.Inventory          `json:"inventory"`

	FingerprintBefore *dbopen.Fingerprint `json:"db_fingerprint_before"`
	FingerprintAfter  *dbopen.Fingerprint `json:"db_fingerprint_after"`
	ZeroWriteProof    bool                `json:"zero_write_proof"`
	WriteAttempts     int                 `json:"write_attempts"`

	StartedAt string  `json:"started_at"` // run evidence, not digested
	DurationS float64 `json:"duration_s"`

	Verdict  string `json:"verdict"`
	ExitCode int    `json:"exit_code"`
}

// Run executes the scan pipeline and writes the report. The returned exit
// code follows the §4.5 precedence table.
func Run(ctx context.Context, opts Options, stderr io.Writer) int {
	started := time.Now()
	usage := func(format string, args ...interface{}) int {
		fmt.Fprintf(stderr, "invalid invocation: "+format+"\n", args...)
		return report.ExitBadInvocation
	}
	if opts.DBPath == "" || opts.AnchorPath == "" || opts.ReportPath == "" {
		return usage("--db, --anchor and --report are required")
	}
	res, err := anchor.Resolve(opts.AnchorPath)
	if err != nil {
		return usage("%v", err)
	}
	if err := dbopen.ValidateOutputPath(opts.ReportPath, opts.DBPath); err != nil {
		return usage("%v", err)
	}

	fpBefore, err := dbopen.FingerprintDir(opts.DBPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: fingerprint source: %v\n", err)
		return report.ExitIO
	}

	open, err := source.OpenSource(opts.DBPath, res, dbopen.Options{Handles: opts.Handles, BlockCacheMB: opts.CacheMB})
	if err != nil {
		code := dbopen.ClassifyExit(err)
		// Path/layout mistakes are invocation errors, not I/O.
		if code == report.ExitIO && isLayoutError(err) {
			code = report.ExitBadInvocation
		}
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
		PreflightNote:   PreflightNote,
		StartedAt:       started.UTC().Format(time.RFC3339),
	}

	code, err := runNormalize(ctx, open, rep)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if code == 0 {
			code = report.ExitIO
		}
	}

	// Source-immutability gate (fingerprints include device/inode): a
	// failed re-fingerprint or any mismatch is exit 14, never ignored.
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
		fmt.Fprintf(stderr, "error: source fingerprint changed during the scan (zero-write proof failed)\n")
		code = report.ResolveExit(code, report.ExitIO)
	}

	if ctx.Err() != nil {
		code = report.ResolveExit(code, interruptExit(ctx))
	}

	rep.DurationS = time.Since(started).Seconds()
	rep.ExitCode = code
	rep.Verdict = verdictFor(code)
	if err := report.WriteJSONAtomic(opts.ReportPath, rep); err != nil {
		fmt.Fprintf(stderr, "error: write report %s: %v\n", opts.ReportPath, err)
		return report.ResolveExit(code, report.ExitIO)
	}
	fmt.Fprintf(stderr, "report written to %s\n", opts.ReportPath)
	fmt.Fprintf(stderr, "normalized validator list length: %d (%s)\n",
		rep.NormalizedValidatorListLength, "compare manually against the preflight receipt, §8 Q2")
	return code
}

func runNormalize(ctx context.Context, open *source.Open, rep *Report) (int, error) {
	srcs, err := open.BuildSources()
	if err != nil {
		var tse *source.TargetStateError
		if errors.As(err, &tse) {
			rep.Verdict = "TARGET_STATE_UNAVAILABLE"
			return report.ExitTargetStateUnavailable, err
		}
		return report.ExitIO, err
	}
	if ctx.Err() != nil {
		return interruptExit(ctx), ctx.Err()
	}
	srcs.Ctx = ctx // long raw iterations observe cancellation (SIGINT)
	res, err := norm.Normalize(open.NormA, srcs)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return interruptExit(ctx), err
		}
		return report.ExitIO, err
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
	rep.NormalizedValidatorListLength = res.NormalizedListLength
	rep.Counts = res.Counts
	rep.Coverage = res.Coverage
	rep.Digests = res.Digests
	rep.Assertions = res.Assertions
	rep.Inventory = res.Inventory
	if rep.Findings, err = BuildFindingsSection(res.Findings); err != nil {
		return report.ExitIO, err
	}
	if rep.Plan, err = BuildPlanSection(res.Deletions); err != nil {
		return report.ExitIO, err
	}
	return res.ExitCode(), nil
}

func verdictFor(code int) string {
	switch code {
	case report.ExitOK:
		return "OK"
	case report.ExitMissingRequired:
		return "MISSING_REQUIRED_METADATA"
	case report.ExitInvalidRetained:
		return "INVALID_RETAINED_METADATA"
	case report.ExitTargetStateUnavailable:
		return "TARGET_STATE_UNAVAILABLE"
	case report.ExitDeterminismMismatch:
		return "DETERMINISM_MISMATCH"
	case report.ExitAuditAnomaly:
		return "AUDIT_ANOMALY"
	case report.ExitUnsafeOpen:
		return "UNSAFE_OPEN"
	case report.ExitIO:
		return "IO_ERROR"
	case report.ExitBadInvocation:
		return "INVALID_INVOCATION"
	case report.ExitInterrupted, report.ExitSIGINT:
		return "INTERRUPTED"
	default:
		return fmt.Sprintf("EXIT_%d", code)
	}
}

// Verdict exposes verdictFor for sibling commands.
func Verdict(code int) string { return verdictFor(code) }

// isLayoutError distinguishes layout/path invocation mistakes (exit 15)
// from real I/O failures (exit 14). dbopen layout errors are produced
// before any LevelDB open.
func isLayoutError(err error) bool {
	s := err.Error()
	for _, sub := range []string{
		"must be an absolute path",
		"does not match the expected",
		"sharded (harmony_sharddb_*) layout",
		"is a symlink",
		"is not a directory",
		"does not look like a LevelDB",
		"resolves inside the source DB",
		"anchor:",
		"source: canonical hash",
		"source: header-number reverse mapping",
	} {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func interruptExit(ctx context.Context) int { return report.InterruptExit(ctx) }
