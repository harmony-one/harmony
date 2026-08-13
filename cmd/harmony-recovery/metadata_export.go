package main

import (
	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/metadata/refexport"
)

func newMetadataExportCommand() *cobra.Command {
	opts := refexport.Options{}
	cmd := &cobra.Command{
		Use:   "export-reference --db /path/to/harmony_db_0 --anchor recovery-anchor.json --out-dir DIR",
		Short: "Produce the canonical .hmr reference package and reference manifest (run-once producer)",
		Long: "export-reference derives the target-height validator metadata and, only on\n" +
			"a fully clean run, atomically publishes <out-dir>/release/ holding\n" +
			"metadata-<target>.hmr, the canonical metadata-<target>.reference.json\n" +
			"(whose SHA-256 is the published reference digest), the internal run\n" +
			"checksums and the success export report — one atomic unit that appears\n" +
			"complete or not at all. On a failed or refused attempt a failure report\n" +
			"is written to <out-dir> instead. Any fatal or missing-required finding\n" +
			"refuses export. The built-in double-run determinism self-check derives\n" +
			"everything twice and byte-compares both serializations before writing\n" +
			"anything; on mismatch it exits 23 and emits only determinism-diff dumps.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signalContext()
			defer stop()
			return metadataExit(refexport.Run(ctx, opts, cmd.ErrOrStderr()))
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.DBPath, "db", "", "absolute path to the node's harmony_db_0 directory (required; node must be stopped)")
	f.StringVar(&opts.AnchorPath, "anchor", "", "path to recovery-anchor.json (required)")
	f.StringVar(&opts.OutDir, "out-dir", "", "output directory for artifacts and reports (required; never inside --db)")
	f.IntVar(&opts.Handles, "handles", 0, "open-file cache capacity for the LevelDB reader (0 = default)")
	f.IntVar(&opts.CacheMB, "db-cache-mb", 0, "LevelDB block cache size in MiB (0 = default)")
	_ = cmd.MarkFlagRequired("db")
	_ = cmd.MarkFlagRequired("anchor")
	_ = cmd.MarkFlagRequired("out-dir")
	return cmd
}
