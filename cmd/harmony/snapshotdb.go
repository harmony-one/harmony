package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/trie"
	"github.com/spf13/cobra"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state/pruner"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/harmony-one/harmony/internal/utils"
)

var bloomFilterSizeFlag = cli.IntFlag{
	Name:      "bloomfilter.size",
	Shorthand: "b",
	Usage:     "Megabytes of memory allocated to bloom-filter for pruning",
	DefValue:  2048,
}

var stateRootFlag = cli.StringFlag{
	Name:      "stateroot",
	Shorthand: "r",
	Usage:     "state root hash",
	DefValue:  "",
}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "A set of commands based on the snapshot",
	Long:  "A set of commands based on the snapshot",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Error: must also specify a subcommand (prune-state, verify, ...)")
	},
}

var pruneStateCmd = &cobra.Command{
	Use:     "prune-state srcdb cachedir",
	Short:   "prune stale harmony state data based on snapshot",
	Long:    "will prune historical state data with the help of state snapshot. All trie nodes that do not belong to the specified version state will be deleted from the database",
	Example: "harmony prune-state /srcDir/harmony_db_0 /prune_cache",
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		srcDBDir, cachedir := args[0], args[1]
		bloomFilterSize := cli.GetIntFlagValue(cmd, bloomFilterSizeFlag)
		stateRoot := cli.GetStringFlagValue(cmd, stateRootFlag)

		chaindb, err := rawdb.NewLevelDBDatabase(srcDBDir, LEVELDB_CACHE_SIZE, LEVELDB_HANDLES, "", false)
		if err != nil {
			fmt.Println("open src db error:", err)
			os.Exit(-1)
		}
		defer chaindb.Close()

		prunerconfig := pruner.Config{
			Datadir:   ResolvePath(""),
			Cachedir:  ResolvePath(cachedir),
			BloomSize: uint64(bloomFilterSize),
		}
		pruner, err := pruner.NewPruner(chaindb, prunerconfig)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to open snapshot tree")
			return
		}

		var targetRoot common.Hash
		if len(stateRoot) >= 3 {
			targetRoot, err = parseRoot(stateRoot)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to resolve state root")
				return
			}
		} else {
			targetRoot = rawdb.ReadHeadBlockHash(chaindb)
		}

		if err = pruner.Prune(targetRoot); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to prune state")
			return
		}

		return
	},
}

var verifyStateCmd = &cobra.Command{
	Use:     "verify-state srcdb",
	Short:   "Recalculate state hash based on snapshot for verification",
	Long:    "Recalculate state hash based on snapshot for verification",
	Example: "harmony verify-state /srcDir/harmony_db_0",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 1 {
			fmt.Println("too many arguments")
			return
		}
		srcDBDir := args[0]
		chaindb, err := rawdb.NewLevelDBDatabase(srcDBDir, LEVELDB_CACHE_SIZE, LEVELDB_HANDLES, "", false)
		if err != nil {
			fmt.Println("open src db error:", err)
			os.Exit(-1)
		}
		defer chaindb.Close()

		headRoot := rawdb.ReadHeadBlockHash(chaindb)
		stateRoot := cli.GetStringFlagValue(cmd, stateRootFlag)
		var targetRoot common.Hash
		if len(stateRoot) >= 3 {
			var err error
			if targetRoot, err = parseRoot(stateRoot); err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to resolve state root")
				return
			}
		} else {
			targetRoot = headRoot
		}
		snapconfig := snapshot.Config{
			CacheSize:  256,
			Recovery:   false,
			NoBuild:    true,
			AsyncBuild: false,
		}
		snaptree, err := snapshot.New(snapconfig, chaindb, trie.NewDatabase(chaindb), headRoot)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to open snapshot tree")
			return
		}
		if err := snaptree.Verify(targetRoot); err != nil {
			utils.Logger().Error().Err(err).Interface("root", targetRoot).Msg("Failed to verify state")
			return
		}
		utils.Logger().Info().Interface("root", targetRoot).Msg("Verified the state")
		if err := snapshot.CheckDanglingStorage(chaindb); err != nil {
			utils.Logger().Error().Err(err).Interface("root", targetRoot).Msg("Failed to check dangling storage")
		}
		return
	},
}

func ResolvePath(filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(filepath.Dir("."), filename)
}

func parseRoot(input string) (common.Hash, error) {
	var h common.Hash
	if err := h.UnmarshalText([]byte(input)); err != nil {
		return h, err
	}
	return h, nil
}

func registerSnapshotCmdFlags() error {
	if err := cli.RegisterFlags(pruneStateCmd, []cli.Flag{bloomFilterSizeFlag, stateRootFlag}); err != nil {
		return err
	}
	if err := cli.RegisterFlags(verifyStateCmd, []cli.Flag{stateRootFlag}); err != nil {
		return err
	}
	return nil
}
