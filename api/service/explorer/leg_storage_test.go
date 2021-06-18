package explorer

import (
	"bytes"
	"testing"
)

// Test for LegGetAddressKey
func TestGetAddressKey(t *testing.T) {
	key := LegGetAddressKey("abcd")
	exp := []byte("ad_abcd")
	if !bytes.Equal(key, exp) {
		t.Errorf("unexpected key: %v / %v", key, exp)
	}
}
