package blockchain

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func TestNewMerkleNode(t *testing.T) {
	data := [][]byte{
		[]byte("node1"),
		[]byte("node2"),
		[]byte("node3"),
	}

	fmt.Println("Testing")
	// Level 1

	n1 := NewMerkleNode(nil, nil, data[0])
	n2 := NewMerkleNode(nil, nil, data[1])
	n3 := NewMerkleNode(nil, nil, data[2])
	n4 := NewMerkleNode(nil, nil, data[2])

	// Level 2
	n5 := NewMerkleNode(n1, n2, nil)
	n6 := NewMerkleNode(n3, n4, nil)

	// Level 3
	n7 := NewMerkleNode(n5, n6, nil)

	if hex.EncodeToString(n7.Data) != "4e3e44e55926330ab6c31892f980f8bfd1a6e910ff1ebc3f778211377f35227e" {
		t.Errorf("merkle tree is not built correctly.")
	}
}

func TestNewMerkleTree(t *testing.T) {
	data := [][]byte{
		[]byte("node1"),
		[]byte("node2"),
		[]byte("node3"),
		[]byte("node4"),
	}
	// Level 1
	n1 := NewMerkleNode(nil, nil, data[0])
	n2 := NewMerkleNode(nil, nil, data[1])
	n3 := NewMerkleNode(nil, nil, data[2])
	n4 := NewMerkleNode(nil, nil, data[3])

	// Level 2
	n5 := NewMerkleNode(n1, n2, nil)
	n6 := NewMerkleNode(n3, n4, nil)

	// Level 3
	n7 := NewMerkleNode(n5, n6, nil)

	rootHash := fmt.Sprintf("%x", n7.Data)
	mTree := NewMerkleTree(data)

	if rootHash != fmt.Sprintf("%x", mTree.RootNode.Data) {
		t.Errorf("Merkle tree root hash is incorrect")
	}
}

func TestNewMerkleTree2(t *testing.T) {
	data := [][]byte{
		[]byte("node1"),
		[]byte("node2"),
	}
	// Level 1
	n1 := NewMerkleNode(nil, nil, data[0])
	n2 := NewMerkleNode(nil, nil, data[1])

	// Level 2
	n3 := NewMerkleNode(n1, n2, nil)

	rootHash := fmt.Sprintf("%x", n3.Data)
	mTree := NewMerkleTree(data)

	if rootHash != fmt.Sprintf("%x", mTree.RootNode.Data) {
		t.Errorf("Merkle tree root hash is incorrect")
	}
}
