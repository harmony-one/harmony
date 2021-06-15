package explorer

import (
	"bytes"
	"testing"
)

// Test for GetAddressKey
func TestGetAddressKey(t *testing.T) {
	key := GetAddressKey("abcd")
	exp := []byte("ad_abcd")
	if !bytes.Equal(key, exp) {
		t.Errorf("unexpected key: %v / %v", key, exp)
	}
}
