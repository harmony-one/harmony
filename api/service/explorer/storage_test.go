package explorer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test for GetAddressKey
func TestGetAddressKey(t *testing.T) {
	assert.Equal(t, GetAddressKey("abcd"), "ad_abcd", "error")
}

// TestInit ..
func TestInit(t *testing.T) {
	ins := GetStorageInstance("1.1.1.1", "3333", true)
	if err := ins.GetDB().Put([]byte{1}, []byte{2}, nil); err != nil {
		t.Fatal("(*LDBDatabase).Put failed:", err)
	}
	value, err := ins.GetDB().Get([]byte{1}, nil)
	assert.Equal(t, bytes.Compare(value, []byte{2}), 0, "value should be []byte{2}")
	assert.Nil(t, err, "error should be nil")
}
