package blockchain

import (
	"testing"
)

func TestVerifyTransactions(t *testing.T) {
	if cbtx := NewCoinbaseTX("minh", genesisCoinbaseData); cbtx == nil {
		t.Errorf("failed to create a coinbase transaction.")
	}
}
