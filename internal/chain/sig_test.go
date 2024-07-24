package chain

import (
	"fmt"
	"testing"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
)

func TestDecodeSigBitmap(t *testing.T) {

	aggSig := bls_core.Sign{}
	rs := aggSig.Serialize()
	var sigBytes bls.SerializedSignature
	copy(sigBytes[:], rs[:])
	// gen pub keys
	pub := bls.WrapperFromPrivateKey(bls.RandPrivateKey())

	pubKeys := []bls.PublicKeyWrapper{*pub.Pub}

	_, _, err := DecodeSigBitmap(sigBytes, nil, pubKeys)
	fmt.Println(err.Error())
	//require.NoError(t, err)
}
