package types

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/common"
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
	Hash *common.Hash `json:"hash" rlp:"-"`
}

// StakingTransactions is a Transaction slice type for basic sorting.
type StakingTransactions []*StakingTransaction
