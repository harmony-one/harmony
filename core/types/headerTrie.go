package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/block"
)

type proofList [][]byte

func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}

func (n *proofList) Delete(key []byte) error {
	panic("not supported")
}

type HeaderTrie struct {
	*trie.Trie
}

func NewHeaderTrie(root common.Hash, db *trie.Database) (*HeaderTrie, error) {
	trie, err := trie.New(root, db)
	if err != nil {
		return nil, err
	}
	return &HeaderTrie{trie}, nil

}

func insertKey(number *big.Int) []byte {
	// Use the square to discretize the number.
	// if the key is continuous, the proof data size may be larger.
	key, _ := rlp.EncodeToBytes(number.Mul(number, number))
	return key
}

func insertValue(header *block.Header) []byte {
	// insert header Hash and receiptRoot into the tree.
	// so can use the tree prove receiptRoot directly
	var value [common.HashLength * 2]byte
	hash := header.Hash()
	receiptHash := header.ReceiptHash()
	copy(value[:], hash[:])
	copy(value[common.HashLength:], receiptHash[:])
	return value[:]
}

func (t HeaderTrie) Insert(header *block.Header) {
	key := insertKey(header.Number())
	value := insertValue(header)
	t.Update(key, value)
}

func (t HeaderTrie) Delete(number *big.Int) {
	key := insertKey(number)
	t.Trie.Delete(key)
}

func (t HeaderTrie) Prove(number *big.Int) ([][]byte, error) {
	var proof proofList
	key := insertKey(number)
	err := t.Trie.Prove(key, 0, &proof)
	return [][]byte(proof), err
}
