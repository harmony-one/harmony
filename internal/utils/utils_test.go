package utils

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/harmony-one/harmony/p2p"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/stretchr/testify/assert"
)

// Tests for TestConvertFixedDataIntoByteArray.
func TestConvertFixedDataIntoByteArray(t *testing.T) {
	res := ConvertFixedDataIntoByteArray(int16(3))
	if len(res) != 2 {
		t.Errorf("Conversion incorrect.")
	}
	res = ConvertFixedDataIntoByteArray(int32(3))
	if len(res) != 4 {
		t.Errorf("Conversion incorrect.")
	}
}

// Tests for TestAllocateShard.
func TestAllocateShard(t *testing.T) {
	num, success := AllocateShard(1, 1)
	assert.Equal(t, num, 1, "error")
	assert.True(t, success, "error")

	num, success = AllocateShard(2, 1)
	assert.False(t, success, "error")
	assert.Equal(t, num, 1, "error")

	num, success = AllocateShard(1, 2)
	assert.True(t, success, "error")
	assert.Equal(t, num, 1, "error")

	num, success = AllocateShard(5, 3)
	assert.False(t, success, "error")
	assert.Equal(t, num, 2, "error")

	num, success = AllocateShard(6, 3)
	assert.False(t, success, "error")
	assert.Equal(t, num, 3, "error")
}

// Test for GenKey
func TestGenKey(t *testing.T) {
	priKey, pubKey := GenKey("3.3.3.3", "3456")
	if priKey == nil || pubKey == nil {
		t.Error("Failed to create keys for BLS sig")
	}
	pubKeyBytes, _ := hex.DecodeString("ca6247713431a59cbadfe282b36cb13746b6e5c5db6e5972a10a83adffdf23f8aab246229cb3050e061e1024aa9b6518e200dd9663a8c855e596f1007150aa0672e6f40d073947aa027e8ffe8e89d894ca3916f80fdb350f4b8643f6ff99510c")

	if bytes.Compare(pubKey.Serialize(), pubKeyBytes) != 0 {
		t.Errorf("Unexpected public key: %s", hex.EncodeToString(pubKey.Serialize()))
	}
}

// Test for GenKeyP2P, noted the length of private key can be random
// thus we don't test it here.
func TestGenKeyP2P(t *testing.T) {
	_, pb, err := GenKeyP2P("127.0.0.1", "8888")
	if err != nil {
		t.Errorf("GenKeyP2p Error: %v", err)
	}
	kpb, _ := crypto.MarshalPublicKey(pb)
	if len(kpb) != 299 {
		t.Errorf("Length of Public Key Error: %v, expected 299", len(kpb))
	}
}

// Test for GenKeyP2PRand, noted the length of private key can be random
// thus we don't test it here.
func TestGenKeyP2PRand(t *testing.T) {
	_, pb, err := GenKeyP2PRand()
	if err != nil {
		t.Errorf("GenKeyP2PRand Error: %v", err)
	}
	kpb, _ := crypto.MarshalPublicKey(pb)
	if len(kpb) != 299 {
		t.Errorf("Length of Public Key Error: %v, expected 299", len(kpb))
	}
}

// Test for GetUniqueIDFromPeer
func TestGetUniqueIDFromPeer(t *testing.T) {
	peer := p2p.Peer{IP: "1.1.1.1", Port: "123"}
	assert.Equal(t, GetUniqueIDFromPeer(peer), uint32(1111123), "should be equal to 1111123")
}

// Test for GetUniqueIDFromIPPort
func TestGetUniqueIDFromIPPort(t *testing.T) {
	assert.Equal(t, GetUniqueIDFromIPPort("1.1.1.1", "123"), uint32(1111123), "should be equal to 1111123")
}

// Test for SavePrivateKey/LoadPrivateKey functions
func TestSaveLoadPrivateKey(t *testing.T) {
	pk, _, err := GenKeyP2P("127.0.0.1", "8888")
	if err != nil {
		t.Fatalf("failed to generate p2p key: %v", err)
	}
	str, err := SavePrivateKey(pk)
	if err != nil {
		t.Fatalf("failed to save private key: %v", err)
	}

	pk1, err := LoadPrivateKey(str)
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}

	if !crypto.KeyEqual(pk, pk1) {
		t.Errorf("loaded key is not right")
		b1, _ := pk.Bytes()
		b2, _ := pk1.Bytes()
		t.Errorf("expecting pk: %v\n", b1)
		t.Errorf("got pk1: %v\n", b2)
	}
}

func TestSaveLoadKeyFile(t *testing.T) {
	filename := "/tmp/keystore"
	nonexist := "/tmp/please_ignore_the_non-exist_file"

	key, _, err := GenKeyP2PRand()
	if err != nil {
		t.Fatalf("failed to generate random p2p key: %v", err)
	}

	err = SaveKeyToFile(filename, key)
	if err != nil {
		t.Fatalf("failed to save key to file: %v", err)
	}

	key1, err := LoadKeyFromFile(filename)
	if err != nil {
		t.Fatalf("failed to load key from file (%s): %v", filename, err)
	}

	if !crypto.KeyEqual(key, key1) {
		t.Fatalf("loaded key is not equal to the saved one")
	}

	key2, err := LoadKeyFromFile(nonexist)

	if err != nil {
		t.Fatalf("failed to load key from non-exist file: %v", err)
	}

	if crypto.KeyEqual(key1, key2) {
		t.Fatalf("new random key can't equal to existing one, something is wrong!")
	}

	os.Remove(filename)
	os.Remove(nonexist)
}
