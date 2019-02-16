package core

import (
	"strings"
	"testing"
)

func TestEncodeGenesisConfig(t *testing.T) {
	fileName := "genesis_block_test.json"
	s := EncodeGenesisConfig(fileName)
	genesisAcc := decodePrealloc(s)

	for k := range genesisAcc {
		key := strings.ToLower(k.Hex())
		if key != "0xb7a2c103728b7305b5ae6e961c94ee99c9fe8e2b" && key != "0xb498bb0f520005b6216a4425b75aa9adc52d622b" {
			t.Errorf("EncodeGenesisConfig incorrect")
		}
	}
}
