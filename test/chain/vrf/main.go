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

	size := 250

	sig := &bls_core.Sign{}
	startTime := time.Now()
	for i := 0; i < size; i++ {
		sig = blsPriKey.SignHash(blockHash[:])
	}
	endTime := time.Now()
	fmt.Printf("Time required to sign %d times: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	for i := 0; i < size; i++ {
		if !sig.VerifyHash(pubKeyWrapper.Object, blockHash[:]) {
			fmt.Println("failed to verify sig")
		}
	}
	endTime = time.Now()
	fmt.Printf("Time required to verify sig %d times: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	sk := vrf_bls.NewVRFSigner(blsPriKey)

	vrf := [32]byte{}
	proof := []byte{}

	startTime = time.Now()
	for i := 0; i < size; i++ {
		vrf, proof = sk.Evaluate(blockHash[:])
	}
	endTime = time.Now()
	fmt.Printf("Time required to generate vrf %d times: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	pk := vrf_bls.NewVRFVerifier(blsPriKey.GetPublicKey())

	resultVrf := [32]byte{}
	startTime = time.Now()
	for i := 0; i < size; i++ {
		resultVrf, _ = pk.ProofToHash(blockHash, proof)
	}
	endTime = time.Now()
	fmt.Printf("Time required to verify vrf %d times: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	if bytes.Compare(vrf[:], resultVrf[:]) != 0 {
		fmt.Printf("Failed to verify VRF")

	}

	allPubs := []bls.PublicKeyWrapper{}
	allSigs := []*bls_core.Sign{}
	aggPub := &bls_core.PublicKey{}
	aggSig := &bls_core.Sign{}
	for i := 0; i < size; i++ {
		blsPriKey := bls.RandPrivateKey()
		pubKeyWrapper := bls.PublicKeyWrapper{Object: blsPriKey.GetPublicKey()}
		pubKeyWrapper.Bytes.FromLibBLSPublicKey(pubKeyWrapper.Object)
		allPubs = append(allPubs, pubKeyWrapper)

		sig := blsPriKey.SignHash(blockHash[:])
		allSigs = append(allSigs, sig)
	}

	startTime = time.Now()
	for i := 0; i < size; i++ {
		aggPub.Add(allPubs[i].Object)
	}
	endTime = time.Now()
	fmt.Printf("Time required to aggregate %d pubKeys: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	for i := 0; i < size; i++ {
		aggSig.Add(allSigs[i])
	}
	endTime = time.Now()
	fmt.Printf("Time required to aggregate %d sigs: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	startTime = time.Now()
	if !aggSig.VerifyHash(aggPub, blockHash[:]) {
		fmt.Println("failed to verify sig")
	}

	endTime = time.Now()
	fmt.Printf("Time required to verify a %d aggregated sig: %f seconds\n", size, endTime.Sub(startTime).Seconds())

	// A example result of a single run:
	//Time required to sign 1000 times: 0.542673 seconds
	//Time required to verify sig 1000 times: 1.499797 seconds
	//Time required to generate vrf 1000 times: 0.525362 seconds
	//Time required to verify vrf 1000 times: 2.076890 seconds
}
