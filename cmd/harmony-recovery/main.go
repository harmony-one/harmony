// harmony-recovery is the emergency shard-0 recovery toolbox. Its single
// subcommand, preflight, is a validator-friendly eligibility sampler: run it
// against the node's shard-0 LevelDB - WITHOUT stopping the node - and it
// verifies the pinned target block header/certificate/ancestry plus the
// complete target state, prints PASS or FAIL, and writes one small JSON
// receipt to paste into Telegram.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
)

// Stamped by scripts/go_executable_build.sh ldflags (same pattern as
// cmd/harmony). Informational only; no gating or refusal paths.
var (
	version string
	commit  string
	builtAt string
	builtBy string
)

func buildInfo() report.Build {
	b := report.Build{GitDescribe: commit}
	if version != "" {
		b.GitDescribe = fmt.Sprintf("%s (build %s)", commit, version)
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		b.GoVersion = info.GoVersion
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				b.VCSRevision = s.Value
			case "vcs.modified":
				b.VCSModified = s.Value == "true"
			}
		}
	}
	return b
}

func versionString() string {
	b := buildInfo()
	return fmt.Sprintf("harmony-recovery version %s commit %s vcs %s (modified=%v) built %s by %s go %s",
		orUnknown(version), orUnknown(commit), orUnknown(b.VCSRevision), b.VCSModified,
		orUnknown(builtAt), orUnknown(builtBy), orUnknown(b.GoVersion))
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "harmony-recovery",
		Short: "Emergency shard-0 recovery tools (preflight eligibility sampler)",
		Long: "harmony-recovery hosts the emergency shard-0 in-place recovery tooling.\n" +
			"The preflight subcommand samples a validator database for recovery\n" +
			"eligibility without stopping the node and never writes to the database.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Version = versionString()
	root.AddCommand(newPreflightCommand())
	return root
}

// exitCodeError carries a pipeline exit code through cobra.
type exitCodeError int

func (e exitCodeError) Error() string { return fmt.Sprintf("exit code %d", int(e)) }

// run executes the CLI and returns the process exit code (testable without
// process boundaries).
func run(args []string, stdout, stderr io.Writer) int {
	root := newRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		var code exitCodeError
		if errors.As(err, &code) {
			return int(code)
		}
		fmt.Fprintln(stderr, "error:", err)
		return report.ExitUnusable
	}
	return report.ExitPass
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
