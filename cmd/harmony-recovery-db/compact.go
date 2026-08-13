package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/compact"
	"github.com/harmony-one/harmony/internal/recoverydb/harness"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

func compactCmd() *cobra.Command {
	var (
		sourceDB        string
		sourceReadOnly  bool
		destinationDB   string
		anchorPath      string
		sourceReference string
		metaRefManifest string
		targetHeight    uint64
		retainFrom      uint64
		batchBytes      int
		failIfNonEmpty  bool
		sizeLimitBytes  uint64
		withStats       bool
		withPreimages   string
		output          string
	)
	cmd := &cobra.Command{
		Use:   "compact-db",
		Short: "Strict target-state compactor into a fresh validator harmony_db_0 (plan WS5)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !sourceReadOnly {
				return usageErr("--source-read-only is mandatory")
			}
			if !failIfNonEmpty {
				return usageErr("--fail-if-destination-nonempty is mandatory (v1 never resumes)")
			}
			if sourceDB == "" || destinationDB == "" || anchorPath == "" || sourceReference == "" || output == "" {
				return usageErr("--source-db, --destination-db, --anchor-manifest, --source-reference and --output are mandatory")
			}
			if targetHeight == 0 {
				return usageErr("--target-height is mandatory")
			}
			if err := requireAbsPaths("source-db", sourceDB, "destination-db", destinationDB,
				"anchor-manifest", anchorPath, "source-reference", sourceReference,
				"metadata-reference-manifest", metaRefManifest,
				"with-preimages", withPreimages, "output", output); err != nil {
				return err
			}
			sched, err := harness.Schedule(flagNetwork)
			if err != nil {
				return usageErr("%v", err)
			}
			window, err := anchor.ComputeWindow(sched, targetHeight, retainFrom)
			if err != nil {
				return preconditionErr(err)
			}
			if _, err := harness.InitSchedule(flagNetwork); err != nil {
				return usageErr("%v", err)
			}
			chainConfig, err := harness.ChainConfig(flagNetwork, flagShard)
			if err != nil {
				return usageErr("%v", err)
			}
			rep, err := compact.Run(compact.Config{
				Network: flagNetwork, ShardID: flagShard, ChainConfig: chainConfig,
				SourceDB: sourceDB, DestinationDB: destinationDB,
				AnchorPath: anchorPath, SourceReferencePath: sourceReference,
				MetadataReferenceManifestPath: metaRefManifest,
				TargetHeight:                  targetHeight,
				RetainFromOverride:            retainFrom,
				BatchBytes:                    batchBytes,
				SizeLimitBytes:                sizeLimitBytes,
				WithValidatorStats:            withStats,
				WithPreimages:                 withPreimages,
				ToolVersion:                   toolVersion(),
				OutputPath:                    output,
			}, window)
			if err != nil {
				return verificationErr(err)
			}
			fmt.Printf("compact-db: %s built (%d bytes, %d files); mode=%s; journal=%s; compact.json %s\n",
				destinationDB, rep.DestinationBytes, rep.DestinationFiles, rep.Mode, rep.JournalState, output)
			if rep.JournalState == report.StateCompleteUnreleasable {
				return verificationErr(fmt.Errorf("build finished COMPLETE_UNRELEASABLE (size gate: %d bytes > limit %d); preserved for diagnosis, refused by package-db",
					rep.SizeGate.ActualBytes, rep.SizeGate.LimitBytes))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceDB, "source-db", "", "absolute path to the replayed working copy (opened strictly read-only)")
	cmd.Flags().BoolVar(&sourceReadOnly, "source-read-only", false, "acknowledge read-only source open (mandatory)")
	cmd.Flags().StringVar(&destinationDB, "destination-db", "", "absolute path of the fresh output harmony_db_0")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "anchor manifest")
	cmd.Flags().StringVar(&sourceReference, "source-reference", "", "the replay gate report (replay.json)")
	cmd.Flags().StringVar(&metaRefManifest, "metadata-reference-manifest", "", "optional in-place reference manifest (reference mode); internal:none sentinel when absent")
	cmd.Flags().Uint64Var(&targetHeight, "target-height", 0, "pinned target height")
	cmd.Flags().Uint64Var(&retainFrom, "retain-from-height", 0, "optional retention extension (may only lower the window start)")
	cmd.Flags().IntVar(&batchBytes, "batch-bytes", compact.DefaultBatchBytes, "write-batch flush threshold")
	cmd.Flags().BoolVar(&failIfNonEmpty, "fail-if-destination-nonempty", false, "acknowledge fresh-destination contract (mandatory)")
	cmd.Flags().Uint64Var(&sizeLimitBytes, "size-limit-bytes", compact.DefaultSizeLimitBytes, "release size gate (200 GB default)")
	cmd.Flags().BoolVar(&withStats, "with-validator-stats", false, "opt in to copying validator stats (omitted by default per in-place §2.2)")
	cmd.Flags().StringVar(&withPreimages, "with-preimages", "", "optional consumer-list JSON declaring a preimage subset to copy")
	cmd.Flags().StringVar(&output, "output", "", "compact.json output path")
	return cmd
}
