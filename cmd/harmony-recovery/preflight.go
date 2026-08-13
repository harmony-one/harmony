package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/certverify"
	"github.com/harmony-one/harmony/internal/recovery/inplace/chainread"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
	"github.com/harmony-one/harmony/internal/recovery/inplace/statecheck"
)

type preflightOptions struct {
	dbPath     string
	name       string
	reportPath string
	network    string
	shard      uint32

	storageWorkers int
	handles        int
	dbCacheMB      int
	trieCacheMB    int

	// Hidden test-only overrides (refused on mainnet).
	targetHeight uint64
	targetHash   string
}

func newPreflightCommand() *cobra.Command {
	opts := &preflightOptions{}
	cmd := &cobra.Command{
		Use:   "preflight --db /path/to/harmony_db_0 [--name \"my-validator\"]",
		Short: "Sample a shard-0 database for recovery target availability (never writes; node may keep running)",
		Long: "preflight verifies - against a live or stopped shard-0 LevelDB - that the\n" +
			"compiled-in recovery target block's header, certificate, ancestry and\n" +
			"complete state are present and cryptographically consistent. It never\n" +
			"writes to the database. The final stdout line is exactly \"PASS\" or\n" +
			"\"FAIL: <reason>\"; a small JSON receipt is written for reporting.\n\n" +
			"Exit codes: 0 PASS, 1 FAIL, 2 unusable (flags/layout), 3 persistent read error.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			code := runPreflight(opts, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if code != report.ExitPass {
				return exitCodeError(code)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.dbPath, "db", "", "path to the node's harmony_db_0 directory (required)")
	f.StringVar(&opts.name, "name", "", "optional validator name for the receipt (coordinators use it to attribute results)")
	f.StringVar(&opts.reportPath, "report", "preflight-result.json", "receipt output path (never inside --db)")
	f.StringVar(&opts.network, "network", "mainnet", "network schedule to verify against")
	f.Uint32Var(&opts.shard, "shard", 0, "shard id (the compiled anchor is shard 0)")
	f.IntVar(&opts.storageWorkers, "storage-workers", 0, "storage/code verification workers (0 = min(8, NumCPU))")
	f.IntVar(&opts.handles, "handles", rodb.DefaultHandles, "open-file cache capacity for the LevelDB reader")
	f.IntVar(&opts.dbCacheMB, "db-cache-mb", rodb.DefaultBlockCacheMB, "LevelDB block cache size in MiB")
	f.IntVar(&opts.trieCacheMB, "trie-cache-mb", 256, "trie clean-cache size in MiB")
	f.Uint64Var(&opts.targetHeight, "target-height", 0, "TEST ONLY: override the target height (refused on mainnet)")
	f.StringVar(&opts.targetHash, "target-hash", "", "TEST ONLY: override the target hash (refused on mainnet)")
	_ = f.MarkHidden("target-height")
	_ = f.MarkHidden("target-hash")
	_ = cmd.MarkFlagRequired("db")
	return cmd
}

// runPreflight is the pipeline: anchor resolution, layout gate, live probe,
// chain checks + certificate, state walk, receipt, exit code.
func runPreflight(opts *preflightOptions, stdout, stderr io.Writer) int {
	started := time.Now()

	usage := func(format string, args ...interface{}) int {
		fmt.Fprintf(stderr, "unusable: "+format+"\n", args...)
		return report.ExitUnusable
	}

	// Anchor + schedule (also validates the flag combination).
	anc, err := anchor.Resolve(opts.network, opts.shard, anchor.Overrides{
		TargetHeight: opts.targetHeight,
		TargetHash:   opts.targetHash,
	})
	if err != nil {
		return usage("%v", err)
	}

	// The report must never land inside the DB directory.
	if opts.reportPath == "" {
		opts.reportPath = "preflight-result.json"
	}
	if err := report.ValidateReportPath(opts.reportPath, opts.dbPath); err != nil {
		return usage("%v", err)
	}

	// File-descriptor budget: fail cleanly up front rather than mid-walk.
	if err := checkFDLimit(opts.handles); err != nil {
		return usage("%v", err)
	}

	// Layout gate: only the default single-LevelDB harmony_db_<shard>
	// layout, and the directory basename must match.
	if err := rodb.CheckLayout(opts.dbPath, opts.shard); err != nil {
		return usage("%v", err)
	}

	// Best-effort, side-effect-free live-writer probe (informational).
	running, known := rodb.ProbeLiveWriter(opts.dbPath)
	var nodeProbablyRunning *bool
	if known {
		nodeProbablyRunning = &running
		if running {
			fmt.Fprintln(stderr, "note: another process holds the database lock (node probably running); results are a point-in-time sample")
		}
	}

	hostname, _ := os.Hostname()
	rec := &report.Receipt{
		Tool:                report.Tool,
		Schema:              report.Schema,
		Build:               buildInfo(),
		Name:                opts.name,
		Hostname:            hostname,
		Network:             opts.network,
		Shard:               opts.shard,
		DBPath:              opts.dbPath,
		NodeProbablyRunning: nodeProbablyRunning,
		SampleNote:          report.SampleNote,
		StartedAt:           started.UTC().Format(time.RFC3339),
		Target: report.Target{
			Height: anc.TargetHeight,
			Hash:   anc.TargetHash.Hex(),
		},
		Checks: report.NewChecks(),
	}

	runner := rodb.NewRunner(opts.dbPath, rodb.Options{
		Handles:      opts.handles,
		BlockCacheMB: opts.dbCacheMB,
	})
	runner.Progress = stderr
	defer runner.Close()

	finish := func(code int, failReason string) int {
		rec.DurationS = time.Since(started).Seconds()
		rec.Retries.ReopenCount = runner.ReopenCount()
		rec.ExitCode = code
		// The receipt result enum is exactly "PASS" | "FAIL" (schema v2); a
		// persistent read error is a FAIL with exit_code 3 distinguishing
		// it from a verification failure (exit_code 1).
		if code == report.ExitPass {
			rec.Result = "PASS"
		} else {
			rec.Result = "FAIL"
			rec.FailReason = failReason
		}
		if err := rec.Write(opts.reportPath); err != nil {
			fmt.Fprintf(stderr, "warning: could not write receipt %s: %v\n", opts.reportPath, err)
		} else {
			fmt.Fprintf(stderr, "receipt written to %s\n", opts.reportPath)
		}
		// The stdout contract holds for every completed run: exactly one
		// "PASS" or "FAIL: <reason>" line. Exit code 3 distinguishes the
		// read-error FAIL class; the remedy goes to stderr.
		switch code {
		case report.ExitPass:
			report.FinalLine(stdout, true, "")
		case report.ExitFail:
			report.FinalLine(stdout, false, failReason)
		case report.ExitReadError:
			report.FinalLine(stdout, false, failReason)
			fmt.Fprintln(stderr, report.RemedyLine)
		}
		return code
	}

	// Stage 1: chain checks (target tuple, body, ancestry, shard state,
	// head sample) + certificate verification.
	var (
		chainOut *chainread.Outcome
		certRes  *certverify.Result
	)
	stageErr := runner.Stage("chain", func(kv *rodb.KV) (err error) {
		defer convertUnexpectedCall(&err)
		out, err := chainread.RunChecks(kv, anc, stderr)
		chainOut = out
		if err != nil {
			return err
		}
		res, err := certverify.Verify(kv, anc, out)
		certRes = res
		if err != nil {
			if f, ok := err.(*report.Failure); ok {
				out.Checks["certificate"] = "fail: " + f.Reason
			}
			return err
		}
		out.Checks["certificate"] = "ok"
		return nil
	})
	if chainOut != nil {
		copyChecks(rec.Checks, chainOut.Checks)
		rec.HeadSample = chainOut.Head
		if chainOut.TargetHeader != nil {
			rec.Target.StateRoot = chainOut.StateRoot.Hex()
			rec.Target.Epoch = chainOut.TargetHeader.Epoch().Uint64()
			rec.Target.ViewID = chainOut.ViewID
		}
	}
	if certRes != nil {
		rec.CertificateSources = certRes.Sources
	}
	if stageErr != nil {
		return finishStageError(stageErr, finish, usage)
	}

	// Stage 2: full target-state completeness walk.
	var stateRes *statecheck.Result
	stageErr = runner.Stage("state", func(kv *rodb.KV) (err error) {
		defer convertUnexpectedCall(&err)
		res, err := statecheck.Walk(statecheck.Config{
			KV:          kv,
			StateRoot:   chainOut.StateRoot,
			TrieCacheMB: opts.trieCacheMB,
			Workers:     opts.storageWorkers,
			Progress:    stderr,
		})
		stateRes = res
		return err
	})
	if stateRes != nil {
		rec.State = report.State{
			Digest:          fmt.Sprintf("%x", stateRes.Digest),
			DigestAlgorithm: statecheck.DigestAlgorithm,
			Counts:          stateRes.Counts,
			Anomalies:       stateRes.Anomalies.Report(),
		}
	}
	if stageErr != nil {
		if f, ok := failureOf(stageErr); ok {
			rec.Checks["state_walk"] = "fail: " + f.Reason
		}
		return finishStageError(stageErr, finish, usage)
	}
	rec.Checks["state_walk"] = "ok"

	if n := runner.Latch().WriteAttempts(); n > 0 {
		// Every attempt was refused; a non-zero count means some code path
		// tried to write. Surface it loudly (fail closed).
		return finish(report.ExitReadError, fmt.Sprintf("internal invariant violated: %d write attempts were made (and refused)", n))
	}
	return finish(report.ExitPass, "")
}

func copyChecks(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

func failureOf(err error) (*report.Failure, bool) {
	var f *report.Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}

// finishStageError maps a stage error to FAIL (exit 1), read error (exit 3)
// or unusable (exit 2).
func finishStageError(err error, finish func(int, string) int, usage func(string, ...interface{}) int) int {
	var unexpected *chainread.UnexpectedCallError
	if errors.As(err, &unexpected) {
		return usage("%v", unexpected)
	}
	if f, ok := failureOf(err); ok {
		return finish(report.ExitFail, f.Error())
	}
	var re *rodb.ReadError
	if errors.As(err, &re) {
		return finish(report.ExitReadError, re.Error())
	}
	return finish(report.ExitReadError, err.Error())
}

// convertUnexpectedCall converts the minimal ChainReader's fail-closed
// panic into an error (mapped to exit 2 by finishStageError).
func convertUnexpectedCall(err *error) {
	if r := recover(); r != nil {
		if uce, ok := r.(*chainread.UnexpectedCallError); ok {
			*err = uce
			return
		}
		panic(r)
	}
}
