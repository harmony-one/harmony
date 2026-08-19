package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestCLIContract is the table-driven flag-combination test: it enumerates
// required/forbidden flag combinations for every subcommand, including both
// reference-manifest modes (plan §4 canonical CLI contract). It exercises the
// argument-validation layer only (RunE returns a usage/precondition error
// before touching any database), so it needs no fixtures.
func TestCLIContract(t *testing.T) {
	build := func() *cobra.Command {
		root := &cobra.Command{Use: "harmony-recovery-db", SilenceUsage: true, SilenceErrors: true}
		root.PersistentFlags().StringVar(&flagNetwork, "network", "", "")
		root.PersistentFlags().Uint32Var(&flagShard, "shard", 0, "")
		root.PersistentFlags().StringVar(&flagLogFile, "log-file", "", "")
		root.AddCommand(inspectCmd(), inventoryCmd(), exportCmd(), compareCmd(),
			replayCmd(), compactCmd(), verifyCmd(), packageCmd())
		return root
	}

	cases := []struct {
		name    string
		args    []string
		wantErr string // substring; "" = must NOT fail at the validation layer
	}{
		// Global mandatory flags (round 13 finding 12: --shard must be given
		// EXPLICITLY; the zero default does not satisfy the contract).
		{"missingNetwork", []string{"inspect-db", "--read-only", "--db", "/x", "--output", "/y"}, "network is mandatory"},
		{"missingShard", []string{"inspect-db", "--network", "mainnet", "--read-only", "--db", "/x", "--output", "/y"}, "--shard is mandatory"},
		{"shardNonZero", []string{"inspect-db", "--network", "mainnet", "--shard", "1", "--read-only", "--db", "/x", "--output", "/y"}, "shard != 0"},

		// Absolute-path contract on every provided path flag (round 13
		// finding 12).
		{"inspectRelativeDB", []string{"inspect-db", "--network", "localnet", "--shard", "0", "--read-only", "--db", "rel/db", "--output", "/y"}, "absolute path"},
		{"replayRelativeBundle", []string{"replay-bundle", "--network", "localnet", "--shard", "0", "--offline", "--no-resume-on-unclean-exit", "--destination-db", "/d", "--inspect-report", "/i", "--baseline-agreement", "/a", "--bundle", "rel/b", "--anchor-manifest", "/m", "--target-height", "5", "--output", "/o"}, "absolute path"},
		{"packageRelativeRoot", []string{"package-db", "--network", "localnet", "--shard", "0", "--db", "/d", "--anchor-manifest", "/m", "--verification-report", "/v", "--release-root", "rel/root", "--target-height", "5", "--output", "/o"}, "absolute path"},

		// inspect-db: --read-only mandatory; --require-preimages needs --full-state-check.
		{"inspectNoReadonly", []string{"inspect-db", "--network", "localnet", "--shard", "0", "--db", "/x", "--output", "/y"}, "--read-only is mandatory"},
		{"inspectRequirePreimagesNeedsState", []string{"inspect-db", "--network", "localnet", "--shard", "0", "--read-only", "--db", "/x", "--output", "/y", "--require-preimages"}, "--full-state-check"},

		// export-bundle: --read-only mandatory; the three height flags mandatory.
		{"exportNoReadonly", []string{"export-bundle", "--network", "localnet", "--shard", "0", "--source-db", "/x", "--baseline-manifest", "/b", "--output", "/o", "--from-height", "2", "--to-height", "3", "--certificate-child-height", "4"}, "--read-only is mandatory"},
		{"exportMissingHeights", []string{"export-bundle", "--network", "localnet", "--shard", "0", "--read-only", "--source-db", "/x", "--baseline-manifest", "/b", "--output", "/o"}, "mandatory"},

		// replay-bundle: --offline + --no-resume mandatory; core inputs mandatory.
		{"replayMissingAck", []string{"replay-bundle", "--network", "localnet", "--shard", "0", "--destination-db", "/d", "--inspect-report", "/i", "--baseline-agreement", "/a", "--bundle", "/b", "--anchor-manifest", "/m", "--target-height", "5", "--output", "/o"}, "--offline"},
		{"replayMissingInputs", []string{"replay-bundle", "--network", "localnet", "--shard", "0", "--offline", "--no-resume-on-unclean-exit", "--target-height", "5"}, "mandatory"},

		// compact-db: --source-read-only + --fail-if-destination-nonempty mandatory.
		{"compactMissingReadonly", []string{"compact-db", "--network", "localnet", "--shard", "0", "--source-db", "/s", "--destination-db", "/d", "--anchor-manifest", "/m", "--source-reference", "/r", "--target-height", "5", "--output", "/o", "--fail-if-destination-nonempty"}, "--source-read-only is mandatory"},
		{"compactMissingFailIfNonEmpty", []string{"compact-db", "--network", "localnet", "--shard", "0", "--source-read-only", "--source-db", "/s", "--destination-db", "/d", "--anchor-manifest", "/m", "--source-reference", "/r", "--target-height", "5", "--output", "/o"}, "--fail-if-destination-nonempty is mandatory"},

		// verify-db: --read-only + both check flags mandatory.
		{"verifyMissingReadonly", []string{"verify-db", "--network", "localnet", "--shard", "0", "--db", "/d", "--anchor-manifest", "/m", "--source-reference", "/r", "--output", "/o", "--full-state-check", "--full-offchain-check"}, "--read-only is mandatory"},
		{"verifyMissingCheckFlags", []string{"verify-db", "--network", "localnet", "--shard", "0", "--read-only", "--db", "/d", "--anchor-manifest", "/m", "--source-reference", "/r", "--output", "/o"}, "--full-state-check and --full-offchain-check are mandatory"},

		// package-db: core inputs mandatory.
		{"packageMissingInputs", []string{"package-db", "--network", "localnet", "--shard", "0", "--db", "/d"}, "mandatory"},

		// compare-bundles: left/right/output mandatory.
		{"compareMissing", []string{"compare-bundles", "--network", "localnet", "--shard", "0", "--left", "/l"}, "mandatory"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flagNetwork, flagShard, flagLogFile = "", 0, ""
			root := build()
			root.SetArgs(c.args)
			err := root.Execute()
			if c.wantErr == "" {
				if err != nil && isValidationErr(err) {
					t.Fatalf("valid combination rejected at the validation layer: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErr)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), c.wantErr)
			}
		})
	}
}

// isValidationErr reports whether err is a usage-layer rejection (as opposed
// to a downstream IO/precondition error from actually touching a fake path).
func isValidationErr(err error) bool {
	var ee *exitError
	if !asExit(err, &ee) {
		return false
	}
	return ee.code == ExitUsage
}

func asExit(err error, target **exitError) bool {
	for err != nil {
		if e, ok := err.(*exitError); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
