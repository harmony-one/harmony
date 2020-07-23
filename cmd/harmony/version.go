package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	versionFormat = "Harmony (C) 2020. %v, version %v-%v (%v %v)"
)

// Version string variables
var (
	version string
	builtBy string
	builtAt string
	commit  string
)

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
