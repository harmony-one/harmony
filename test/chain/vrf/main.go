package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
)

func init() {
	bls_core.Init(bls_core.BLS12_381)
}

func main() {
	blsPriKey := bls.RandPrivateKey()
	pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
	pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)

	blockHash := hash.Keccak256([]byte{1, 2, 3, 4, 5})

	sig := &bls_core.Sign{}
	startTime := time.Now()
	for i := 0; i < 1000; i++ {
		sig = blsPriKey.SignHash(blockHash[:])
	}
	endTime := time.Now()
	fmt.Printf("Time required to sign 1000 times: %f seconds\n", endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		sig.VerifyHash(pubKeyWrapper.Object, blockHash[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to verify sig 1000 times: %f seconds\n", endTime.Sub(startTime).Seconds())

	sk := vrf_bls.NewVRFSigner(blsPriKey)

	vrf := [32]byte{}
	proof := []byte{}

	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		vrf, proof = sk.Evaluate(blockHash[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to generate vrf 1000 times: %f seconds\n", endTime.Sub(startTime).Seconds())

	pk := vrf_bls.NewVRFVerifier(blsPriKey.GetPublicKey())

	resultVrf := [32]byte{}
	startTime = time.Now()
	for i := 0; i < 1000; i++ {
		resultVrf, _ = pk.ProofToHash(blockHash, proof)
	}
	endTime = time.Now()
	fmt.Printf("Time required to verify vrf 1000 times: %f seconds\n", endTime.Sub(startTime).Seconds())

	if bytes.Compare(vrf[:], resultVrf[:]) != 0 {
		fmt.Printf("Failed to verify VRF")

	}

	// A example result of a single run:
	//Time required to sign 1000 times: 0.542673 seconds
	//Time required to verify sig 1000 times: 1.499797 seconds
	//Time required to generate vrf 1000 times: 0.525362 seconds
	//Time required to verify vrf 1000 times: 2.076890 seconds
}
