package hash

import (
	"github.com/harmony-one/harmony/common"
	"github.com/harmony-one/harmony/rlp"
	"golang.org/x/crypto/sha3"
)

// FromRLP hashes the RLP representation of the given object.
func FromRLP(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// FromRLPNew256 hashes the RLP representation of the given object using New256
func FromRLPNew256(x interface{}) (h common.Hash) {
	hw := sha3.New256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
