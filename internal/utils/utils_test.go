package utils

import (
	"bytes"
	"encoding/hex"
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

// Test for GenKeyBLS
func TestGenKeyBLS(t *testing.T) {
	priKey, pubKey := GenKeyBLS("3.3.3.3", "3456")
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

var (
	savedKey = "CAASqQkwggSlAgEAAoIBAQCwKJVWOIbfLXNKxK8VetvHS7eEOtC8VJ/GoyeWsaaGVknLyDGYkvWYpeYpu5hsKYfq6yPPBOjKqx0Oz0uKxclys7RcM/cLiGr9x86XaHH80YYAN7MDLyzS9QVlCFjkWoDh2xFUWgTmLgpGaLmi4vO1GTAdbtymO8CI499E2+hz2O4E1xOARum+vH+Y+kXML6ap2982KOEHXMwsCjLyymdL+/hsu3VzmTpxIfpSGnlJHoacwlMuTWIyj1J9Z5pMRLoPOwK6xFCgY5p4/CC20VcCJQal6BtEh4tQciwxFZlZ4eUsVc3JIw096NcJ0wx6Jz5qGXJjOy2bJaKHuLSUmqxXAgMBAAECggEAcOzTLr5110OvkNKc2kwz74JeVmnNva0R76hPjI69jYhrLjNbd89dmUlgTohvoYbOFo4+GkuvX5xpuECy0HcSOHFywViemcoNrDoV+YF+8O7v09vg6b2oImPn3WiIc3qA/EgOx+AdG+GPvKsNtZl/WSyYZ4XV9MqBFj/dtKq0TO5Ga8Pg0H/NTYu4clQuqu4pw5dHoXXfhXfrLGZpsXPhAlOV96mNKIiRK3c7Lj4so6g29W3XFgOgw1oYuzpMlv95airHnKiwMWty3wwQ+orL+A7WiSi1Kt2d9kZQ5RTJwchwc241fv+7+aL0lfzzCxmoDo17967aaPeSGASzUFAL4QKBgQDG2LJzw7VR89UbfFH4Yaw7J6Ll0bppDD+TwBtA8jtRTykL9kOpS3aTt2nNDv9YYGKlqIF+186MuQ6dOIA7J67hXWRH78nF95XhhlpWrphAa5udNHgF6QCGe617MJDVFUgXNkcJPzvSHxuBz8h5Z7mP5ArA9g/DkBMBVnY8h5jYtQKBgQDiynyiiYzaeLugqzeRDy3QdH5fJjIQXwCNfCHmCQ5NmHj7XB9jVf8nAWzekuwqrANaCLj3N8rELOC9uAcuSh7ymB9/4mZ251M3jjF5Q+RTF1xN4EubE2mFdAz3ITGiUt7d5w0tgpVjq5e+zyCs6Hn+6GWk5B8wqwC4cp0YcymUWwKBgQDDwG4lAsRMclMX5NI5R8Yq0gFOZ6Iwaetow5TQ4eY9TEWnTf8b+Xs5PjV8tkfvs6tJU9JvkXn4FPHrGsU59v31RGBFZSzoo6y8QOxMK0MdIBIot490mgV3XufQv2XFL1cx6rARzVtRpmgI6gl8Yv1NRvzDKzknl3zuMzTgr8hrhQKBgQDNlqWZanvnaN8d7Vh4BXyQpaoRczybHqQPnmHUeI0gxoGVy5Mgp8qff2lD84hnvntjWNjkMw16/PvWwEayLbsUS9byRTiBvX3wtNQgi+0lbd3dMuEW+WgE9Ij0VoD6F4m1O0j04pWuPtVWwclrNWuyKtZJvgqQQdRrYGsMyQj+VQKBgQCC15mFp1nh3/KfB07JU3w7pcqalvq5ZrkZDK9NrgwXNoXMWxgQ/2BfoX2rETM2HbfCIN5rLSt2paElkxIn8fF2NV/e4Hdk3WMfd0OFhMZ4hJCscx2DukMSW2qJ4sp97RudplsK9eW0y8OIhY1UF8eME9bOdWD0bU10QiMNwVzOWQ=="
)

// Test for SavePrivateKey function
func TestSavePrivateKey(t *testing.T) {
	pk, _, err := GenKeyP2P("127.0.0.1", "8888")
	if err != nil {
		t.Fatalf("failed to generate p2p key: %v", err)
	}
	str, err := SavePrivateKey(&pk)
	if err != nil {
		t.Fatalf("failed to save private key: %v", err)
	}
	if str != savedKey {
		t.Errorf("key is not right")
		t.Errorf("got: %s", str)
		t.Errorf("expecting: %s", savedKey)
	}
}

// Test for LoadPrivateKey function
func TestLoadPrivateKey(t *testing.T) {
	pk, _, err := GenKeyP2P("127.0.0.1", "8888")
	if err != nil {
		t.Fatalf("failed to generate p2p key: %v", err)
	}
	pk1, err := LoadPrivateKey(savedKey)
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}
	if !crypto.KeyEqual(pk, *pk1) {
		t.Errorf("loaded key is not right")
		b1, _ := pk.Bytes()
		b2, _ := (*pk1).Bytes()
		t.Errorf("expecting pk: %v\n", b1)
		t.Errorf("got pk1: %v\n", b2)
	}
}
