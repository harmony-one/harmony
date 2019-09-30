package types

import (
	"bytes"
	"errors"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"
)

// StakingTransaction struct.
type txdata struct {
	AccountNonce uint64         `json:"nonce"      gencodec:"required"`
	Price        *big.Int       `json:"gasPrice"   gencodec:"required"`
	GasLimit     uint64         `json:"gas"        gencodec:"required"`
	Msg          StakingMessage `json:"msg"        gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	hash *common.Hash `json:"hash" rlp:"-"`
}

// StakingTransaction is the staking transaction
type StakingTransaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// NewTransaction produces a new staking transaction record
func NewTransaction(
	nonce, gasLimit uint64,
	gasPrice *big.Int, sM StakingMessage) *StakingTransaction {
	p := txdata{nonce, gasPrice, gasLimit, sM, nil, nil, nil, nil}
	return &StakingTransaction{data: p}
}

var (
	// ErrInvalidSig is a bad signature
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// StakingTransactions is a Transaction slice type for basic sorting.
type StakingTransactions []*StakingTransaction

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *StakingTransaction) Hash() common.Hash {
	emptyHash := common.Hash{}
	if bytes.Compare(tx.data.hash[:], emptyHash[:]) == 0 {
		h := hash.FromRLP(tx)
		tx.data.hash = &h
	}
	return *tx.data.hash
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *StakingTransaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// anything not 27 or 28 is considered protected
	return true
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *StakingTransaction) WithSignature(signer Signer, sig []byte) (*StakingTransaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &StakingTransaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// ChainID is what chain this staking transaction for
func (tx *StakingTransaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}
