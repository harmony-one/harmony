package main

import (
	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recovery/metadata/audit"
)

func newMetadataAuditCommand() *cobra.Command {
	opts := audit.Options{}
	cmd := &cobra.Command{
		Use:   "audit-branch --db /path/to/harmony_db_0 --anchor recovery-anchor.json --out-dir DIR --scratch DIR",
		Short: "Re-execute the abandoned branch over a masked overlay and reconcile every planned deletion",
		Long: "audit-branch executes the abandoned blocks from the target state over a\n" +
			"masked overlay (reads: scratch, then source minus the mask; writes: scratch\n" +
			"only — the source stays strictly read-only), captures native and precompile\n" +
			"staking operations, requires every computed state root to match its header,\n" +
			"byte-verifies the reproduced next-epoch election records against the\n" +
			"to-be-deleted originals, reconciles the deletion plan bidirectionally, and\n" +
			"derives the shard-1 subsets and last-crosslink pointer for B4. Mandatory\n" +
			"two-pass structure: pass 1 discovers the branch-written crosslink/spent\n" +
			"masks; pass 2 is authoritative.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signalContext()
			defer stop()
			return metadataExit(audit.Run(ctx, opts, cmd.ErrOrStderr()))
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.DBPath, "db", "", "absolute path to the source harmony_db_0 (required; node must be stopped)")
	f.StringVar(&opts.AnchorPath, "anchor", "", "path to recovery-anchor.json (required)")
	f.StringVar(&opts.OutDir, "out-dir", "", "output directory for abandoned-branch-audit.json (required; never inside --db)")
	f.StringVar(&opts.Scratch, "scratch", "", "disposable scratch directory for overlay writes (required; never inside --db)")
	f.Uint64Var(&opts.EndHeight, "end-height", 0, "last branch height to execute (0 = anchor audit_end_height)")
	f.BoolVar(&opts.KeepScratch, "keep-scratch", false, "retain the scratch directory for forensics")
	f.BoolVar(&opts.SinglePass, "single-pass", false, "DEBUG ONLY: skip pass 2; output marked non-authoritative")
	f.StringVar(&opts.TrustedShard1Pointer, "trusted-shard1-pointer", "", "pre-incident pointer escape hatch as <shardID>:<blockNum> (validated against the §4.4 invariants)")
	f.StringVar(&opts.TrustedProvenance, "trusted-shard1-pointer-provenance", "", "provenance note recorded with a trusted pointer")
	f.StringVar(&opts.ReferencePath, "reference", "", "optional exported reference manifest (metadata-<target>.reference.json) to cross-check and bind into the report hash chain")
	f.IntVar(&opts.ScratchReserveGB, "scratch-reserve-gb", 200, "minimum free space on the scratch filesystem (must be >= 0; the gate cannot be bypassed)")
	f.IntVar(&opts.Handles, "handles", 0, "open-file cache capacity for the LevelDB reader (0 = default)")
	f.IntVar(&opts.CacheMB, "db-cache-mb", 0, "LevelDB block cache size in MiB (0 = default)")
	_ = cmd.MarkFlagRequired("db")
	_ = cmd.MarkFlagRequired("anchor")
	_ = cmd.MarkFlagRequired("out-dir")
	_ = cmd.MarkFlagRequired("scratch")
	return cmd
}
