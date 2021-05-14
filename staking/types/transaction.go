package types

import (
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

var (
	errStakingTransactionTypeCastErr = errors.New("cannot type cast to matching staking type")
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
	// This is workaround, direct RLP encoding/decoding not work
	if d2.StakeMsg == nil {
		utils.Logger().Debug().Msg("[CopyFrom] d2.StakeMsg is nil")
	}
	payload, _ := rlp.EncodeToBytes(d2.StakeMsg)
	restored, err := RLPDecodeStakeMsg(
		payload, d2.Directive,
	)
	if restored == nil || err != nil {
		utils.Logger().Error().Err(err).Msg("[CopyFrom] RLPDeocdeStakeMsg returns nil/err")
		d.StakeMsg = d2.StakeMsg
	} else {
		d.StakeMsg = restored.(StakeMsg).Copy()
	}
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

// Len ..
func (s StakingTransactions) Len() int { return len(s) }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s StakingTransactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

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

func (tx *StakingTransaction) SetRawSignature(v, r, s *big.Int) {
	tx.data.R, tx.data.S, tx.data.V = r, s, v
}

// GasLimit returns gas of StakingTransaction.
func (tx *StakingTransaction) GasLimit() uint64 {
	return tx.data.GasLimit
}

// GasPrice returns price of StakingTransaction.
func (tx *StakingTransaction) GasPrice() *big.Int {
	return tx.data.Price
}

// Cost ..
func (tx *StakingTransaction) Cost() (*big.Int, error) {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	switch tx.StakingType() {
	case DirectiveCreateValidator:
		msg, err := RLPDecodeStakeMsg(tx.Data(), DirectiveCreateValidator)
		if err != nil {
			return nil, err
		}
		stkMsg, ok := msg.(*CreateValidator)
		if !ok {
			return nil, errStakingTransactionTypeCastErr
		}
		total.Add(total, stkMsg.Amount)
	case DirectiveDelegate:
		// Temporary hack: Cost function is not accurate for delegate transaction.
		// Thus the cost validation is done in `txPool.validateTx`.
		// TODO: refactor this hack.
	default:
	}
	return total, nil
}

// ChainID is what chain this staking transaction for
func (tx *StakingTransaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}

// ShardID returns which shard id this transaction was signed for, implicitly shard 0.
func (tx *StakingTransaction) ShardID() uint32 {
	return shard.BeaconChainShardID
}

// ToShardID returns which shard id this transaction was signed for, implicitly shard 0.
func (tx *StakingTransaction) ToShardID() uint32 {
	return shard.BeaconChainShardID
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

// Protected ..
func (tx *StakingTransaction) Protected() bool {
	return true
}

// To ..
func (tx *StakingTransaction) To() *common.Address {
	return nil
}

// Data ..
func (tx *StakingTransaction) Data() []byte {
	data, err := tx.RLPEncodeStakeMsg()
	if err != nil {
		return nil
	}
	return data
}

// Value ..
func (tx *StakingTransaction) Value() *big.Int {
	return new(big.Int).SetInt64(0)
}

// Size ..
func (tx *StakingTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// IsEthCompatible returns whether the txn is ethereum compatible
func (tx *StakingTransaction) IsEthCompatible() bool {
	return false
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
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

// RawSignatureValues return raw signature values.
func (tx *StakingTransaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
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

// From returns the sender address of the transaction
func (tx *StakingTransaction) From() *atomic.Value {
	return &tx.from
}
