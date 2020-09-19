package main

import (
	"fmt"
	"github.com/harmony-one/harmony/crypto/bls"
	"math/big"

	common2 "github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/common"

	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	validatorAddress = common2.Address(common.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))

	testBLSPubKey    = "30b2c38b1316da91e068ac3bd8751c0901ef6c02a1d58bc712104918302c6ed03d5894671d0c816dad2b4d303320f202"
	testBLSPrvKey    = "c6d7603520311f7a4e6aac0b26701fc433b75b38df504cd416ef2b900cd66205"
	postStakingEpoch = big.NewInt(200)
)

func init() {
	bls_core.Init(bls_core.BLS12_381)
}

func generateBLSKeySigPair() (bls.SerializedPublicKey, bls.SerializedSignature) {
	p := &bls_core.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey)
	pub := bls.SerializedPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BLSVerificationStr)
	privateKey := &bls_core.SecretKey{}
	privateKey.DeserializeHexStr(testBLSPrvKey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig bls.SerializedSignature
	copy(sig[:], signature.Serialize())
	return pub, sig
}

func main() {
	pri1 := bls.RandPrivateKey()
	pri2 := bls.RandPrivateKey()
	pri3 := bls.RandPrivateKey()
	pri4 := bls.RandPrivateKey()

	pub1 := pri1.GetPublicKey()
	pub2 := pri2.GetPublicKey()
	pub3 := pri3.GetPublicKey()
	pub4 := pri4.GetPublicKey()

	w1 := bls.PublicKeyWrapper{Object: pub1}
	w1.Bytes.FromLibBLSPublicKey(w1.Object)

	w2 := bls.PublicKeyWrapper{Object: pub2}
	w2.Bytes.FromLibBLSPublicKey(w2.Object)

	w3 := bls.PublicKeyWrapper{Object: pub3}
	w3.Bytes.FromLibBLSPublicKey(w3.Object)

	w4 := bls.PublicKeyWrapper{Object: pub4}
	w4.Bytes.FromLibBLSPublicKey(w4.Object)

	wrappers := []bls.PublicKeyWrapper{w1, w2, w3, w4}

	bls.NewMask(wrappers, nil)

	messageBytes := []byte(staking.BLSVerificationStr)
	msgHash := hash.Keccak256(messageBytes)

	s1 := pri1.SignHash(msgHash)
	//s2 := pri2.SignHash(msgHash)
	//s3 := pri3.SignHash(msgHash)
	//s4 := pri4.SignHash(msgHash)

	var aggregatedSig bls_core.Sign
	aggregatedSig.Add(s1)

	aggPub := &bls_core.PublicKey{}

	aggPub.Add(pub1)

	fmt.Println(aggregatedSig.VerifyHash(pub1, msgHash))
}
