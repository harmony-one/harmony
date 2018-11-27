package main

import (
	"fmt"

	"github.com/harmony-one/harmony/beaconchain"
)

// this is a comment

func main() {
	bc := beaconchain.New("temp")
	bc.StartServer()
	fmt.Println("Hello World")
}
