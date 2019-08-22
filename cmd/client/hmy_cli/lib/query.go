package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdTimes)
}

var cmdTimes = &cobra.Command{
	Use:   "account",
	Short: "Query account balance",
	Long:  `Query account balances`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(strings.Join(args, " "))
	},
}
