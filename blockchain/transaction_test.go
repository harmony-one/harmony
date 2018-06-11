package blockchain

import (
	"testing"
)

func TestNewCoinbaseTX(t *testing.T) {
	NewCoinbaseTX("minh", genesisCoinbaseData)
}
