package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/ethereum/go-ethereum/log"
	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/libs"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func printVersion(me string) {
	fmt.Fprintf(os.Stderr, "Harmony (C) 2018. %v, version %v-%v (%v %v)\n", path.Base(me), version, commit, builtBy, builtAt)
	os.Exit(0)
}

func main() {
	numShards := flag.Int("numShards", 1, "number of shards of identity chain")
	ip := flag.String("ip", "127.0.0.1", "ip on which beaconchain listens")
	port := flag.String("port", "8081", "port on which beaconchain listens")
	versionFlag := flag.Bool("version", false, "Output version info")
	resetFlag := flag.String("path", "bc_config.json", "path to file")
	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	h := log.StreamHandler(os.Stdout, log.TerminalFormat(false))
	log.Root().SetHandler(h)
	var bc *beaconchain.BeaconChain

	if _, err := os.Stat(*resetFlag); err == nil {
		bc, err = beaconchain.LoadBeaconChainInfo(*resetFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not reset beaconchain from file: %+v\n", err)
		}
	} else {
		fmt.Printf("Starting new beaconchain\n")
		beaconchain.SetSaveFile(*resetFlag)
		bc = beaconchain.New(*numShards, *ip, *port)
	}

	fmt.Printf("Beacon Chain Started: /ip4/%s/tcp/%v/ipfs/%s\n", *ip, *port, bc.GetID().Pretty())

	go bc.SupportRPC()
	bc.StartServer()
}
