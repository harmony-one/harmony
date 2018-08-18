package waitnode

import (
	"testing"
)

func TestNewNode(test *testing.T) {
	addressNode := &address{IP: "1", Port: "2"}
	ID := 1
	node := New(addressNode, ID)
	if node.Address == nil {
		test.Error("Address is not initialized for the node")
	}
	if node.ID != 1 {
		test.Error("ID is not initialized for the node")
	}
	if node.Worker != "pow" {
		test.Error("Worker is not initialized for the node")
	}
}
