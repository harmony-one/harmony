package main

import (
	"fmt"
	"log"
	"time"

	"github.com/harmony-one/harmony/internal/genesis"

	"github.com/harmony-one/bls/ffi/go/bls"
)

func init() {
	bls.Init(bls.BLS12_381)
}

func main() {
	m := "message to sign"
	var aggSig *bls.Sign
	var aggPub *bls.PublicKey

	startTime := time.Now()
	for i := 0; i < len(genesis.NewNodeAccounts); i++ {
		var sec bls.SecretKey
		sec.SetByCSPRNG()
		err := sec.DeserializeHexStr(sec.SerializeToHexStr())
		if err != nil {
			fmt.Println(err)
		}
		if i%10 == 0 {
			fmt.Println()
			fmt.Printf("// %d - %d\n", i, i+9)
		}
		fmt.Printf("{Address: \"%s\", BLSPriKey: \"%s\"},\n", genesis.NewNodeAccounts[i].Address, sec.SerializeToHexStr())
		if i == 0 {
			aggSig = sec.Sign(m)
			aggPub = sec.GetPublicKey()
		} else {
			aggSig.Add(sec.Sign(m))
			aggPub.Add(sec.GetPublicKey())
		}
	}
	endTime := time.Now()
	log.Printf("Time required to sign 1000 messages and aggregate 1000 pub keys and signatures: %f seconds", endTime.Sub(startTime).Seconds())
	log.Printf("Aggregate Signature: 0x%s, length: %d", aggSig.SerializeToHexStr(), len(aggSig.Serialize()))
	log.Printf("Aggregate Public Key: 0x%s, length: %d", aggPub.SerializeToHexStr(), len(aggPub.Serialize()))

	startTime = time.Now()
	if !aggSig.Verify(aggPub, m) {
		log.Fatal("Aggregate Signature Does Not Verify")
	}
	log.Printf("Aggregate Signature Verifies Correctly!")
	endTime = time.Now()
	log.Printf("Time required to verify aggregate sig: %f seconds", endTime.Sub(startTime).Seconds())
}
