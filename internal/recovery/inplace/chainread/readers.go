package chainread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
)

// Get is the strict read primitive: (value, found, error). A missing key is
// (nil, false, nil); any other error propagates and is never treated as
// absence.
func Get(db ethdb.KeyValueReader, key []byte) ([]byte, bool, error) {
	val, err := db.Get(key)
	if err != nil {
		if rodb.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return val, true, nil
}

// ReadCanonicalHash reads the canonical hash for a height.
func ReadCanonicalHash(db ethdb.KeyValueReader, number uint64) (common.Hash, bool, error) {
	raw, found, err := Get(db, HeaderHashKey(number))
	if err != nil || !found {
		return common.Hash{}, found, err
	}
	if len(raw) != common.HashLength {
		return common.Hash{}, true, fmt.Errorf("canonical mapping for height %d has %d bytes, want 32", number, len(raw))
	}
	return common.BytesToHash(raw), true, nil
}

// ReadHeaderNumber reads the reverse hash->number mapping.
func ReadHeaderNumber(db ethdb.KeyValueReader, hash common.Hash) (uint64, bool, error) {
	raw, found, err := Get(db, HeaderNumberKey(hash))
	if err != nil || !found {
		return 0, found, err
	}
	if len(raw) != 8 {
		return 0, true, fmt.Errorf("reverse number mapping for %s has %d bytes, want 8", hash.Hex(), len(raw))
	}
	return binary.BigEndian.Uint64(raw), true, nil
}

// DecodeErr marks bytes that were present but failed to decode - a data
// integrity FAIL, not a read error.
type DecodeErr struct {
	What string
	Err  error
}

func (e *DecodeErr) Error() string { return fmt.Sprintf("%s: undecodable: %v", e.What, e.Err) }

func (e *DecodeErr) Unwrap() error { return e.Err }

// ReadHeader reads and decodes the header stored under (number, hash).
// Returns (nil, true, *DecodeErr) when present but undecodable.
func ReadHeader(db ethdb.KeyValueReader, number uint64, hash common.Hash) (*block.Header, bool, error) {
	raw, found, err := Get(db, HeaderKey(number, hash))
	if err != nil || !found {
		return nil, found, err
	}
	header := new(block.Header)
	if err := rlp.Decode(bytes.NewReader(raw), header); err != nil {
		return nil, true, &DecodeErr{What: fmt.Sprintf("header %d %s", number, hash.Hex()), Err: err}
	}
	return header, true, nil
}

// ReadBody reads and decodes the block body stored under (number, hash).
func ReadBody(db ethdb.KeyValueReader, number uint64, hash common.Hash) (*types.Body, bool, error) {
	raw, found, err := Get(db, BlockBodyKey(number, hash))
	if err != nil || !found {
		return nil, found, err
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(raw), body); err != nil {
		return nil, true, &DecodeErr{What: fmt.Sprintf("body %d %s", number, hash.Hex()), Err: err}
	}
	return body, true, nil
}

// ReadShardStateBytes reads the raw ss<epoch> record.
func ReadShardStateBytes(db ethdb.KeyValueReader, epoch *big.Int) ([]byte, bool, error) {
	return Get(db, ShardStateKey(epoch))
}

// ReadBlockCommitSig reads the exact block-sig-<n> record (no legacy
// LastCommits fallback).
func ReadBlockCommitSig(db ethdb.KeyValueReader, number uint64) ([]byte, bool, error) {
	return Get(db, BlockCommitSigKey(number))
}

// ReadHeadPointer reads a 32-byte head pointer ("LastHeader"/"LastBlock").
func ReadHeadPointer(db ethdb.KeyValueReader, key []byte) (common.Hash, bool, error) {
	raw, found, err := Get(db, key)
	if err != nil || !found {
		return common.Hash{}, found, err
	}
	if len(raw) != common.HashLength {
		return common.Hash{}, true, fmt.Errorf("head pointer %q has %d bytes, want 32", string(key), len(raw))
	}
	return common.BytesToHash(raw), true, nil
}
