package main

import (
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

func registerFlags(cmd *cobra.Command, flags []cli.Flag) error {
	fs := cmd.Flags()

	for _, flag := range flags {
		if err := flag.RegisterTo(fs); err != nil {
			return err
		}
	}
	return nil
}
