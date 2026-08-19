// Package chainread provides strict raw readers over the exact rawdb key
// schema plus the minimal fail-closed engine.ChainReader used for
// certificate verification. Stock rawdb readers swallow read errors into
// "absence"; on a live DB transient errors are expected, so every reader
// here propagates errors and distinguishes them from genuine absence.
package chainread

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Exact raw keys, re-derived from core/rawdb/schema.go (the builders there
// are unexported). TestKeySchemaAgainstRawdb pins byte-equality with what
// the production rawdb writers put on disk.

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// HeaderHashKey is the canonical mapping: "h" + BE64(number) + "n" -> hash.
func HeaderHashKey(number uint64) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), 'n')
}

// HeaderNumberKey is the reverse mapping: "H" + hash -> BE64(number).
func HeaderNumberKey(hash common.Hash) []byte {
	return append([]byte("H"), hash.Bytes()...)
}

// HeaderKey: "h" + BE64(number) + hash -> header RLP.
func HeaderKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("h"), encodeBlockNumber(number)...), hash.Bytes()...)
}

// BlockBodyKey: "b" + BE64(number) + hash -> body RLP.
func BlockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append([]byte("b"), encodeBlockNumber(number)...), hash.Bytes()...)
}

// BlockCommitSigKey: "block-sig-" + BE64(number) -> 96-byte BLS aggregate
// signature followed by the bitmap. The legacy "LastCommits" fallback inside
// rawdb.ReadBlockCommitSig is deliberately not consulted: the preflight
// reads exact keys only.
func BlockCommitSigKey(number uint64) []byte {
	return append([]byte("block-sig-"), encodeBlockNumber(number)...)
}

// ShardStateKey: "ss" + epoch.Bytes() -> shard state RLP (the boundary
// header's raw ShardState() bytes, stored unmodified by all three
// production write sites).
func ShardStateKey(epoch *big.Int) []byte {
	return append([]byte("ss"), epoch.Bytes()...)
}

// HeadHeaderKey is the "LastHeader" head pointer (informational only).
var HeadHeaderKey = []byte("LastHeader")

// HeadBlockKey is the "LastBlock" head pointer (informational only).
var HeadBlockKey = []byte("LastBlock")
