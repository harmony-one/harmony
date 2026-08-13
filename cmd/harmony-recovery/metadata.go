package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/report"
)

// newMetadataCommand builds the `metadata` command group (plan §4.1): the
// offline validator-metadata maintenance commands extending the
// preflight-owned harmony-recovery root. No root-global flags; every flag
// is group-local. There is no --network flag: network, shard and the
// frozen constants come from the JSON anchor config (--anchor), which each
// subcommand resolves via the preflight-owned inplace/anchor.Resolve
// (installing the process-global schedule) and cross-checks against the
// compiled constants and the source DB.
func newMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Offline validator-metadata maintenance (scan, export-reference, audit-branch; stopped node required)",
		Long: "The metadata command family derives the recovery target's validator\n" +
			"metadata from a STOPPED node's shard-0 LevelDB: scan (diagnosis/dry-run),\n" +
			"export-reference (the run-once canonical .hmr + reference JSON producer)\n" +
			"and audit-branch (masked re-execution of the abandoned branch).\n\n" +
			"Exit codes: 0 OK; 13 unsafe open/concurrent writer; 14 I/O or corruption;\n" +
			"15 invalid config/paths; 16 interrupted; 20 MISSING_REQUIRED_METADATA;\n" +
			"21 INVALID_RETAINED_METADATA; 22 TARGET_STATE_UNAVAILABLE;\n" +
			"23 DETERMINISM_MISMATCH; 24 AUDIT_ANOMALY; 130 SIGINT.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newMetadataScanCommand())
	cmd.AddCommand(newMetadataExportCommand())
	cmd.AddCommand(newMetadataAuditCommand())
	return cmd
}

// signalContext returns a context canceled with cause report.ErrSIGINT on
// SIGINT delivery; the deferred stop restores default signal behavior.
// report.InterruptExit distinguishes SIGINT (130) from any other
// cancellation (16) via the recorded cause (§4.5 table).
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		select {
		case <-ch:
			cancel(report.ErrSIGINT)
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(ch)
		cancel(context.Canceled)
	}
}

// metadataExit converts a §4.5 code into the root's exit delivery
// (exitCodeError for nonzero, nil for success — main.go:83-103).
func metadataExit(code int) error {
	if code == report.ExitOK {
		return nil
	}
	return exitCodeError(code)
}
