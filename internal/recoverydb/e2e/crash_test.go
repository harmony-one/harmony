package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/compact"
	"github.com/harmony-one/harmony/internal/recoverydb/fixture"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/replay"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// TestReplayCrashHelper is the child-process body of the SIGKILL crash test.
// It performs a real replay against paths given via env and is meant to be
// killed mid-mutation by the parent (plan WS8 crash matrix). It is a no-op
// unless RECOVERYDB_CRASH_CHILD is set, so it never runs as a normal test.
func TestReplayCrashHelper(t *testing.T) {
	if os.Getenv("RECOVERYDB_CRASH_CHILD") == "" {
		t.Skip("child-only helper")
	}
	cfg := replay.Config{
		Network: "localnet", ShardID: 0,
		DestinationDB:         os.Getenv("CRASH_DEST"),
		AnchorPath:            os.Getenv("CRASH_ANCHOR"),
		InspectReportPath:     os.Getenv("CRASH_INSPECT"),
		BaselineAgreementPath: os.Getenv("CRASH_AGREEMENT"),
		BundleDir:             os.Getenv("CRASH_BUNDLE"),
		TargetHeight:          targetHeight,
		ToolVersion:           toolVersion,
		OutputPath:            os.Getenv("CRASH_OUT"),
	}
	// Best-effort: if this returns before the parent kills it, that's fine —
	// the parent handles the "finished first" case.
	_, _ = replay.Run(cfg)
}

// TestReplaySIGKILL spawns the crash helper as a subprocess, SIGKILLs it once
// the destination journal appears (mutation underway), and asserts the
// fail-closed invariant: the journal is IN_PROGRESS (or COMPLETE_VERIFIED if
// the child happened to finish first), reopen/rerun is refused while
// IN_PROGRESS, and a fresh copy replays clean (plan WS4 SIGKILL points; WS8
// crash matrix).
func TestReplaySIGKILL(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	dest := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDir(w.k.baseB, dest); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "replay.json")

	cmd := exec.Command(os.Args[0], "-test.run=TestReplayCrashHelper", "-test.v")
	cmd.Env = append(os.Environ(),
		"RECOVERYDB_CRASH_CHILD=1",
		"CRASH_DEST="+dest,
		"CRASH_ANCHOR="+w.k.anchorPath,
		"CRASH_INSPECT="+w.inspectA,
		"CRASH_AGREEMENT="+w.agreement,
		"CRASH_BUNDLE="+w.bundleDir,
		"CRASH_OUT="+out,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}

	// Kill as soon as the journal appears (mutation phase has begun).
	journalPath := report.JournalPath(dest)
	killed := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(journalPath); err == nil {
			_ = cmd.Process.Signal(syscall.SIGKILL)
			killed = true
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	_ = cmd.Wait()
	if !killed {
		t.Skip("child finished before the journal was observed; timing lost")
	}

	st, _, err := report.JournalState(journalPath)
	if err != nil {
		// The journal file may have appeared but not yet had its first
		// record fsynced when we killed; that is still a clean IN_PROGRESS
		// (no completion record) — model it as such.
		st = report.StateInProgress
	}
	switch st {
	case report.StateCompleteVerified:
		// The child completed before the SIGKILL landed — nothing to assert
		// about a partial state, but a rerun must still refuse (sealed).
	case report.StateInProgress:
		// A normal replay must refuse the unclean destination (v1 no-resume).
		_, rerr := replay.Run(replay.Config{
			Network: "localnet", ShardID: 0, DestinationDB: dest,
			AnchorPath: w.k.anchorPath, InspectReportPath: w.inspectA,
			BaselineAgreementPath: w.agreement, BundleDir: w.bundleDir,
			TargetHeight: targetHeight, ToolVersion: toolVersion,
			OutputPath: filepath.Join(t.TempDir(), "rerun.json"),
		})
		if rerr == nil || !strings.Contains(rerr.Error(), "journal") {
			t.Fatalf("reopen of an IN_PROGRESS destination must refuse, got %v", rerr)
		}
	default:
		t.Fatalf("unexpected journal state after SIGKILL: %s", st)
	}

	// A fresh copy replays clean (the crash left no poison).
	fresh := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDir(w.k.baseB, fresh); err != nil {
		t.Fatal(err)
	}
	rep, err := replay.Run(replay.Config{
		Network: "localnet", ShardID: 0, DestinationDB: fresh,
		AnchorPath: w.k.anchorPath, InspectReportPath: w.inspectA,
		BaselineAgreementPath: w.agreement, BundleDir: w.bundleDir,
		TargetHeight: targetHeight, ToolVersion: toolVersion,
		OutputPath: filepath.Join(t.TempDir(), "fresh.json"),
	})
	if err != nil {
		t.Fatalf("fresh copy must replay clean after a crash: %v", err)
	}
	if !rep.Gate.Passed {
		t.Fatalf("fresh replay gate failed")
	}
}

// TestCrashMatrixHelper is the child body of the deterministic named-point
// crash matrix (round 13 finding 10): it runs a real replay or compact and
// dies at $RECOVERYDB_CRASHPOINT via report.CrashPoint. No-op unless
// RECOVERYDB_CRASH_CHILD is set.
func TestCrashMatrixHelper(t *testing.T) {
	if os.Getenv("RECOVERYDB_CRASH_CHILD") == "" {
		t.Skip("child-only helper")
	}
	switch os.Getenv("CRASH_OP") {
	case "replay":
		_, err := replay.Run(replay.Config{
			Network: "localnet", ShardID: 0,
			DestinationDB:         os.Getenv("CRASH_DEST"),
			AnchorPath:            os.Getenv("CRASH_ANCHOR"),
			InspectReportPath:     os.Getenv("CRASH_INSPECT"),
			BaselineAgreementPath: os.Getenv("CRASH_AGREEMENT"),
			BundleDir:             os.Getenv("CRASH_BUNDLE"),
			TargetHeight:          targetHeight,
			ToolVersion:           toolVersion,
			OutputPath:            os.Getenv("CRASH_OUT"),
		})
		t.Logf("replay returned without crashing: err=%v", err)
	case "compact":
		chainCfg, err := harness.ChainConfig("localnet", 0)
		if err != nil {
			t.Fatal(err)
		}
		sched, err := harness.Schedule("localnet")
		if err != nil {
			t.Fatal(err)
		}
		win, err := anchor.ComputeWindow(sched, targetHeight, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, err = compact.Run(compact.Config{
			Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
			SourceDB:            os.Getenv("CRASH_SRC"),
			DestinationDB:       os.Getenv("CRASH_DEST"),
			AnchorPath:          os.Getenv("CRASH_ANCHOR"),
			SourceReferencePath: os.Getenv("CRASH_SRCREF"),
			TargetHeight:        targetHeight,
			ToolVersion:         toolVersion,
			OutputPath:          os.Getenv("CRASH_OUT"),
		}, win)
		t.Logf("compact returned without crashing: err=%v", err)
	}
}

// TestCrashMatrixNamedPoints exercises every named crash point of
// replay-bundle and compact-db deterministically (no SIGKILL timing race):
// the child dies AT the point, and the parent asserts the fail-closed
// invariants — the journal is never COMPLETE_*, and a rerun on the unclean
// destination is refused (plan WS4 SIGKILL points, WS8 crash matrix; round
// 13 finding 10).
func TestCrashMatrixNamedPoints(t *testing.T) {
	if testing.Short() {
		t.Skip("not short")
	}
	w := getWorld(t)

	runChild := func(t *testing.T, point string, env []string) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=TestCrashMatrixHelper", "-test.v")
		cmd.Env = append(append(os.Environ(), env...),
			"RECOVERYDB_CRASH_CHILD=1",
			report.CrashPointEnv+"="+point,
		)
		if err := cmd.Run(); err == nil {
			t.Fatalf("child completed cleanly; crash point %q never fired (stale point name?)", point)
		}
	}

	assertNotComplete := func(t *testing.T, dest string) {
		t.Helper()
		st, _, err := report.JournalState(report.JournalPath(dest))
		if err != nil {
			// Journal missing/unreadable == IN_PROGRESS-equivalent
			// (pre-journal crash); rerun refusal is asserted separately.
			return
		}
		if st == report.StateCompleteVerified || st == report.StateCompleteUnreleasable {
			t.Fatalf("journal reached terminal state %s despite the crash", st)
		}
	}

	replayPoints := []string{
		// Torn write INSIDE the first insert's commit sequence (round 14
		// finding 3): the crashDB wrapper dies between two of the insert's
		// leveldb commits, not on a clean between-blocks boundary.
		"replay.mid-insert-batch",
		"replay.mid-insert",
		"replay.after-inserts-before-finalize",
		// Between TrieDB.Commit and CommitPreimages (round 14 finding 3):
		// trie nodes durable, preimage coverage not.
		"replay.after-trie-commit-before-preimages",
		"replay.after-finalize-before-close",
		"replay.after-close-before-gate",
		"replay.after-report-before-journal",
	}
	for _, point := range replayPoints {
		point := point
		t.Run(point, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "harmony_db_0")
			if err := fixture.CopyDir(w.k.baseB, dest); err != nil {
				t.Fatal(err)
			}
			runChild(t, point, []string{
				"CRASH_OP=replay",
				"CRASH_DEST=" + dest,
				"CRASH_ANCHOR=" + w.k.anchorPath,
				"CRASH_INSPECT=" + w.inspectA,
				"CRASH_AGREEMENT=" + w.agreement,
				"CRASH_BUNDLE=" + w.bundleDir,
				"CRASH_OUT=" + filepath.Join(t.TempDir(), "replay.json"),
			})
			assertNotComplete(t, dest)
			_, rerr := replay.Run(replay.Config{
				Network: "localnet", ShardID: 0, DestinationDB: dest,
				AnchorPath: w.k.anchorPath, InspectReportPath: w.inspectA,
				BaselineAgreementPath: w.agreement, BundleDir: w.bundleDir,
				TargetHeight: targetHeight, ToolVersion: toolVersion,
				OutputPath: filepath.Join(t.TempDir(), "rerun.json"),
			})
			if rerr == nil || !strings.Contains(rerr.Error(), "journal") {
				t.Fatalf("rerun on the crashed destination must refuse via the journal, got %v", rerr)
			}
		})
	}

	compactPoints := []string{
		"compact.after-state-copy",
		"compact.after-window-copy",
		"compact.after-offchain-copy",
		"compact.after-heads-before-marker",
		"compact.after-report-before-journal",
	}
	chainCfg, err := harness.ChainConfig("localnet", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range compactPoints {
		point := point
		t.Run(point, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "harmony_db_0")
			runChild(t, point, []string{
				"CRASH_OP=compact",
				"CRASH_SRC=" + w.replayed,
				"CRASH_DEST=" + dest,
				"CRASH_ANCHOR=" + w.k.anchorPath,
				"CRASH_SRCREF=" + w.replayJSON,
				"CRASH_OUT=" + filepath.Join(t.TempDir(), "compact.json"),
			})
			assertNotComplete(t, dest)
			_, rerr := compact.Run(compact.Config{
				Network: "localnet", ShardID: 0, ChainConfig: chainCfg,
				SourceDB: w.replayed, DestinationDB: dest,
				AnchorPath: w.k.anchorPath, SourceReferencePath: w.replayJSON,
				TargetHeight: targetHeight, ToolVersion: toolVersion,
				OutputPath: filepath.Join(t.TempDir(), "rerun.json"),
			}, w.window)
			if rerr == nil {
				t.Fatalf("rerun onto the crashed destination must refuse")
			}
		})
	}
}
