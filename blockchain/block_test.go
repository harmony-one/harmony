package blockchain

import (
	"bytes"
	"testing"
)

func TestBlockSerialize(t *testing.T) {
	cbtx := NewCoinbaseTX("minh", genesisCoinbaseData)
	if cbtx == nil {
		t.Errorf("Failed to create a coinbase transaction.")
	}
	block := NewGenesisBlock(cbtx)

	serializedValue := block.Serialize()
	deserializedBlock := DeserializeBlock(serializedValue)

	if block.Timestamp != deserializedBlock.Timestamp {
		t.Errorf("Serialize or Deserialize incorrect at TimeStamp.")
	}

	if bytes.Compare(block.PrevBlockHash[:], deserializedBlock.PrevBlockHash[:]) != 0 {
		t.Errorf("Serialize or Deserialize incorrect at PrevBlockHash.")
	}

	if bytes.Compare(block.Hash[:], deserializedBlock.Hash[:]) != 0 {
		t.Errorf("Serialize or Deserialize incorrect at Hash.")
	}
}
