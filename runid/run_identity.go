package main

import (
	"fmt"

	"github.com/simple-rules/harmony-benchmark/beaconchain"
)

func main() {
	bc := beaconchain.New("temp")
	fmt.Print(bc)
}
