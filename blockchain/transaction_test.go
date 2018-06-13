package blockchain

import (
	"testing"
)

func TestNewCoinbaseTX(t *testing.T) {
	if NewCoinbaseTX("minh", genesisCoinbaseData) == nil {
		t.Errorf("failed to create a coinbase transaction.")
	}
}
