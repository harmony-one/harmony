package waitnode

import (
	"testing"

	"github.com/simple-rules/harmony-benchmark/log"
)

func TestNewNode(test *testing.T) {
	addressNode := address{IP: "1", Port: "2"}
	Worker := "pow"
	ID := 1
	Log := log.New()
	node := New(addressNode, ID)
	if node.Address == nil {
		test.Error("Address is not initialized for the node")
	}

	if node.ID == nil {
		test.Error("ID is not initialized for the node")
	}

	if node.Worker == nil {
		test.Error("Worker is not initialized for the node")
	}
}
