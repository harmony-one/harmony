package main

import "github.com/harmony-one/harmony/internal/cli"

type p2pConfig struct {
	Port    int
	KeyFile string
}

var (
	p2pPortFlag = &cli.IntFlag{
		Name:       "p2p.port",
		Usage:      "port to listen for p2p communication",
		DefValue:   nodeconfig.,
	}
)
