package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/inspect"
)

func inspectCmd() *cobra.Command {
	var (
		dbPath           string
		readOnly         bool
		fullState        bool
		fullOffchain     bool
		requirePreimages bool
		targetHeight     uint64
		anchorPath       string
		output           string
		compareWith      string
		agreementOutput  string
	)
	cmd := &cobra.Command{
		Use:   "inspect-db",
		Short: "Baseline tuple pinning, full state/off-chain digest passes, full-archival replay preflight, two-copy agreement (plan WS2)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if !readOnly {
				return usageErr("--read-only is mandatory for inspect-db (sources are never opened writable)")
			}
			if dbPath == "" || output == "" {
				return usageErr("--db and --output are mandatory")
			}
			if requirePreimages && !fullState {
				return usageErr("--require-preimages needs --full-state-check")
			}
			if err := requireAbsPaths("db", dbPath, "output", output,
				"anchor-manifest", anchorPath, "compare-with", compareWith,
				"agreement-output", agreementOutput); err != nil {
				return err
			}
			rep, sum, err := inspect.Run(inspect.Params{
				Network: flagNetwork, ShardID: flagShard,
				DBPath: dbPath, FullState: fullState, FullOffchain: fullOffchain,
				RequirePreimages: requirePreimages, TargetHeight: targetHeight,
				AnchorPath: anchorPath, Output: output, ToolVersion: toolVersion(),
			})
			if err != nil {
				return err
			}
			fmt.Printf("inspect-db: report written to %s (sha256 %s)\n", output, sum)

			if compareWith != "" {
				out := agreementOutput
				if out == "" {
					out = output + ".agreement.json"
				}
				verdict, err := inspect.Agreement(flagNetwork, flagShard, toolVersion(), output, compareWith, out)
				if err != nil {
					return preconditionErr(err)
				}
				fmt.Printf("inspect-db: agreement verdict written to %s (agreed=%v)\n", out, verdict.Agreed)
				if !verdict.Agreed {
					return verificationErr(fmt.Errorf("two-copy agreement failed: %v", verdict.Differences))
				}
			}
			if inspect.Failed(rep) {
				return verificationErr(fmt.Errorf("inspect-db: one or more checks failed (see %s)", output))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "absolute path to the source harmony_db_0 (opened strictly read-only)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "acknowledge read-only source open (mandatory)")
	cmd.Flags().BoolVar(&fullState, "full-state-check", false, "full account/storage/code traversal from the head root")
	cmd.Flags().BoolVar(&fullOffchain, "full-offchain-check", false, "strict-iterator digest passes over the off-chain namespaces")
	cmd.Flags().BoolVar(&requirePreimages, "require-preimages", false, "any missing preimage is fatal (mandatory for full-archival sources)")
	cmd.Flags().Uint64Var(&targetHeight, "target-height", 0, "pinned target height (enables replay preflight + baseline gate)")
	cmd.Flags().StringVar(&anchorPath, "anchor-manifest", "", "optional anchor manifest (known-bad lists for the baseline gate)")
	cmd.Flags().StringVar(&output, "output", "", "report output path (report.json)")
	cmd.Flags().StringVar(&compareWith, "compare-with", "", "other copy's inspect report for the two-copy agreement")
	cmd.Flags().StringVar(&agreementOutput, "agreement-output", "", "agreement verdict output (default <output>.agreement.json)")
	return cmd
}
