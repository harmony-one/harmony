package types

import (
	"io"
	"math/big"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	//MaxP2PNodeDataSize is a 1.25Mb heuristic data limit for DOS prevention on node message
	MaxP2PNodeDataSize = 1280 * 1024
	//MaxPoolTransactionDataSize is a 128KB heuristic data limit for DOS prevention on txn
	MaxPoolTransactionDataSize = 128 * 1024
	//MaxEncodedPoolTransactionSize is a heuristic raw/encoded data size limit. It has an additional 10KB for metadata
	MaxEncodedPoolTransactionSize = MaxPoolTransactionDataSize + (10 * 1024)
)

var (
	// ErrUnknownPoolTxType is returned when attempting to assert a PoolTransaction to its concrete type
	ErrUnknownPoolTxType = errors.New("unknown transaction type in tx-pool")
)

// PoolTransaction is the general transaction interface used by the tx pool
type PoolTransaction interface {
	CoreTransaction

	SenderAddress() (common.Address, error)
	Size() common.StorageSize
	Cost() (*big.Int, error)
	EncodeRLP(w io.Writer) error
	DecodeRLP(s *rlp.Stream) error
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
