package main

import (
	"flag"

	"github.com/harmony-one/harmony/beaconchain"
)

func main() {
	numShards := flag.Int("numShards", 2, "number of shards of identity chain")
	ip := flag.String("ip", "127.0.0.1", "ip on which beaconchain listens")
	port := flag.String("port", "8081", "port on which beaconchain listens")
	flag.Parse()
	bc := beaconchain.New(*numShards, *ip, *port)
	bc.StartServer()
}
