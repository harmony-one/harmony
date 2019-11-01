package main

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	common2 "github.com/harmony-one/harmony/internal/common"
)

func main() {
	// Create an account
	key, _ := crypto.GenerateKey()

	// Get the address
	address := crypto.PubkeyToAddress(key.PublicKey)
	// 0x8ee3333cDE801ceE9471ADf23370c48b011f82a6

	// Get the private key
	privateKey := hex.EncodeToString(key.D.Bytes())
	// 05b14254a1d0c77a49eae3bdf080f926a2df17d8e2ebdf7af941ea001481e57f

	fmt.Printf("account: %s\n", common2.MustAddressToBech32(address))
	fmt.Printf("private Key : %s\n", privateKey)
}
