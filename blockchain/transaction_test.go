package blockchain

import (
	"testing"
)

func TestNewCoinbaseTX(t *testing.T) {
	if cbtx := NewCoinbaseTX(TestAddressOne, genesisCoinbaseData, 0); cbtx == nil {
		t.Errorf("failed to create a coinbase transaction.")
	}
}
