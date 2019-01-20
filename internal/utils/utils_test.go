package utils

import (
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
	GenKey("3.3.3.3", "3456")
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
