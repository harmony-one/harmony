package waitnode

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/log"
)

func TestNewNode(test *testing.T) {
	Address := address{IP: "1", Port: "2"}
	Worker := "pow"
	ID := 1
	Log := log.new()
	node := New(address, ID)
	if node.Address == nil {
		test.Error("Address is not initialized for the node")
	}

	if node.ID == nil {
		test.Error("ID is not initialized for the node")
	}

	if node.Wroker == nil {
		test.Error("Worker is not initialized for the node")
	}
}
