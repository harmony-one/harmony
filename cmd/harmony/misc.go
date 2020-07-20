package main

import (
	"github.com/harmony-one/harmony/internal/cli"
	"github.com/spf13/cobra"
)

// miscFlags are legacy flags that have multiple usage.
var miscFlags = []cli.Flag{
	legacyPortFlag,
}

var (
	legacyPortFlag = cli.IntFlag{
		Name:       "port",
		Usage:      "port of the node",
		DefValue:   defaultConfig.P2P.Port,
		Deprecated: "Use --p2p.port, --http.port instead",
	}
)

// TODO: move all port manipulation +500 -3000 logic here
func applyMiscFlags(cmd *cobra.Command, config *hmyConfig) {
	fs := cmd.Flags()

	if fs.Changed(legacyPortFlag.Name) {
		legacyPort := cli.GetIntFlagValue(cmd, legacyPortFlag)
		config.P2P.Port = legacyPort
		config.RPC.Port = legacyPort
	}
}
