package main

import (
	"fmt"
	"os"

	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

const (
	versionFormat = "Harmony (C) 2023. %v, version %v-%v (%v %v)"
)

// Version string variables
var (
	version string
	builtBy string
	builtAt string
	commit  string
)

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
		printVersion()
		os.Exit(0)
	},
}

func getHarmonyVersion() string {
	return fmt.Sprintf(versionFormat, "harmony", version, commit, builtBy, builtAt)
}

func printVersion() {
	fmt.Println(getHarmonyVersion())
}
