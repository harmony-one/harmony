package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/harmony-one/harmony/crypto/bls"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide # of keys to be generated")
		os.Exit(1)
	}

	if n, err := strconv.Atoi(os.Args[1]); err == nil {
		for i := 0; i < n; i++ {
			// randomBytes := [32]byte{}
			// _, err := io.ReadFull(rand.Reader, randomBytes[:])

			privateKey := bls.RandPrivateKey()
			publickKey := privateKey.GetPublicKey()
			fmt.Printf("{Private: \"%s\", Public: \"%s\"},\n", privateKey.GetHexString(), publickKey.GetHexString())
		}
	} else {
		fmt.Println("Unable to parse # as the argument.")
	}
}
