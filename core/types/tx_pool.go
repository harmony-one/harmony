package types

import (
	"io"
	"math/big"

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
