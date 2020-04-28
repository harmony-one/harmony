package types

import (
	"bytes"
	"container/heap"
	"io"
	"math/big"
	"sort"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	//MaxPoolTransactionDataSize is a 32KB heuristic data limit for DOS prevention
	MaxPoolTransactionDataSize = 32 * 1024
	//MaxEncodedPoolTransactionSize is a heuristic raw/encoded data size limit. It has an additional 10KB for metadata
	MaxEncodedPoolTransactionSize = MaxPoolTransactionDataSize + (10 * 1024)
)

var (
	// ErrUnknownPoolTxType is returned when attempting to assert a PoolTransaction to its concrete type
	ErrUnknownPoolTxType = errors.New("unknown transaction type in tx-pool")
)

// PoolTransaction is the general transaction interface used by the tx pool
type PoolTransaction interface {
	Hash() common.Hash
	Nonce() uint64
	ChainID() *big.Int
	ShardID() uint32
	To() *common.Address
	Size() common.StorageSize
	Data() []byte
	GasPrice() *big.Int
	Gas() uint64
	Cost() (*big.Int, error)
	Value() *big.Int
	EncodeRLP(w io.Writer) error
	DecodeRLP(s *rlp.Stream) error
	Protected() bool
}

// PoolTransactionSender returns the address derived from the signature (V, R, S) u
// sing secp256k1 elliptic curve and an error if it failed deriving or upon an
// incorrect signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
//
// Note that the signer is an interface since different txs have different signers.
func PoolTransactionSender(signer interface{}, tx PoolTransaction) (common.Address, error) {
	if plainTx, ok := tx.(*Transaction); ok {
		if sig, ok := signer.(Signer); ok {
			return Sender(sig, plainTx)
		}
	} else if stakingTx, ok := tx.(*staking.StakingTransaction); ok {
		return stakingTx.SenderAddress()
	}
	return common.Address{}, errors.WithMessage(ErrUnknownPoolTxType, "when fetching transaction sender")
}

// PoolTransactions is a PoolTransactions slice type for basic sorting.
type PoolTransactions []PoolTransaction

// Len returns the length of s.
func (s PoolTransactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s PoolTransactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s PoolTransactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// PoolTxDifference returns a new set which is the difference between a and b.
func PoolTxDifference(a, b PoolTransactions) PoolTransactions {
	keep := make(PoolTransactions, 0, len(a))

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

// PoolTxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type PoolTxByNonce PoolTransactions

func (s PoolTxByNonce) Len() int           { return len(s) }
func (s PoolTxByNonce) Less(i, j int) bool { return (s[i]).Nonce() < (s[j]).Nonce() }
func (s PoolTxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// PoolTxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type PoolTxByPrice PoolTransactions

func (s PoolTxByPrice) Len() int           { return len(s) }
func (s PoolTxByPrice) Less(i, j int) bool { return s[i].GasPrice().Cmp(s[j].GasPrice()) > 0 }
func (s PoolTxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// Push pushes a transaction.
func (s *PoolTxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

// Pop pops a transaction.
func (s *PoolTxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// PoolTransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type PoolTransactionsByPriceAndNonce struct {
	txs    map[common.Address]PoolTransactions // Per account nonce-sorted list of transactions
	heads  PoolTxByPrice                       // Next transaction for each unique account (price heap)
	signer Signer                              // Signer for the set of transactions
}

// NewPoolTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewPoolTransactionsByPriceAndNonce(signer Signer, txs map[common.Address]PoolTransactions) *PoolTransactionsByPriceAndNonce {
	// Initialize a price based heap with the head transactions
	sortedAddrs := []common.Address{}

	for from := range txs {
		sortedAddrs = append(sortedAddrs, from)
	}

	sort.SliceStable(sortedAddrs, func(i, j int) bool {
		return bytes.Compare(sortedAddrs[i].Bytes(), sortedAddrs[j].Bytes()) < 0
	})

	heads := make(PoolTxByPrice, 0, len(txs))
	for _, from := range sortedAddrs {
		accTxs := txs[from]
		heads = append(heads, accTxs[0])
		// Ensure the sender address is from the signer
		var acc common.Address
		if plainTx, ok := accTxs[0].(*Transaction); ok {
			acc, _ = Sender(signer, plainTx)
		} else if stx, ok := accTxs[0].(*staking.StakingTransaction); ok {
			acc, _ = stx.SenderAddress()
		}
		txs[acc] = accTxs[1:]
		if from != acc {
			delete(txs, from)
		}
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &PoolTransactionsByPriceAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *PoolTransactionsByPriceAndNonce) Peek() PoolTransaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *PoolTransactionsByPriceAndNonce) Shift() {
	var acc common.Address
	if plainTx, ok := t.heads[0].(*Transaction); ok {
		acc, _ = Sender(t.signer, plainTx)
	} else if stx, ok := t.heads[0].(*staking.StakingTransaction); ok {
		acc, _ = stx.SenderAddress()
	}
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
func (t *PoolTransactionsByPriceAndNonce) Pop() {
	heap.Pop(&t.heads)
}
