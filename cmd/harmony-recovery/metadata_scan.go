package main

import (
	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/metadata/scan"
)

func newMetadataScanCommand() *cobra.Command {
	opts := scan.Options{}
	cmd := &cobra.Command{
		Use:   "scan --db /path/to/harmony_db_0 --anchor recovery-anchor.json --report scan-report.json",
		Short: "Diagnose and dry-run the metadata normalization (read-only; run before export and audit)",
		Long: "scan strict-opens the STOPPED node's database read-only, cross-verifies the\n" +
			"anchor config against the schedule and the DB, resolves the target state, runs\n" +
			"the shared normalization once and writes the full report: per-section counts,\n" +
			"deletion plan, digests, absence assertions, coverage, stats/sync-era/shard-1\n" +
			"inventories, and zero-write proof. The normalized validator-list length is\n" +
			"printed prominently for the manual preflight comparison.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signalContext()
			defer stop()
			return metadataExit(scan.Run(ctx, opts, cmd.ErrOrStderr()))
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.DBPath, "db", "", "absolute path to the node's harmony_db_0 directory (required; node must be stopped)")
	f.StringVar(&opts.AnchorPath, "anchor", "", "path to recovery-anchor.json (required)")
	f.StringVar(&opts.ReportPath, "report", "metadata-scan-report.json", "report output path (never inside --db)")
	f.IntVar(&opts.Handles, "handles", 0, "open-file cache capacity for the LevelDB reader (0 = default)")
	f.IntVar(&opts.CacheMB, "db-cache-mb", 0, "LevelDB block cache size in MiB (0 = default)")
	_ = cmd.MarkFlagRequired("db")
	_ = cmd.MarkFlagRequired("anchor")
	return cmd
}
