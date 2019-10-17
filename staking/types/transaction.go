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
	Directive
	StakeMsg     interface{}
	AccountNonce uint64   `json:"nonce"      gencodec:"required"`
	Price        *big.Int `json:"gasPrice"   gencodec:"required"`
	GasLimit     uint64   `json:"gas"        gencodec:"required"`
	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`
	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

// StakingTransaction is a record captuing all staking operations
type StakingTransaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

// StakeMsgFulfiller is signature of callback intended to produce the StakeMsg
type StakeMsgFulfiller func() (Directive, interface{})

// NewStakingTransaction produces a new staking transaction record
func NewStakingTransaction(
	nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller,
) (*StakingTransaction, error) {
	directive, payload := f()
	// TODO(Double check that this is legitmate directive, use type switch)
	newStake := &StakingTransaction{data: txdata{
		directive,
		payload,
		nonce,
		big.NewInt(0).Set(gasPrice),
		gasLimit,
		big.NewInt(0),
		big.NewInt(0),
		big.NewInt(0),
		nil,
	}}
	return newStake, nil
}

var (
	// ErrInvalidSig is a bad signature
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// StakingTransactions is a stake slice type for basic sorting.
type StakingTransactions []*StakingTransaction

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *StakingTransaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := hash.FromRLP(tx)
	tx.hash.Store(v)
	return v
}

// WithSignature returns a new transaction with the given signature.
func (tx *StakingTransaction) WithSignature(signer Signer, sig []byte) (*StakingTransaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &StakingTransaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Gas returns gas of StakingTransaction.
func (tx *StakingTransaction) Gas() uint64 {
	return tx.data.GasLimit
}

// Price returns price of StakingTransaction.
func (tx *StakingTransaction) Price() *big.Int {
	return tx.data.Price
}

// ChainID is what chain this staking transaction for
func (tx *StakingTransaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}

// EncodeRLP implements rlp.Encoder
func (tx *StakingTransaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *StakingTransaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err != nil {
		return err
	}
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}
	return err
}

// Nonce returns nonce of staking tx
func (tx *StakingTransaction) Nonce() uint64 {
	return tx.data.AccountNonce
}

// StakingMsgToBytes returns the bytes of staking message
func (tx *StakingTransaction) StakingMsgToBytes() (by []byte, err error) {
	stakeType := tx.StakingType()

	switch stakeType {
	case DirectiveNewValidator:
		newValidator := tx.StakingMessage().(NewValidator)
		by, err = rlp.EncodeToBytes(newValidator)
	case DirectiveEditValidator:
		editValidator := tx.StakingMessage().(EditValidator)
		by, err = rlp.EncodeToBytes(editValidator)
	case DirectiveDelegate:
		delegate := tx.StakingMessage().(Delegate)
		by, err = rlp.EncodeToBytes(delegate)
	case DirectiveRedelegate:
		redelegate := tx.StakingMessage().(Redelegate)
		by, err = rlp.EncodeToBytes(redelegate)
	case DirectiveUndelegate:
		undelegate := tx.StakingMessage().(Undelegate)
		by, err = rlp.EncodeToBytes(undelegate)
	default:
		by = []byte{}
		err = ErrInvalidStakingKind
	}
	return
}

// StakingType returns the type of staking transaction
func (tx *StakingTransaction) StakingType() Directive {
	return tx.data.Directive
}

// StakingMessage returns the stake message of staking transaction
func (tx *StakingTransaction) StakingMessage() interface{} {
	return tx.data.StakeMsg
}

// SenderAddress returns the address of staking transaction sender
func (tx *StakingTransaction) SenderAddress() (common.Address, error) {
	signer := NewEIP155Signer(tx.ChainID())
	addr, err := Sender(signer, tx)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}
