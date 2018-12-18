package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	beaconchain "github.com/harmony-one/harmony/beaconchain/libs"
	"github.com/harmony-one/harmony/log"
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

	flag.Parse()

	if *versionFlag {
		printVersion(os.Args[0])
	}

	h := log.StdoutHandler
	log.Root().SetHandler(h)

	bc := beaconchain.New(*numShards, *ip, *port)
	go bc.SupportRPC()

	bc.StartServer()
}
