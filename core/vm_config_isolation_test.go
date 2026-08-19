package core

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestGetVMConfigIsIsolated checks that the VM config handed to a caller is
// theirs to adjust. Callers set fields such as the tracer on it for their own
// execution, which must not reach the configuration blocks are processed with.
func TestGetVMConfigIsIsolated(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, _, _ := getTestEnvironment(*key)

	before := chain.GetVMConfig()
	if before.Debug {
		t.Fatal("expected debug to start off")
	}

	// A caller adjusting its own copy.
	before.Debug = true
	before.Tracer = nil

	after := chain.GetVMConfig()
	if after.Debug {
		t.Error("adjusting the returned config changed the chain's own config")
	}
}
