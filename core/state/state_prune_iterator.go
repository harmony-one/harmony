package state

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/trie"
)

type differenceIterator struct {
	a, b  trie.NodeIterator // Nodes returned are those in b - a.
	eof   bool              // Indicates a has run out of elements
	count int               // Number of nodes scanned on either trie
}

// NewPruneIterator is same as trie.NewDifferenceIterator, except for the compareNodes function.
func NewPruneIterator(a, b trie.NodeIterator) (trie.NodeIterator, *int) {
	a.Next(true)
	it := &differenceIterator{
		a: a,
		b: b,
	}
	return it, &it.count
}

func (it *differenceIterator) Hash() common.Hash {
	return it.b.Hash()
}

func (it *differenceIterator) Parent() common.Hash {
	return it.b.Parent()
}

func (it *differenceIterator) Leaf() bool {
	return it.b.Leaf()
}

func (it *differenceIterator) LeafKey() []byte {
	return it.b.LeafKey()
}

func (it *differenceIterator) LeafBlob() []byte {
	return it.b.LeafBlob()
}

func (it *differenceIterator) LeafProof() [][]byte {
	return it.b.LeafProof()
}

func (it *differenceIterator) Path() []byte {
	return it.b.Path()
}

func (it *differenceIterator) Next(bool) bool {
	// Invariants:
	// - We always advance at least one element in b.
	// - At the start of this function, a's path is lexically greater than b's.
	if !it.b.Next(true) {
		return false
	}
	it.count++

	if it.eof {
		// a has reached eof, so we just return all elements from b
		return true
	}

	for {
		switch compareNodes(it.a, it.b) {
		case -1:
			// b jumped past a; advance a
			if !it.a.Next(true) {
				it.eof = true
				return true
			}
			it.count++
		case 1:
			// b is before a
			return true
		case 0:
			// a and b are identical; skip this whole subtree if the nodes have hashes
			hasHash := it.a.Hash() == common.Hash{}
			if !it.b.Next(hasHash) {
				return false
			}
			it.count++
			if !it.a.Next(hasHash) {
				it.eof = true
				return true
			}
			it.count++
		}
	}
}

func (it *differenceIterator) Error() error {
	if err := it.a.Error(); err != nil {
		return err
	}
	return it.b.Error()
}

func compareNodes(a, b trie.NodeIterator) int {
	if cmp := bytes.Compare(a.Path(), b.Path()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && !b.Leaf() {
		return -1
	} else if b.Leaf() && !a.Leaf() {
		return 1
	}
	if cmp := bytes.Compare(a.Hash().Bytes(), b.Hash().Bytes()); cmp != 0 {
		return cmp
	}
	if a.Leaf() && b.Leaf() {
		// differen with trie.compareNodes: always return 1.
		// This makes it possible to access a's leafKey outside in this case.
		// Otherwise a's leafKey would be skipped.
		return 1
	}
	return 0
}
