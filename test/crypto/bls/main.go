package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/bls/ffi/go/bls"
	"log"
	"time"
)

func main() {
	m := "message to sign"
	var aggSig *bls.Sign
	var aggPub *bls.PublicKey

	startTime := time.Now()
	for i := 0; i < 1000; i++ {
		var sec bls.SecretKey
		sec.SetByCSPRNG()

		if i == 0 {
			testECKey, _ := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
			log.Printf("Secret Key: 0x%s", sec.GetHexString())
			log.Printf("Secret Key: 0x%s", hex.EncodeToString(sec.GetLittleEndian()))
			log.Printf("Secret Key Length: %d", len(sec.GetLittleEndian()))
			log.Printf("Secret Key: 0x%s", hex.EncodeToString(testECKey.D.Bytes()))
			log.Printf("Secret Key Length: %d", len(testECKey.D.Bytes()))
		}
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
	log.Printf("Aggregate Signature: 0x%s, length: %d", aggSig.GetHexString(), len(aggSig.Serialize()))
	log.Printf("Aggregate Public Key: 0x%s, length: %d", aggPub.GetHexString(), len(aggPub.Serialize()))

	startTime = time.Now()
	if !aggSig.Verify(aggPub, m) {
		log.Fatal("Aggregate Signature Does Not Verify")
	}
	log.Printf("Aggregate Signature Verifies Correctly!")
	endTime = time.Now()
	log.Printf("Time required to verify aggregate sig: %f seconds", endTime.Sub(startTime).Seconds())
}
