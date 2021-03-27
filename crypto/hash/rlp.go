package hash

import (
	"hash"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

var kec256Pool = sync.Pool{
	New: func() interface{} {
		return sha3.NewLegacyKeccak256()
	},
}

// FromRLP hashes the RLP representation of the given object.
func FromRLP(x interface{}) (h common.Hash) {
	hw := kec256Pool.Get().(hash.Hash)
	defer func() {
		hw.Reset()
		kec256Pool.Put(hw)
	}()

	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

var sha256Pool = sync.Pool{
	New: func() interface{} {
		return sha3.New256()
	},
}

// FromRLPNew256 hashes the RLP representation of the given object using New256
func FromRLPNew256(x interface{}) (h common.Hash) {
	hw := sha256Pool.Get().(hash.Hash)
	defer func() {
		hw.Reset()
		sha256Pool.Put(hw)
	}()

	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
