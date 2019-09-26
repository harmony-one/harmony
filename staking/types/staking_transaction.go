package types

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
)

// StakingTransaction struct.
type StakingTransaction struct {
	AccountNonce uint64         `json:"nonce"      gencodec:"required"`
	Price        *big.Int       `json:"gasPrice"   gencodec:"required"`
	GasLimit     uint64         `json:"gas"        gencodec:"required"`
	Msg          StakingMessage `json:"msg"      gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	hash *common.Hash `json:"hash" rlp:"-"`
}

// StakingTransactions is a Transaction slice type for basic sorting.
type StakingTransactions []*StakingTransaction

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *StakingTransaction) Hash() common.Hash {
	emptyHash := common.Hash{}
	if bytes.Compare(tx.hash[:], emptyHash[:]) == 0 {
		h := hash.FromRLP(tx)
		tx.hash = &h
	}
	return *tx.hash
}
