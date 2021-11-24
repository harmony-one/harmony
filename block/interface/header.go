package blockif

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/shard"
)

// Header defines the block header interface.
type Header interface {
	// ParentHash is the header hash of the parent block.  For the genesis block
	// which has no parent by definition, this field is zeroed out.
	ParentHash() common.Hash

	// SetParentHash sets the parent hash field.
	SetParentHash(newParentHash common.Hash)

	// Coinbase is the address of the node that proposed this block and all
	// transactions in it.
	Coinbase() common.Address

	// SetCoinbase sets the coinbase address field.
	SetCoinbase(newCoinbase common.Address)

	// Root is the state (account) trie root hash.
	Root() common.Hash

	// SetRoot sets the state trie root hash field.
	SetRoot(newRoot common.Hash)

	// TxHash is the transaction trie root hash.
	TxHash() common.Hash

	// SetTxHash sets the transaction trie root hash field.
	SetTxHash(newTxHash common.Hash)

	// ReceiptHash is the same-shard transaction receipt trie hash.
	ReceiptHash() common.Hash

	// SetReceiptHash sets the same-shard transaction receipt trie hash.
	SetReceiptHash(newReceiptHash common.Hash)

	// OutgoingReceiptHash is the egress transaction receipt trie hash.
	OutgoingReceiptHash() common.Hash

	// SetOutgoingReceiptHash sets the egress transaction receipt trie hash.
	SetOutgoingReceiptHash(newOutgoingReceiptHash common.Hash)

	// IncomingReceiptHash is the ingress transaction receipt trie hash.
	IncomingReceiptHash() common.Hash

	// SetIncomingReceiptHash sets the ingress transaction receipt trie hash.
	SetIncomingReceiptHash(newIncomingReceiptHash common.Hash)

	// Bloom is the Bloom filter that indexes accounts and topics logged by smart
	// contract transactions (executions) in this block.
	Bloom() types.Bloom

	// SetBloom sets the smart contract log Bloom filter for this block.
	SetBloom(newBloom types.Bloom)

	// Number is the block number.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Number() *big.Int

	// SetNumber sets the block number.
	//
	// It stores a copy; the caller may freely modify the original.
	SetNumber(newNumber *big.Int)

	// GasLimit is the gas limit for transactions in this block.
	GasLimit() uint64

	// SetGasLimit sets the gas limit for transactions in this block.
	SetGasLimit(newGasLimit uint64)

	// GasUsed is the amount of gas used by transactions in this block.
	GasUsed() uint64

	// SetGasUsed sets the amount of gas used by transactions in this block.
	SetGasUsed(newGasUsed uint64)

	// Time is the UNIX timestamp of this block.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Time() *big.Int

	// SetTime sets the UNIX timestamp of this block.
	//
	// It stores a copy; the caller may freely modify the original.
	SetTime(newTime *big.Int)

	// Extra is the extra data field of this block.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Extra() []byte

	// SetExtra sets the extra data field of this block.
	//
	// It stores a copy; the caller may freely modify the original.
	SetExtra(newExtra []byte)

	// MixDigest is the mixhash.
	//
	// This field is a remnant from Ethereum, and Harmony does not use it and always
	// zeroes it out.
	MixDigest() common.Hash

	// SetMixDigest sets the mixhash of this block.
	SetMixDigest(newMixDigest common.Hash)

	// ViewID is the ID of the view in which this block was originally proposed.
	//
	// It normally increases by one for each subsequent block, or by more than one
	// if one or more PBFT/FBFT view changes have occurred.
	//
	// The returned instance is a copy; the caller may do anything with it.
	ViewID() *big.Int

	// SetViewID sets the view ID in which the block was originally proposed.
	//
	// It stores a copy; the caller may freely modify the original.
	SetViewID(newViewID *big.Int)

	// Epoch is the epoch number of this block.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Epoch() *big.Int

	// SetEpoch sets the epoch number of this block.
	//
	// It stores a copy; the caller may freely modify the original.
	SetEpoch(newEpoch *big.Int)

	// ShardID is the shard ID to which this block belongs.
	ShardID() uint32

	// SetShardID sets the shard ID to which this block belongs.
	SetShardID(newShardID uint32)

	// LastCommitSignature is the FBFT commit group signature for the last block.
	LastCommitSignature() [96]byte

	// SetLastCommitSignature sets the FBFT commit group signature for the last
	// block.
	SetLastCommitSignature(newLastCommitSignature [96]byte)

	// LastCommitBitmap is the signatory bitmap of the previous block.  Bit
	// positions index into committee member array.
	//
	// The returned slice is a copy; the caller may do anything with it.
	LastCommitBitmap() []byte

	// SetLastCommitBitmap sets the signatory bitmap of the previous block.
	//
	// It stores a copy; the caller may freely modify the original.
	SetLastCommitBitmap(newLastCommitBitmap []byte)

	// ShardStateHash is the shard state hash.
	ShardStateHash() common.Hash

	// SetShardStateHash sets the shard state hash.
	SetShardStateHash(newShardStateHash common.Hash)

	// Vrf is the output of the VRF for the epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Vrf() []byte

	// SetVrf sets the output of the VRF for the epoch.
	//
	// It stores a copy; the caller may freely modify the original.
	SetVrf(newVrf []byte)

	// Vdf is the output of the VDF for the epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Vdf() []byte

	// SetVdf sets the output of the VDF for the epoch.
	//
	// It stores a copy; the caller may freely modify the original.
	SetVdf(newVdf []byte)

	// ShardState is the RLP-encoded form of shard state (list of committees) for
	// the next epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	ShardState() []byte

	// SetShardState sets the RLP-encoded form of shard state
	//
	// It stores a copy; the caller may freely modify the original.
	SetShardState(newShardState []byte)

	// CrossLinks is the RLP-encoded form of non-beacon block headers chosen to be
	// canonical by the beacon committee.  This field is present only on beacon
	// chain block headers.
	//
	// The returned slice is a copy; the caller may do anything with it.
	CrossLinks() []byte

	// SetCrossLinks sets the RLP-encoded form of non-beacon block headers chosen to
	// be canonical by the beacon committee.
	//
	// It stores a copy; the caller may freely modify the original.
	SetCrossLinks(newCrossLinks []byte)

	// Hash returns the block hash of the header, which is simply the legacy
	// Keccak256 hash of its RLP encoding.
	Hash() common.Hash

	// Size returns the approximate memory used by all internal contents. It is
	// used to approximate and limit the memory consumption of various caches.
	Size() common.StorageSize

	// Logger returns a sub-logger with block contexts added.
	Logger(logger *zerolog.Logger) *zerolog.Logger

	// GetShardState returns the deserialized shard state object.
	// Note that header encoded shard state only exists in the last block of the previous epoch
	GetShardState() (shard.State, error)

	// Copy returns a copy of the header.
	Copy() Header

	// Slashes is the RLP-encoded form of []slash.Record,
	// The returned slice is a copy; the caller may do anything with it
	Slashes() []byte

	// SetSlashes sets the RLP-encoded form of slashes
	// It stores a copy; the caller may freely modify the original.
	SetSlashes(newSlashes []byte)
}
