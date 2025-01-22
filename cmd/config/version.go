package config

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

var VersionMetaData []interface{}

var versionFlag = cli.BoolFlag{
	Name:      "version",
	Shorthand: "V",
	Usage:     "display version info",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version of the harmony binary",
	Long:  "print version of the harmony binary",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		PrintVersion()
		os.Exit(0)
	},
}

func VersionFlag() cli.BoolFlag {
	return versionFlag
}

func GetHarmonyVersion() string {
	var version, commit, builtBy, builtAt, commitAt string
	version = VersionMetaData[1].(string)
	commit = VersionMetaData[2].(string)
	commitAt = VersionMetaData[3].(string)
	builtBy = VersionMetaData[4].(string)
	builtAt = VersionMetaData[5].(string)

	//Harmony (C) 2025. harmony, version v8507-v2024.3.1-85-g735e68a31-dirty (soph@ 2025-01-21T14:07:44+0700)
	commitYear, err := time.Parse("2006-01-02T15:04:05-0700", commitAt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing commit date: %v\n", err)
		os.Exit(1)
	}
	var currentYear = commitYear.Year()
	versionFormat := fmt.Sprintf("Harmony (C) %d. %%v, version %%v-%%v (%%v %%v)", currentYear)
	return fmt.Sprintf(versionFormat, path.Base(os.Args[0]), version, commit, builtBy, builtAt)
}

func PrintVersion() {
	fmt.Println(GetHarmonyVersion())
}
