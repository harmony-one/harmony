package services

import (
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestGetAddressFromPublicKey(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	refAddr := crypto.PubkeyToAddress(key.PublicKey)
	compressedPublicKey := crypto.CompressPubkey(&key.PublicKey)
	addr, rosettaError := getAddressFromPublicKey(&types.PublicKey{
		Bytes:     compressedPublicKey,
		CurveType: types.Secp256k1,
	})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(refAddr, *addr) {
		t.Errorf("expected adder %v, got %v", refAddr, *addr)
	}

	_, rosettaError = getAddressFromPublicKey(&types.PublicKey{
		Bytes:     compressedPublicKey,
		CurveType: types.Edwards25519,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	_, rosettaError = getAddressFromPublicKey(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}
