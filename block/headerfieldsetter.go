package block

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// HeaderFieldSetter is a header field setter.
//
// See NewHeaderWith for how it is used.
type HeaderFieldSetter struct {
	h *Header
}

// ParentHash sets the parent hash field.
func (s HeaderFieldSetter) ParentHash(newParentHash common.Hash) HeaderFieldSetter {
	s.h.SetParentHash(newParentHash)
	return s
}

// Coinbase sets the coinbase address field.
func (s HeaderFieldSetter) Coinbase(newCoinbase common.Address) HeaderFieldSetter {
	s.h.SetCoinbase(newCoinbase)
	return s
}

// Root sets the state trie root hash field.
func (s HeaderFieldSetter) Root(newRoot common.Hash) HeaderFieldSetter {
	s.h.SetRoot(newRoot)
	return s
}

// TxHash sets the transaction trie root hash field.
func (s HeaderFieldSetter) TxHash(newTxHash common.Hash) HeaderFieldSetter {
	s.h.SetTxHash(newTxHash)
	return s
}

// ReceiptHash sets the same-shard transaction receipt trie hash.
func (s HeaderFieldSetter) ReceiptHash(newReceiptHash common.Hash) HeaderFieldSetter {
	s.h.SetReceiptHash(newReceiptHash)
	return s
}

// OutgoingReceiptHash sets the egress transaction receipt trie hash.
func (s HeaderFieldSetter) OutgoingReceiptHash(newOutgoingReceiptHash common.Hash) HeaderFieldSetter {
	s.h.SetOutgoingReceiptHash(newOutgoingReceiptHash)
	return s
}

// IncomingReceiptHash sets the ingress transaction receipt trie hash.
func (s HeaderFieldSetter) IncomingReceiptHash(newIncomingReceiptHash common.Hash) HeaderFieldSetter {
	s.h.SetIncomingReceiptHash(newIncomingReceiptHash)
	return s
}

// Bloom sets the smart contract log Bloom filter for this block.
func (s HeaderFieldSetter) Bloom(newBloom types.Bloom) HeaderFieldSetter {
	s.h.SetBloom(newBloom)
	return s
}

// Number sets the block number.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Number(newNumber *big.Int) HeaderFieldSetter {
	s.h.SetNumber(newNumber)
	return s
}

// GasLimit sets the gas limit for transactions in this block.
func (s HeaderFieldSetter) GasLimit(newGasLimit uint64) HeaderFieldSetter {
	s.h.SetGasLimit(newGasLimit)
	return s
}

// GasUsed sets the amount of gas used by transactions in this block.
func (s HeaderFieldSetter) GasUsed(newGasUsed uint64) HeaderFieldSetter {
	s.h.SetGasUsed(newGasUsed)
	return s
}

// Time sets the UNIX timestamp of this block.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Time(newTime *big.Int) HeaderFieldSetter {
	s.h.SetTime(newTime)
	return s
}

// Extra sets the extra data field of this block.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Extra(newExtra []byte) HeaderFieldSetter {
	s.h.SetExtra(newExtra)
	return s
}

// MixDigest sets the mixhash of this block.
func (s HeaderFieldSetter) MixDigest(newMixDigest common.Hash) HeaderFieldSetter {
	s.h.SetMixDigest(newMixDigest)
	return s
}

// ViewID sets the view ID in which the block was originally proposed.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) ViewID(newViewID *big.Int) HeaderFieldSetter {
	s.h.SetViewID(newViewID)
	return s
}

// Epoch sets the epoch number of this block.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Epoch(newEpoch *big.Int) HeaderFieldSetter {
	s.h.SetEpoch(newEpoch)
	return s
}

// ShardID sets the shard ID to which this block belongs.
func (s HeaderFieldSetter) ShardID(newShardID uint32) HeaderFieldSetter {
	s.h.SetShardID(newShardID)
	return s
}

// LastCommitSignature sets the FBFT commit group signature for the last block.
func (s HeaderFieldSetter) LastCommitSignature(newLastCommitSignature [96]byte) HeaderFieldSetter {
	s.h.SetLastCommitSignature(newLastCommitSignature)
	return s
}

// LastCommitBitmap sets the signatory bitmap of the previous block.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) LastCommitBitmap(newLastCommitBitmap []byte) HeaderFieldSetter {
	s.h.SetLastCommitBitmap(newLastCommitBitmap)
	return s
}

// ShardStateHash sets the shard state hash.
func (s HeaderFieldSetter) ShardStateHash(newShardStateHash common.Hash) HeaderFieldSetter {
	s.h.SetShardStateHash(newShardStateHash)
	return s
}

// Vrf sets the output of the VRF for the epoch.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Vrf(newVrf []byte) HeaderFieldSetter {
	s.h.SetVrf(newVrf)
	return s
}

// Vdf sets the output of the VDF for the epoch.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) Vdf(newVdf []byte) HeaderFieldSetter {
	s.h.SetVdf(newVdf)
	return s
}

// ShardState sets the RLP-encoded form of shard state
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) ShardState(newShardState []byte) HeaderFieldSetter {
	s.h.SetShardState(newShardState)
	return s
}

// CrossLinks sets the RLP-encoded form of non-beacon block headers chosen to be
// canonical by the beacon committee.
//
// It stores a copy; the caller may freely modify the original.
func (s HeaderFieldSetter) CrossLinks(newCrossLinks []byte) HeaderFieldSetter {
	s.h.SetCrossLinks(newCrossLinks)
	return s
}

// Header returns the header whose fields have been set.  Call this at the end
// of a field setter chain.
func (s HeaderFieldSetter) Header() *Header {
	return s.h
}
