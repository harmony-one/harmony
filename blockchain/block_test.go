package blockchain

import (
	"reflect"
	"testing"
)

func TestBlockSerialize(t *testing.T) {
	cbtx := NewCoinbaseTX(TestAddressOne, genesisCoinbaseData, 0)
	if cbtx == nil {
		t.Errorf("Failed to create a coinbase transaction.")
	}
	block := NewGenesisBlock(cbtx, 0)

	serializedValue := block.Serialize()
	deserializedBlock, _ := DeserializeBlock(serializedValue)

	if !reflect.DeepEqual(block, deserializedBlock) {
		t.Errorf("Original block and the deserialized block not equal.")
	}
}
