package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/replay"
)

func replayCmd() *cobra.Command {
	var (
		destination      string
		inspectReport    string
		agreement        string
		bundleDir        string
		bundleComparison string
		anchorPath       string
		targetHeight     uint64
		offline          bool
		noResume         bool
		minFreeBytes     uint64
		output           string
	)
	cmd := &cobra.Command{
		Use:   "replay-bundle",
		Short: "Strict offline replay of a verified bundle into the working copy up to the pinned target (plan WS4)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !offline || !noResume {
				return usageErr("--offline and --no-resume-on-unclean-exit are mandatory acknowledgements")
			}
			if destination == "" || inspectReport == "" || agreement == "" || bundleDir == "" || anchorPath == "" || output == "" {
				return usageErr("--destination-db, --inspect-report, --baseline-agreement, --bundle, --anchor-manifest and --output are mandatory")
			}
			if targetHeight == 0 {
				return usageErr("--target-height is mandatory")
			}
			if err := requireAbsPaths("destination-db", destination, "inspect-report", inspectReport,
				"baseline-agreement", agreement, "bundle", bundleDir,
				"bundle-comparison", bundleComparison, "anchor-manifest", anchorPath,
				"output", output); err != nil {
				return err
			}
			rep, err := replay.Run(replay.Config{
				Network: flagNetwork, ShardID: flagShard,
				DestinationDB:         destination,
				AnchorPath:            anchorPath,
				InspectReportPath:     inspectReport,
				BaselineAgreementPath: agreement,
				BundleDir:             bundleDir,
				BundleComparisonPath:  bundleComparison,
				TargetHeight:          targetHeight,
				MinFreeBytes:          minFreeBytes,
				ToolVersion:           toolVersion(),
				OutputPath:            output,
			})
			if err != nil {
				return verificationErr(err)
			}
			fmt.Printf("replay-bundle: %d blocks replayed to target; gate passed; replay.json written to %s\n",
				rep.BlocksReplayed, output)
			return nil
		},
	}
	cmd.Flags().StringVar(&destination, "destination-db", "", "absolute path to the working copy harmony_db_0 (consumed as the replay destination)")
	cmd.Flags().StringVar(&inspectReport, "inspect-report", "", "fresh inspect report of this destination")
	cmd.Flags().StringVar(&agreement, "baseline-agreement", "", "two-copy agreement verdict naming the inspect report")
	cmd.Flags().StringVar(&bundleDir, "bundle", "", "verified bundle directory")
	cmd.Flags().StringVar(&bundleComparison, "bundle-comparison", "", "optional compare-bundles report (single-donor mode)")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "anchor manifest (pinned incident values)")
	cmd.Flags().Uint64Var(&targetHeight, "target-height", 0, "pinned target height")
	cmd.Flags().BoolVar(&offline, "offline", false, "acknowledge fully offline operation (mandatory)")
	cmd.Flags().BoolVar(&noResume, "no-resume-on-unclean-exit", false, "acknowledge v1 no-resume (mandatory)")
	cmd.Flags().Uint64Var(&minFreeBytes, "min-free-bytes", 0, "continuous free-space reserve on the destination filesystem")
	cmd.Flags().StringVar(&output, "output", "", "replay.json output path")
	return cmd
}
