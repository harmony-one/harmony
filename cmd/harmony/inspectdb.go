package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/cli"
)

var prefixFlag = cli.StringFlag{
	Name:      "prefix",
	Shorthand: "p",
	Usage:     "key prefix",
	DefValue:  "",
}

var startKeyFlag = cli.StringFlag{
	Name:      "start_key",
	Shorthand: "s",
	Usage:     "start key",
	DefValue:  "",
}

var inspectDBCmd = &cobra.Command{
	Use:     "inspectdb srcdb prefix startKey",
	Short:   "inspect a db.",
	Long:    "inspect a db.",
	Example: "harmony inspectdb /srcDir/harmony_db_0",
	Args:    cobra.RangeArgs(1, 3),
	Run: func(cmd *cobra.Command, args []string) {
		srcDBDir := args[0]
		prefix := cli.GetStringFlagValue(cmd, prefixFlag)
		startKey := cli.GetStringFlagValue(cmd, startKeyFlag)
		fmt.Println("db path: ", srcDBDir)
		inspectDB(srcDBDir, prefix, startKey)
		os.Exit(0)
	},
}

func registerInspectionFlags() error {
	return cli.RegisterFlags(inspectDBCmd, []cli.Flag{prefixFlag, startKeyFlag})

}

func inspectDB(srcDBDir, prefix, startKey string) {
	fmt.Println("===inspectDB===")
	srcDB, err := rawdb.NewLevelDBDatabase(srcDBDir, LEVELDB_CACHE_SIZE, LEVELDB_HANDLES, "", false)
	if err != nil {
		fmt.Println("open src db error:", err)
		os.Exit(-1)
	}

	rawdb.InspectDatabase(srcDB, []byte(prefix), []byte(startKey))

	fmt.Println("database inspection completed!")
}
