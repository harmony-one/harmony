// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/harmony-one/harmony/internal/params"

	"github.com/ethereum/go-ethereum/common/hexutil"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"

	"github.com/harmony-one/harmony/crypto/hash"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:generate gencodec -type ethTxdata -field-override ethTxdataMarshaling -out gen_eth_tx_json.go

// EthTransaction ethereum-compatible transaction
type EthTransaction struct {
	data ethTxdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
	// time at which the node received the tx
	// and not the time set by the sender
	time time.Time
}

type ethTxdata struct {
	AccountNonce uint64          `json:"nonce"    gencodec:"required"`
	Price        *big.Int        `json:"gasPrice" gencodec:"required"`
	GasLimit     uint64          `json:"gas"      gencodec:"required"`
	Recipient    *common.Address `json:"to"       rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"    gencodec:"required"`
	Payload      []byte          `json:"input"    gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func (d *ethTxdata) CopyFrom(d2 *ethTxdata) {
	d.AccountNonce = d2.AccountNonce
	d.Price = new(big.Int).Set(d2.Price)
	d.GasLimit = d2.GasLimit
	d.Recipient = copyAddr(d2.Recipient)
	d.Amount = new(big.Int).Set(d2.Amount)
	d.Payload = append(d2.Payload[:0:0], d2.Payload...)
	d.V = new(big.Int).Set(d2.V)
	d.R = new(big.Int).Set(d2.R)
	d.S = new(big.Int).Set(d2.S)
	d.Hash = copyHash(d2.Hash)
}

type ethTxdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

// NewEthTransaction returns new ethereum-compatible transaction, which works as a intra-shard transaction
func NewEthTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *EthTransaction {
	return newEthTransaction(nonce, &to, amount, gasLimit, gasPrice, data)
}

func newEthTransaction(nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *EthTransaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := ethTxdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &EthTransaction{data: d, time: time.Now()}
}

// From returns the sender address of the transaction
func (tx *EthTransaction) From() *atomic.Value {
	return &tx.from
}

// Time returns the time at which the transaction was received by the node
func (tx *EthTransaction) Time() time.Time {
	return tx.time
}

// V value of the transaction signature
func (tx *EthTransaction) V() *big.Int {
	return tx.data.V
}

// R value of the transaction signature
func (tx *EthTransaction) R() *big.Int {
	return tx.data.R
}

// S value of the transaction signature
func (tx *EthTransaction) S() *big.Int {
	return tx.data.S
}

// Value is the amount of ONE token transfered (in Atto)
func (tx *EthTransaction) Value() *big.Int {
	return tx.data.Amount
}

// GasLimit of the transcation
func (tx *EthTransaction) GasLimit() uint64 {
	return tx.data.GasLimit
}

// Data returns data payload of Transaction.
func (tx *EthTransaction) Data() []byte {
	return common.CopyBytes(tx.data.Payload)
}

// ShardID returns which shard id this transaction was signed for (if at all)
func (tx *EthTransaction) ShardID() uint32 {
	return tx.shardID()
}

// ToShardID returns the destination shard id this transaction is going to
func (tx *EthTransaction) ToShardID() uint32 {
	return tx.shardID()
}

func (tx *EthTransaction) shardID() uint32 {
	ethChainIDBase := nodeconfig.GetDefaultConfig().GetNetworkType().ChainConfig().EthCompatibleChainID
	return uint32(tx.ChainID().Uint64()-ethChainIDBase.Uint64()) + nodeconfig.GetDefaultConfig().ShardID
}

// ChainID returns which chain id this transaction was signed for (if at all)
func (tx *EthTransaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *EthTransaction) Protected() bool {
	return isProtectedV(tx.data.V)
}

// Copy returns a copy of the transaction.
func (tx *EthTransaction) Copy() *EthTransaction {
	var tx2 EthTransaction
	tx2.data.CopyFrom(&tx.data)
	tx2.time = tx.time
	return &tx2
}

// ConvertToHmy converts eth txn to hmy txn by filling in ShardID and ToShardID fields.
func (tx *EthTransaction) ConvertToHmy() *Transaction {
	var tx2 Transaction
	d := &tx.data
	d2 := &tx2.data

	d2.AccountNonce = d.AccountNonce
	d2.Price = new(big.Int).Set(d.Price)
	d2.GasLimit = d.GasLimit
	d2.Recipient = copyAddr(d.Recipient)
	d2.Amount = new(big.Int).Set(d.Amount)
	d2.Payload = append(d.Payload[:0:0], d.Payload...)
	d2.V = new(big.Int).Set(d.V)
	d2.R = new(big.Int).Set(d.R)
	d2.S = new(big.Int).Set(d.S)

	d2.ShardID = tx.ShardID()
	d2.ToShardID = tx.ToShardID()

	copy := tx2.Hash()
	d2.Hash = &copy

	tx2.time = tx.time

	return &tx2
}

// EncodeRLP implements rlp.Encoder
func (tx *EthTransaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *EthTransaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
		tx.time = time.Now()
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *EthTransaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *EthTransaction) UnmarshalJSON(input []byte) error {
	var dec ethTxdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}

	withSignature := dec.V.Sign() != 0 || dec.R.Sign() != 0 || dec.S.Sign() != 0
	if withSignature {
		var V byte
		if isProtectedV(dec.V) {
			chainID := deriveChainID(dec.V).Uint64()
			V = byte(dec.V.Uint64() - 35 - 2*chainID)
		} else {
			V = byte(dec.V.Uint64() - 27)
		}
		if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
			return ErrInvalidSig
		}
	}

	*tx = EthTransaction{data: dec}
	return nil
}

// Gas returns gas of Transaction.
func (tx *EthTransaction) Gas() uint64 {
	return tx.data.GasLimit
}

// GasPrice returns gas price of Transaction.
func (tx *EthTransaction) GasPrice() *big.Int {
	return new(big.Int).Set(tx.data.Price)
}

// Nonce returns account nonce from Transaction.
func (tx *EthTransaction) Nonce() uint64 {
	return tx.data.AccountNonce
}

// CheckNonce returns check nonce from Transaction.
func (tx *EthTransaction) CheckNonce() bool {
	return true
}

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *EthTransaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *EthTransaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := hash.FromRLP(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *EthTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// Cost returns amount + gasprice * gaslimit.
func (tx *EthTransaction) Cost() (*big.Int, error) {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total, nil
}

// SenderAddress returns the address of transaction sender
// Note that mainnet has unprotected transactions prior to Epoch 28
func (tx *EthTransaction) SenderAddress() (common.Address, error) {
	var signer Signer
	if !tx.Protected() {
		signer = HomesteadSigner{}
	} else {
		signer = NewEIP155Signer(tx.ChainID())
	}
	addr, err := Sender(signer, tx)
	if err != nil {
		return common.Address{}, err
	}
	return addr, nil
}

// IsEthCompatible returns whether the txn is ethereum compatible
func (tx *EthTransaction) IsEthCompatible() bool {
	return params.IsEthCompatible(tx.ChainID())
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *EthTransaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be in the [R || S || V] format where V is 0 or 1.
func (tx *EthTransaction) WithSignature(signer Signer, sig []byte) (*EthTransaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &EthTransaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// RawSignatureValues returns the V, R, S signature values of the transaction.
// The return values should not be modified by the caller.
func (tx *EthTransaction) RawSignatureValues() (v, r, s *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

// EthTransactions is a Transaction slice type for basic sorting.
type EthTransactions []*EthTransaction

// Len returns the length of s.
func (s EthTransactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s EthTransactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s EthTransactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}
