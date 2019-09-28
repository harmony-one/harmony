package bech32_test

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/harmony-one/harmony/internal/bech32"
)

func TestEncodeAndDecode(t *testing.T) {

	sum := sha256.Sum256([]byte("hello world123\n"))

	bech, err := bech32.ConvertAndEncode("shasum", sum[:])

	if err != nil {
		t.Error(err)
	}
	hrp, data, err := bech32.DecodeAndConvert(bech)

	if err != nil {
		t.Error(err)
	}
	if hrp != "shasum" {
		t.Error("Invalid hrp")
	}
	if !bytes.Equal(data, sum[:]) {
		t.Error("Invalid decode")
	}
}
