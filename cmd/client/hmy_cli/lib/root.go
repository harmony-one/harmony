package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hmy_cli",
	Short: "Harmony blockchain",
	Long: `
CLI interface to the Harmony blockchain
`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var (
	Verbose bool
	version string
	builtBy string
	builtAt string
	commit  string
)

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "version", "v", false, "verbose output")

	cmdKeys := &cobra.Command{
		Use:   "keys",
		Short: "Add or view local private keys",
		Long: `
Manage your local keys
`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// u := url.URL{Scheme: "ws", Host: "localhost:9090", Path: "/"}
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	cmdVersion := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(os.Stderr, "Harmony (C) 2019 version %v-%v (%v %v)\n", version, commit, builtBy, builtAt)
			os.Exit(0)
		},
	}

	rootCmd.AddCommand(cmdKeys, cmdVersion)
}

func Execute(version, builtBy, builtAt, commit string) {
	version = version
	builtBy = builtBy
	builtAt = builtAt
	commit = commit
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
