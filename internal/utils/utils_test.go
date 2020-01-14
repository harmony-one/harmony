package utils

import (
	"net"
	"os"
	"testing"

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

	pk1, _, err := LoadPrivateKey(str)
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

	key1, _, err := LoadKeyFromFile(filename)
	if err != nil {
		t.Fatalf("failed to load key from file (%s): %v", filename, err)
	}

	if !crypto.KeyEqual(key, key1) {
		t.Fatalf("loaded key is not equal to the saved one")
	}

	key2, _, err := LoadKeyFromFile(nonexist)

	if err != nil {
		t.Fatalf("failed to load key from non-exist file: %v", err)
	}

	if crypto.KeyEqual(key1, key2) {
		t.Fatalf("new random key can't equal to existing one, something is wrong!")
	}

	os.Remove(filename)
	os.Remove(nonexist)
}

func TestIsPrivateIP(t *testing.T) {
	addr := []struct {
		ip        net.IP
		isPrivate bool
	}{
		{
			net.IPv4(127, 0, 0, 1),
			true,
		},
		{
			net.IPv4(172, 31, 82, 23),
			true,
		},
		{
			net.IPv4(192, 168, 82, 23),
			true,
		},
		{
			net.IPv4(54, 172, 99, 189),
			false,
		},
		{
			net.IPv4(10, 1, 0, 1),
			true,
		},
	}

	for _, a := range addr {
		r := IsPrivateIP(a.ip)
		if r != a.isPrivate {
			t.Errorf("IP: %v, IsPrivate: %v, Expected: %v", a.ip, r, a.isPrivate)
		}
	}
}
