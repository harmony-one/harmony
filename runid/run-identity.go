package main

import (
	"github.com/simple-rules/harmony-benchmark/beaconchain"
)

func main() {

	bc := beaconchain.New("temp")
	bc.StartServer()

}
