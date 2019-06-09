package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"

	crypto2 "github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/internal/common"
)

var (
	version string
	builtBy string
	builtAt string
	commit  string
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please provide # of keys to be generated")
		os.Exit(1)
	}

	if n, err := strconv.Atoi(os.Args[1]); err == nil {
		for i := 0; i < n; i++ {
			randomBytes := [32]byte{}
			_, err := io.ReadFull(rand.Reader, randomBytes[:])
			if err != nil {
				fmt.Println("Failed to get randomness for the private key...")
				return
			}
			priKey, err := crypto2.GenerateKey()
			if err != nil {
				panic("Failed to generate the private key")
			}
			crypto2.FromECDSA(priKey)

			fmt.Printf("{Address: \"%s\", Private: \"%s\", Public: \"%s\"},"+"\n", common.MustAddressToBech32(crypto2.PubkeyToAddress(priKey.PublicKey)), hex.EncodeToString(crypto2.FromECDSA(priKey)), common.MustAddressToBech32(crypto2.PubkeyToAddress(priKey.PublicKey)))
		}
	} else {
		fmt.Println("Unable to parse # as the argument.")
	}
}
