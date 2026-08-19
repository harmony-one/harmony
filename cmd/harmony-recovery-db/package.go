package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/release"
)

func packageCmd() *cobra.Command {
	var (
		dbPath             string
		anchorPath         string
		targetHeight       uint64
		verificationReport string
		recoveryBinarySHA  string
		provisionalViewID  string
		releaseRoot        string
		output             string
	)
	cmd := &cobra.Command{
		Use:   "package-db",
		Short: "Single-invocation sealer: stage, fully verify, atomically promote, seal with READY (plan WS7)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if dbPath == "" || anchorPath == "" || verificationReport == "" || releaseRoot == "" || output == "" {
				return usageErr("--db, --anchor-manifest, --verification-report, --release-root and --output are mandatory")
			}
			if targetHeight == 0 {
				return usageErr("--target-height is mandatory")
			}
			if err := requireAbsPaths("db", dbPath, "anchor-manifest", anchorPath,
				"verification-report", verificationReport, "release-root", releaseRoot,
				"output", output); err != nil {
				return err
			}
			rep, finalDir, err := release.Run(release.Config{
				Network: flagNetwork, ShardID: flagShard,
				DBPath:                        dbPath,
				AnchorPath:                    anchorPath,
				TargetHeight:                  targetHeight,
				VerificationReportPath:        verificationReport,
				RecoveryHarmonyBinarySHA256:   recoveryBinarySHA,
				ProvisionalMinimumStartViewID: provisionalViewID,
				ReleaseRoot:                   releaseRoot,
				ToolVersion:                   toolVersion(),
				OutputPath:                    output,
			})
			if err != nil {
				return preconditionErr(err)
			}
			// package.json was written durably inside the journaled
			// operation, before the terminal record (round 13 finding 8).
			fmt.Printf("package-db: sealed release %s\n  dir: %s\n  payload: %d bytes in %d files\n  package.json: %s\n\n",
				rep.ReleaseID, rep.ReleaseDir, rep.PayloadBytes, rep.PayloadFiles, output)
			fmt.Print(release.PublishNote(finalDir))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "absolute path to the COMPLETE_VERIFIED compact artifact")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "anchor manifest")
	cmd.Flags().Uint64Var(&targetHeight, "target-height", 0, "pinned target height")
	cmd.Flags().StringVar(&verificationReport, "verification-report", "", "passing verification.json for this artifact")
	cmd.Flags().StringVar(&recoveryBinarySHA, "recovery-harmony-binary-sha256", "", "optional in-place integration field (\"absent\" otherwise)")
	cmd.Flags().StringVar(&provisionalViewID, "provisional-start-view-id", "", "optional in-place integration field (\"absent\" otherwise)")
	cmd.Flags().StringVar(&releaseRoot, "release-root", "", "release tree root (recovery/<network>/shard-<id>/... created beneath)")
	cmd.Flags().StringVar(&output, "output", "", "package.json output path")
	return cmd
}
