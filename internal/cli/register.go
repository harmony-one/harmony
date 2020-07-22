package cli

import (
	"github.com/spf13/cobra"
)

// RegisterFlags register the flags to command's local flag
func RegisterFlags(cmd *cobra.Command, flags []Flag) error {
	fs := cmd.Flags()

	for _, flag := range flags {
		if err := flag.RegisterTo(fs); err != nil {
			return err
		}
	}
	return nil
}

// RegisterPFlags register the flags to command's persistent flag
func RegisterPFlags(cmd *cobra.Command, flags []Flag) error {
	fs := cmd.PersistentFlags()

	for _, flag := range flags {
		if err := flag.RegisterTo(fs); err != nil {
			return err
		}
	}
	return nil
}
