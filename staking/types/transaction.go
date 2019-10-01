package types

import (
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/crypto/hash"
)

type txdata struct {
	AccountNonce uint64   `json:"nonce"      gencodec:"required"`
	Price        *big.Int `json:"gasPrice"   gencodec:"required"`
	GasLimit     uint64   `json:"gas"        gencodec:"required"`
	Message      `json:"msg"        gencodec:"required"`
	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

// Transaction is the staking transaction
type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// NewTransaction produces a new staking transaction record
func NewTransaction(
	nonce, gasLimit uint64,
	gasPrice *big.Int, sM *Message) *Transaction {
	return &Transaction{data: txdata{
		AccountNonce: nonce,
		Price:        big.NewInt(0).Set(gasPrice),
		GasLimit:     gasLimit,
		Message:      Message{sM.Kind, sM.Signer},
		V:            big.NewInt(0),
		R:            big.NewInt(0),
		S:            big.NewInt(0),
	}}
}

var (
	// ErrInvalidSig is a bad signature
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := hash.FromRLP(tx)
	tx.hash.Store(v)
	return v
}

// WithSignature returns a new transaction with the given signature.
func (tx *Transaction) WithSignature(
	signer Signer, sig []byte,
) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// ChainID is what chain this staking transaction for
func (tx *Transaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}

// Need to have equivalent of the gen_tx_json stuff
// func (tx *StakingTransaction) MarshalJSON() ([]byte, error) {
// 	hash := tx.Hash()
// 	data := tx.data
// 	data.Hash = &hash
// 	return data.MarshalJSON()
// }

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}
	return err
}
