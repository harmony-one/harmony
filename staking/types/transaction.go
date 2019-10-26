package types

import (
	"errors"
	"io"
	"math/big"
	"reflect"
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

func (d *txdata) CopyFrom(d2 *txdata) {
	d.Directive = d2.Directive
	d.AccountNonce = d2.AccountNonce
	d.Price = new(big.Int).Set(d2.Price)
	d.GasLimit = d2.GasLimit
	d.StakeMsg = reflect.New(reflect.ValueOf(d2.StakeMsg).Elem().Type()).Interface()
	d.V = new(big.Int).Set(d2.V)
	d.R = new(big.Int).Set(d2.R)
	d.S = new(big.Int).Set(d2.S)
	d.Hash = copyHash(d2.Hash)
}

func copyHash(hash *common.Hash) *common.Hash {
	if hash == nil {
		return nil
	}
	copy := *hash
	return &copy
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

// Copy returns a copy of the transaction.
func (tx *StakingTransaction) Copy() *StakingTransaction {
	var tx2 StakingTransaction
	tx2.data.CopyFrom(&tx.data)
	return &tx2
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

// RLPEncodeStakeMsg ..
func (tx *StakingTransaction) RLPEncodeStakeMsg() (by []byte, err error) {
	return rlp.EncodeToBytes(tx.data.StakeMsg)
}

// RLPDecodeStakeMsg ..
func RLPDecodeStakeMsg(payload []byte, d Directive) (interface{}, error) {
	var oops error
	var ds interface{}

	switch _, ok := directiveNames[d]; ok {
	case false:
		return nil, ErrInvalidStakingKind
	default:
		switch d {
		case DirectiveCreateValidator:
			ds = &CreateValidator{}
		case DirectiveEditValidator:
			ds = &EditValidator{}
		case DirectiveDelegate:
			ds = &Delegate{}
		case DirectiveUndelegate:
			ds = &Undelegate{}
		case DirectiveCollectRewards:
			ds = &CollectRewards{}
		default:
			return nil, nil
		}
	}

	oops = rlp.DecodeBytes(payload, ds)

	if oops != nil {
		return nil, oops
	}

	return ds, nil
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
	addr, err := Sender(NewEIP155Signer(tx.ChainID()), tx)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}
