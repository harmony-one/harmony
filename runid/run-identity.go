package main

import (
	"github.com/harmony-one/harmony/beaconchain"
)

func main() {

	bc := beaconchain.New("temp")
	bc.StartServer()

}
