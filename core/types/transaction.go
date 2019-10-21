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
	"container/heap"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
)

// no go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

// Errors constants for Transaction.
var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// TransactionType different types of transactions
type TransactionType byte

// Different Transaction Types
const (
	SameShardTx     TransactionType = iota
	SubtractionOnly                 // only subtract tokens from source shard account
	InvalidTx
)

// Transaction struct.
type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

//String print mode string
func (txType TransactionType) String() string {
	if txType == SameShardTx {
		return "SameShardTx"
	} else if txType == SubtractionOnly {
		return "SubtractionOnly"
	} else if txType == InvalidTx {
		return "InvalidTx"
	}
	return "Unknown"
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"      gencodec:"required"`
	Price        *big.Int        `json:"gasPrice"   gencodec:"required"`
	GasLimit     uint64          `json:"gas"        gencodec:"required"`
	ShardID      uint32          `json:"shardID"    gencodec:"required"`
	ToShardID    uint32          `json:"toShardID"  gencodec:"required"`
	Recipient    *common.Address `json:"to"         rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"      gencodec:"required"`
	Payload      []byte          `json:"input"      gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func copyAddr(addr *common.Address) *common.Address {
	if addr == nil {
		return nil
	}
	copy := *addr
	return &copy
}

func copyHash(hash *common.Hash) *common.Hash {
	if hash == nil {
		return nil
	}
	copy := *hash
	return &copy
}

func (d *txdata) CopyFrom(d2 *txdata) {
	d.AccountNonce = d2.AccountNonce
	d.Price = new(big.Int).Set(d2.Price)
	d.GasLimit = d2.GasLimit
	d.ShardID = d2.ShardID
	d.ToShardID = d2.ToShardID
	d.Recipient = copyAddr(d2.Recipient)
	d.Amount = new(big.Int).Set(d2.Amount)
	d.Payload = append(d2.Payload[:0:0], d2.Payload...)
	d.V = new(big.Int).Set(d2.V)
	d.R = new(big.Int).Set(d2.R)
	d.S = new(big.Int).Set(d2.S)
	d.Hash = copyHash(d2.Hash)
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

// NewTransaction returns new transaction, this method is to create same shard transaction
func NewTransaction(nonce uint64, to common.Address, shardID uint32, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, &to, shardID, amount, gasLimit, gasPrice, data)
}

// NewCrossShardTransaction returns new cross shard transaction
func NewCrossShardTransaction(nonce uint64, to *common.Address, shardID uint32, toShardID uint32, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newCrossShardTransaction(nonce, to, shardID, toShardID, amount, gasLimit, gasPrice, data)
}

// NewContractCreation returns same shard contract transaction.
func NewContractCreation(nonce uint64, shardID uint32, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, nil, shardID, amount, gasLimit, gasPrice, data)
}

func newTransaction(nonce uint64, to *common.Address, shardID uint32, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		ShardID:      shardID,
		ToShardID:    shardID,
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

	return &Transaction{data: d}
}

func newCrossShardTransaction(nonce uint64, to *common.Address, shardID uint32, toShardID uint32, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		ShardID:      shardID,
		ToShardID:    toShardID,
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

	return &Transaction{data: d}
}

// ChainID returns which chain id this transaction was signed for (if at all)
func (tx *Transaction) ChainID() *big.Int {
	return deriveChainID(tx.data.V)
}

// ShardID returns which shard id this transaction was signed for (if at all)
func (tx *Transaction) ShardID() uint32 {
	return tx.data.ShardID
}

// ToShardID returns the destination shard id this transaction is going to
func (tx *Transaction) ToShardID() uint32 {
	return tx.data.ToShardID
}

// Protected returns whether the transaction is protected from replay protection.
func (tx *Transaction) Protected() bool {
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

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
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

	*tx = Transaction{data: dec}
	return nil
}

// Data returns data payload of Transaction.
func (tx *Transaction) Data() []byte {
	return common.CopyBytes(tx.data.Payload)
}

// Gas returns gas of Transaction.
func (tx *Transaction) Gas() uint64 {
	return tx.data.GasLimit
}

// GasPrice returns gas price of Transaction.
func (tx *Transaction) GasPrice() *big.Int {
	return new(big.Int).Set(tx.data.Price)
}

// Value returns data payload of Transaction.
func (tx *Transaction) Value() *big.Int {
	return new(big.Int).Set(tx.data.Amount)
}

// Nonce returns account nonce from Transaction.
func (tx *Transaction) Nonce() uint64 {
	return tx.data.AccountNonce
}

// CheckNonce returns check nonce from Transaction.
func (tx *Transaction) CheckNonce() bool {
	return true
}

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

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

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previously cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a core.Message.
//
// AsMessage requires a signer to derive the sender.
//
// XXX Rename message to something less arbitrary?
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
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
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Cost returns amount + gasprice * gaslimit.
func (tx *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

// RawSignatureValues return raw signature values.
func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

// Copy returns a copy of the transaction.
func (tx *Transaction) Copy() *Transaction {
	var tx2 Transaction
	tx2.data.CopyFrom(&tx.data)
	return &tx2
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// ToShardID returns the destination shardID of given transaction
func (s Transactions) ToShardID(i int) uint32 {
	return s[i].data.ToShardID
}

// MaxToShardID returns 0, arbitrary value, NOT use
func (s Transactions) MaxToShardID() uint32 {
	return 0
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].data.AccountNonce < s[j].data.AccountNonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].data.Price.Cmp(s[j].data.Price) > 0 }
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Push pushes a transaction.
func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

// Pop pops a transaction.
func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByPriceAndNonce struct {
	txs    map[common.Address]Transactions // Per account nonce-sorted list of transactions
	heads  TxByPrice                       // Next transaction for each unique account (price heap)
	signer Signer                          // Signer for the set of transactions
}

// NewTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByPriceAndNonce(signer Signer, txs map[common.Address]Transactions) *TransactionsByPriceAndNonce {
	// Initialize a price based heap with the head transactions
	heads := make(TxByPrice, 0, len(txs))
	for from, accTxs := range txs {
		heads = append(heads, accTxs[0])
		// Ensure the sender address is from the signer
		acc, _ := Sender(signer, accTxs[0])
		txs[acc] = accTxs[1:]
		if from != acc {
			delete(txs, from)
		}
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByPriceAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *TransactionsByPriceAndNonce) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByPriceAndNonce) Shift() {
	acc, _ := Sender(t.signer, t.heads[0])
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0], t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByPriceAndNonce) Pop() {
	heap.Pop(&t.heads)
}

// Message is a fully derived transaction and implements core.Message
// NOTE: In a future PR this will be removed.
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
	txType     TransactionType
}

// NewMessage returns new message.
func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

// From returns from address from Message.
func (m Message) From() common.Address {
	return m.from
}

// To returns to address from Message.
func (m Message) To() *common.Address {
	return m.to
}

// GasPrice returns gas price from Message.
func (m Message) GasPrice() *big.Int {
	return m.gasPrice
}

// Value returns the value amount from Message.
func (m Message) Value() *big.Int {
	return m.amount
}

// Gas returns gas limit of the Message.
func (m Message) Gas() uint64 {
	return m.gasLimit
}

// Nonce returns Nonce of the Message.
func (m Message) Nonce() uint64 {
	return m.nonce
}

// Data return data of the Message.
func (m Message) Data() []byte {
	return m.data
}

// CheckNonce returns checkNonce of Message.
func (m Message) CheckNonce() bool {
	return m.checkNonce
}

// Type returns the type of message
func (m Message) Type() TransactionType {
	return m.txType
}

// SetType set the type of message
func (m Message) SetType(typ TransactionType) {
	m.txType = typ
}

// RecentTxsStats is a recent transactions stats map tracking stats like BlockTxsCounts.
type RecentTxsStats map[uint64]BlockTxsCounts

// String returns the string formatted representation of RecentTxsStats
func (rts RecentTxsStats) String() string {
	ret := "{ "
	for blockNum, blockTxsCounts := range rts {
		ret += fmt.Sprintf("blockNum:%d=%s", blockNum, blockTxsCounts.String())
	}
	ret += " }"
	return ret
}

// BlockTxsCounts is a transactions counts map of
// the number of transactions made by each account in a block on this node.
type BlockTxsCounts map[common.Address]uint64

// String returns the string formatted representation of BlockTxsCounts
func (btc BlockTxsCounts) String() string {
	ret := "{ "
	for sender, numTxs := range btc {
		ret += fmt.Sprintf("%s:%d,", common2.MustAddressToBech32(sender), numTxs)
	}
	ret += " }"
	return ret
}
