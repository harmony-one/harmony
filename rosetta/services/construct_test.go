package services

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestGetAddressFromPublicKeyBytes(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	refAddr := crypto.PubkeyToAddress(key.PublicKey)
	compressedPublicKey := crypto.CompressPubkey(&key.PublicKey)
	addr, rosettaError := getAddressFromPublicKeyBytes(compressedPublicKey)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(refAddr, *addr) {
		t.Errorf("expected adder %v, got %v", refAddr, *addr)
	}
}
