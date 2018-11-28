package main

import (
	"flag"

	"github.com/harmony-one/harmony/beaconchain"
)

// this is a comment

func main() {
	numShards := flag.Int("numShards", 2, "number of shards of identity chain")
	flag.Parse()
	bc := beaconchain.New(*numShards)
	bc.StartServer()
}
