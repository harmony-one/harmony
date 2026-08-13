package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/bundle"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

func compareCmd() *cobra.Command {
	var (
		left   string
		right  string
		output string
	)
	cmd := &cobra.Command{
		Use:   "compare-bundles",
		Short: "Optional byte-comparator for two bundles (plan WS3; not on the single-donor critical path)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireGlobals(cmd); err != nil {
				return err
			}
			if left == "" || right == "" || output == "" {
				return usageErr("--left, --right and --output are mandatory")
			}
			if err := requireAbsPaths("left", left, "right", right, "output", output); err != nil {
				return err
			}
			res, err := bundle.Compare(left, right, flagNetwork, flagShard, toolVersion())
			if err != nil {
				return ioErr(err)
			}
			if _, err := report.WriteJSON(output, res); err != nil {
				return ioErr(err)
			}
			fmt.Printf("compare-bundles: %d records compared, identical=%v, donor-sig differences=%d (informational)\n",
				res.RecordsCompared, res.Identical, res.DonorSigDifferences)
			if !res.Identical {
				return verificationErr(fmt.Errorf("bundles differ: %s (chain differences are fatal)", res.FirstDifference))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&left, "left", "", "left bundle directory")
	cmd.Flags().StringVar(&right, "right", "", "right bundle directory")
	cmd.Flags().StringVar(&output, "output", "", "comparison report output path")
	return cmd
}
