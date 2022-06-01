// Copyright 2018 The go-ethereum Authors
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

package rawdb

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// MsgNoShardStateFromDB error message for shard state reading failure
var MsgNoShardStateFromDB = "failed to read shard state from DB"

// Indicate whether the receipts corresponding to a blockHash is spent or not
const (
	SpentByte byte = iota
	UnspentByte
	NAByte // not exist
)

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db DatabaseReader, number uint64) common.Hash {
	data, _ := db.Get(headerHashKey(number))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db DatabaseWriter, hash common.Hash, number uint64) error {
	if err := db.Put(headerHashKey(number), hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store number to hash mapping")
		return err
	}
	return nil
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db DatabaseDeleter, number uint64) error {
	if err := db.Delete(headerHashKey(number)); err != nil {
		utils.Logger().Error().Msg("Failed to delete number to hash mapping")
		return err
	}
	return nil
}

// ReadHeaderNumber returns the header number assigned to a hash.
func ReadHeaderNumber(db DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerNumberKey(hash))
	if len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadHeadHeaderHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func WriteHeadHeaderHash(db DatabaseWriter, hash common.Hash) error {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last header's hash")
		return err
	}
	return nil
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db DatabaseWriter, hash common.Hash) error {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last block's hash")
		return err
	}
	return nil
}

// ReadHeadFastBlockHash retrieves the hash of the current fast-sync head block.
func ReadHeadFastBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headFastBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadFastBlockHash stores the hash of the current fast-sync head block.
func WriteHeadFastBlockHash(db DatabaseWriter, hash common.Hash) error {
	if err := db.Put(headFastBlockKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last fast block's hash")
		return err
	}
	return nil
}

// ReadHeaderRLP retrieves a block header in its raw RLP database encoding.
func ReadHeaderRLP(db DatabaseReader, hash common.Hash, number uint64) rlp.RawValue {
	data, _ := db.Get(headerKey(number, hash))
	return data
}

// HasHeader verifies the existence of a block header corresponding to the hash.
func HasHeader(db DatabaseReader, hash common.Hash, number uint64) bool {
	if has, err := db.Has(headerKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadHeader(db DatabaseReader, hash common.Hash, number uint64) *block.Header {
	data := ReadHeaderRLP(db, hash, number)
	if len(data) == 0 {
		return nil
	}
	header := new(block.Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid block header RLP")
		return nil
	}
	return header
}

// WriteHeader stores a block header into the database and also stores the hash-
// to-number mapping.
func WriteHeader(db DatabaseWriter, header *block.Header) error {
	// Write the hash -> number mapping
	var (
		hash    = header.Hash()
		number  = header.Number().Uint64()
		encoded = encodeBlockNumber(number)
	)
	key := headerNumberKey(hash)
	if err := db.Put(key, encoded); err != nil {
		utils.Logger().Error().Msg("Failed to store hash to number mapping")
		return err
	}
	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode header")
		return err
	}
	key = headerKey(number, hash)
	if err := db.Put(key, data); err != nil {
		utils.Logger().Error().Msg("Failed to store header")
		return err
	}
	return nil
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteHeader(db DatabaseDeleter, hash common.Hash, number uint64) error {
	if err := db.Delete(headerKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete header")
		return err
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete hash to number mapping")
		return err
	}
	return nil
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func ReadBodyRLP(db DatabaseReader, hash common.Hash, number uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(number, hash))
	return data
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func WriteBodyRLP(db DatabaseWriter, hash common.Hash, number uint64, rlp rlp.RawValue) error {
	if err := db.Put(blockBodyKey(number, hash), rlp); err != nil {
		utils.Logger().Error().Msg("Failed to store block body")
		return err
	}
	return nil
}

// HasBody verifies the existence of a block body corresponding to the hash.
func HasBody(db DatabaseReader, hash common.Hash, number uint64) bool {
	if has, err := db.Has(blockBodyKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadBody retrieves the block body corresponding to the hash.
func ReadBody(db DatabaseReader, hash common.Hash, number uint64) *types.Body {
	data := ReadBodyRLP(db, hash, number)
	if len(data) == 0 {
		return nil
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid block body RLP")
		return nil
	}
	return body
}

// WriteBody storea a block body into the database.
func WriteBody(db DatabaseWriter, hash common.Hash, number uint64, body *types.Body) error {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode body")
		return err
	}
	return WriteBodyRLP(db, hash, number, data)
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBody(db DatabaseDeleter, hash common.Hash, number uint64) error {
	if err := db.Delete(blockBodyKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block body")
		return err
	}
	return nil
}

// ReadTd retrieves a block's total difficulty corresponding to the hash.
func ReadTd(db DatabaseReader, hash common.Hash, number uint64) *big.Int {
	data, _ := db.Get(headerTDKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	td := new(big.Int)
	if err := rlp.Decode(bytes.NewReader(data), td); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid block total difficulty RLP")
		return nil
	}
	return td
}

// WriteTd stores the total difficulty of a block into the database.
func WriteTd(db DatabaseWriter, hash common.Hash, number uint64, td *big.Int) error {
	data, err := rlp.EncodeToBytes(td)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode block total difficulty")
		return err
	}
	if err := db.Put(headerTDKey(number, hash), data); err != nil {
		utils.Logger().Error().Msg("Failed to store block total difficulty")
		return err
	}
	return nil
}

// DeleteTd removes all block total difficulty data associated with a hash.
func DeleteTd(db DatabaseDeleter, hash common.Hash, number uint64) error {
	if err := db.Delete(headerTDKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block total difficulty")
		return err
	}
	return nil
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func ReadReceipts(db DatabaseReader, hash common.Hash, number uint64) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	// Convert the receipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid receipt array RLP")
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func WriteReceipts(db DatabaseWriter, hash common.Hash, number uint64, receipts types.Receipts) error {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		utils.Logger().Error().Msg("Failed to encode block receipts")
		return err
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(number, hash), bytes); err != nil {
		utils.Logger().Error().Msg("Failed to store block receipts")
		return err
	}
	return nil
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceipts(db DatabaseDeleter, hash common.Hash, number uint64) error {
	if err := db.Delete(blockReceiptsKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block receipts")
		return err
	}
	return nil
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func ReadBlock(db DatabaseReader, hash common.Hash, number uint64) *types.Block {
	header := ReadHeader(db, hash, number)
	if header == nil {
		return nil
	}
	body := ReadBody(db, hash, number)
	if body == nil {
		return nil
	}
	return types.NewBlockWithHeader(header).WithBody(body.Transactions(), body.StakingTransactions(), body.Uncles(), body.IncomingReceipts())
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteBlock(db DatabaseWriter, block *types.Block) error {
	if err := WriteBody(db, block.Hash(), block.NumberU64(), block.Body()); err != nil {
		return err
	}
	if err := WriteHeader(db, block.Header()); err != nil {
		return err
	}

	curSig := block.GetCurrentCommitSig()
	if len(curSig) > 96 {
		if err := WriteBlockCommitSig(db, block.NumberU64(), curSig); err != nil {
			return err
		}
	}
	return nil
}

// DeleteBlock removes all block data associated with a hash.
func DeleteBlock(db DatabaseDeleter, hash common.Hash, number uint64) error {
	if err := DeleteReceipts(db, hash, number); err != nil {
		return err
	}
	if err := DeleteHeader(db, hash, number); err != nil {
		return err
	}
	if err := DeleteBody(db, hash, number); err != nil {
		return err
	}
	if err := DeleteTd(db, hash, number); err != nil {
		return err
	}
	return nil
}

// FindCommonAncestor returns the last common ancestor of two block headers
func FindCommonAncestor(db DatabaseReader, a, b *block.Header) *block.Header {
	for bn := b.Number().Uint64(); a.Number().Uint64() > bn; {
		a = ReadHeader(db, a.ParentHash(), a.Number().Uint64()-1)
		if a == nil {
			return nil
		}
	}
	for an := a.Number().Uint64(); an < b.Number().Uint64(); {
		b = ReadHeader(db, b.ParentHash(), b.Number().Uint64()-1)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadHeader(db, a.ParentHash(), a.Number().Uint64()-1)
		if a == nil {
			return nil
		}
		b = ReadHeader(db, b.ParentHash(), b.Number().Uint64()-1)
		if b == nil {
			return nil
		}
	}
	return a
}

func IteratorBlocks(iterator DatabaseIterator, cb func(blockNum uint64, hash common.Hash) bool) (minKey []byte, maxKey []byte) {
	iter := iterator.NewIteratorWithPrefix(headerPrefix)
	defer iter.Release()

	minKey = headerPrefix
	for iter.Next() {
		// headerKey = headerPrefix + num (uint64 big endian) + hash
		key := iter.Key()
		if len(key) != len(headerPrefix)+8+32 {
			continue
		}

		maxKey = key
		blockNum := decodeBlockNumber(key[len(headerPrefix) : len(headerPrefix)+8])
		hash := common.BytesToHash(key[len(headerPrefix)+8:])

		if !cb(blockNum, hash) {
			return
		}
	}

	return
}
