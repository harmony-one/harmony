// harmony-recovery-db is the internal producer pipeline for the shard-0
// target-sync and clean-DB fallback tooling (plan §2.2.5 naming: the
// harmony-recovery binary name is reserved for the in-place maintenance
// tool). It never initializes networking, RPC, txpool, consensus services,
// or BLS signing keys (in-place handoff §4 safety contract).
//
// Exit codes: 0 ok, 2 usage, 3 precondition, 4 verification-failed,
// 5 io/corruption.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/utils"
)

// Stamped via -ldflags "-X main.version=... -X main.commit=...".
var (
	version = "dev"
	commit  = "unknown"
)

// Exit codes (plan WS1).
const (
	ExitOK           = 0
	ExitUsage        = 2
	ExitPrecondition = 3
	ExitVerification = 4
	ExitIO           = 5
)

// exitError carries an explicit exit code through the command layer.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func usageErr(format string, args ...interface{}) error {
	return &exitError{code: ExitUsage, err: fmt.Errorf(format, args...)}
}

func preconditionErr(err error) error {
	return &exitError{code: ExitPrecondition, err: err}
}

func verificationErr(err error) error {
	return &exitError{code: ExitVerification, err: err}
}

func ioErr(err error) error {
	return &exitError{code: ExitIO, err: err}
}

// classify maps bare errors to exit codes: corruption/lock/layout problems
// are IO-class, everything else defaults to precondition.
func classify(err error) int {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	switch {
	case errors.Is(err, dbopen.ErrCorrupted),
		errors.Is(err, dbopen.ErrLocked),
		errors.Is(err, dbopen.ErrShardedLayout):
		return ExitIO
	default:
		return ExitPrecondition
	}
}

// Global flags (plan §4: --network and --shard mandatory, v1 refuses
// --shard != 0; absolute paths only, enforced per-path).
var (
	flagNetwork string
	flagShard   uint32
	flagLogFile string
)

// requireGlobals enforces the documented CLI contract (round 13 finding 12):
// --network and --shard must both be EXPLICITLY provided — the zero default
// of --shard does not satisfy the contract.
func requireGlobals(cmd *cobra.Command) error {
	if flagNetwork == "" {
		return usageErr("--network is mandatory")
	}
	if !cmd.Flags().Changed("shard") {
		return usageErr("--shard is mandatory (must be given explicitly; v1 accepts only 0)")
	}
	if flagShard != 0 {
		return usageErr("v1 refuses --shard != 0 (got %d)", flagShard)
	}
	return nil
}

// requireAbsPaths enforces the absolute-path contract on every provided
// (non-empty) path flag value, name->value pairs (round 13 finding 12).
func requireAbsPaths(pairs ...string) error {
	for i := 0; i+1 < len(pairs); i += 2 {
		name, val := pairs[i], pairs[i+1]
		if val == "" {
			continue
		}
		if !filepath.IsAbs(val) {
			return usageErr("--%s must be an absolute path (got %q)", name, val)
		}
	}
	return nil
}

func toolVersion() string {
	return fmt.Sprintf("harmony-recovery-db/%s+%s", version, commit)
}

func main() {
	root := &cobra.Command{
		Use:           "harmony-recovery-db",
		Short:         "Internal producer pipeline: replay a shard-0 DB offline to a pinned target and export a clean compact harmony_db_0",
		Version:       toolVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// --log-file is wired, not decorative (round 13 finding 12).
			if flagLogFile != "" {
				if !filepath.IsAbs(flagLogFile) {
					return usageErr("--log-file must be an absolute path (got %q)", flagLogFile)
				}
				utils.AddLogFile(flagLogFile, 100 /*MB*/, 3, 30 /*days*/)
			}
			return nil
		},
	}
	root.PersistentFlags().StringVar(&flagNetwork, "network", "", "network schedule/config (mainnet|testnet|localnet|partner|stressnet|pangaea) — mandatory")
	root.PersistentFlags().Uint32Var(&flagShard, "shard", 0, "shard ID — v1 supports only 0, mandatory")
	root.PersistentFlags().StringVar(&flagLogFile, "log-file", "", "optional log file (stderr otherwise)")

	root.AddCommand(
		inspectCmd(),
		inventoryCmd(),
		exportCmd(),
		compareCmd(),
		replayCmd(),
		compactCmd(),
		verifyCmd(),
		packageCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "harmony-recovery-db: error: %v\n", err)
		os.Exit(classify(err))
	}
}
