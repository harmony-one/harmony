package types

import (
	"io"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

// BodyV0 is the V0 block body
type BodyV0 struct {
	f bodyFieldsV0
}

type bodyFieldsV0 struct {
	Transactions []*Transaction
	Uncles       []*block.Header
}

// Transactions returns the list of transactions.
//
// The returned list is a deep copy; the caller may do anything with it without
// affecting the original.
func (b *BodyV0) Transactions() (txs []*Transaction) {
	for _, tx := range b.f.Transactions {
		txs = append(txs, tx.Copy())
	}
	return txs
}

// TransactionAt returns the transaction at the given index in this block.
// It returns nil if index is out of bounds.
func (b *BodyV0) TransactionAt(index int) *Transaction {
	if index < 0 || index >= len(b.f.Transactions) {
		return nil
	}
	return b.f.Transactions[index].Copy()
}

// StakingTransactionAt returns the staking transaction at the given index in this block.
// It returns nil if index is out of bounds. (not supported by Body V0)
func (b *BodyV0) StakingTransactionAt(index int) *staking.StakingTransaction {
	// not supported
	return nil
}

// CXReceiptAt returns the CXReceipt at given index in this block
// It returns nil if index is out of bounds
// V0 will just return nil because we don't support CXReceipt
func (b *BodyV0) CXReceiptAt(index int) *CXReceipt {
	return nil
}

// SetTransactions sets the list of transactions with a deep copy of the given
// list.
func (b *BodyV0) SetTransactions(newTransactions []*Transaction) {
	var txs []*Transaction
	for _, tx := range newTransactions {
		txs = append(txs, tx.Copy())
	}
	b.f.Transactions = txs
}

// SetStakingTransactions sets the list of staking transactions with a deep copy of the given
// list. (not supported by Body V0)
func (b *BodyV0) SetStakingTransactions(newTransactions []*staking.StakingTransaction) {
	// not supported
}

// Uncles returns a deep copy of the list of uncle headers of this block.
func (b *BodyV0) Uncles() (uncles []*block.Header) {
	for _, uncle := range b.f.Uncles {
		uncles = append(uncles, CopyHeader(uncle))
	}
	return uncles
}

// SetUncles sets the list of uncle headers with a deep copy of the given list.
func (b *BodyV0) SetUncles(newUncle []*block.Header) {
	var uncles []*block.Header
	for _, uncle := range newUncle {
		uncles = append(uncles, CopyHeader(uncle))
	}
	b.f.Uncles = uncles
}

// IncomingReceipts returns a deep copy of the list of incoming cross-shard
// transaction receipts of this block.
func (b *BodyV0) IncomingReceipts() (incomingReceipts CXReceiptsProofs) {
	return nil
}

// StakingTransactions returns the list of staking transactions.
// The returned list is a deep copy; the caller may do anything with it without
// affecting the original.
func (b *BodyV0) StakingTransactions() (txs []*staking.StakingTransaction) {
	return nil
}

// SetIncomingReceipts sets the list of incoming cross-shard transaction
// receipts of this block with a dep copy of the given list.
func (b *BodyV0) SetIncomingReceipts(newIncomingReceipts CXReceiptsProofs) {
	if len(newIncomingReceipts) > 0 {
		utils.Logger().Warn().
			Msg("cannot store incoming CX receipts in v0 block body")
	}
}

// EncodeRLP RLP-encodes the block body into the given writer.
func (b *BodyV0) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &b.f)
}

// DecodeRLP RLP-decodes a block body from the given RLP stream into the
// receiver.
func (b *BodyV0) DecodeRLP(s *rlp.Stream) error {
	return s.Decode(&b.f)
}
