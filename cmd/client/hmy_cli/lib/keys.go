package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	cmdKeys := &cobra.Command{
		Use:   "keys",
		Short: "Add or view local private keys",
		Long: `
Manage your local keys
`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Print: " + strings.Join(args, " "))
		},
	}

	cmdMnemonic := &cobra.Command{
		Use:   "mnemonic",
		Short: "Compute the bip39 mnemonic for some input entropy",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}
	cmdAdd := &cobra.Command{
		Use:   "add",
		Short: "Create a new key, or import from seed",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}
	cmdList := &cobra.Command{
		Use:   "list",
		Short: "List all keys",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}
	cmdShow := &cobra.Command{
		Use:   "show",
		Short: "Show key info for the given name",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}
	cmdDelete := &cobra.Command{
		Use:   "delete",
		Short: "Delete the given key",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}
	cmdUpdate := &cobra.Command{
		Use:   "update",
		Short: "Change the password used to protect private key",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cmd)
		},
	}

	cmdKeys.AddCommand(cmdMnemonic, cmdAdd, cmdList, cmdShow, cmdDelete, cmdUpdate)
	rootCmd.AddCommand(cmdKeys)
}
