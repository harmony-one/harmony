package config

import (
	"fmt"
	"os"

	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

const (
	versionFormat = "Harmony (C) 2023. %v, version %v-%v (%v %v)"
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
	return fmt.Sprintf(versionFormat, VersionMetaData[:5]...) // "harmony", version, commit, builtBy, builtAt
}

func PrintVersion() {
	fmt.Println(GetHarmonyVersion())
}
