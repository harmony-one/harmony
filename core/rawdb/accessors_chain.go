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
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

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
func WriteCanonicalHash(db DatabaseWriter, hash common.Hash, number uint64) {
	if err := db.Put(headerHashKey(number), hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store number to hash mapping")
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db DatabaseDeleter, number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		utils.Logger().Error().Msg("Failed to delete number to hash mapping")
	}
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
func WriteHeadHeaderHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last header's hash")
	}
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
func WriteHeadBlockHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last block's hash")
	}
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
func WriteHeadFastBlockHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headFastBlockKey, hash.Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store last fast block's hash")
	}
}

// ReadFastTrieProgress retrieves the number of tries nodes fast synced to allow
// reporting correct numbers across restarts.
func ReadFastTrieProgress(db DatabaseReader) uint64 {
	data, _ := db.Get(fastTrieProgressKey)
	if len(data) == 0 {
		return 0
	}
	return new(big.Int).SetBytes(data).Uint64()
}

// WriteFastTrieProgress stores the fast sync trie process counter to support
// retrieving it across restarts.
func WriteFastTrieProgress(db DatabaseWriter, count uint64) {
	if err := db.Put(fastTrieProgressKey, new(big.Int).SetUint64(count).Bytes()); err != nil {
		utils.Logger().Error().Msg("Failed to store fast sync trie progress")
	}
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
func WriteHeader(db DatabaseWriter, header *block.Header) {
	// Write the hash -> number mapping
	var (
		hash    = header.Hash()
		number  = header.Number.Uint64()
		encoded = encodeBlockNumber(number)
	)
	key := headerNumberKey(hash)
	if err := db.Put(key, encoded); err != nil {
		utils.Logger().Error().Msg("Failed to store hash to number mapping")
	}
	// Write the encoded header
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode header")
	}
	key = headerKey(number, hash)
	if err := db.Put(key, data); err != nil {
		utils.Logger().Error().Msg("Failed to store header")
	}
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteHeader(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(headerKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete header")
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete hash to number mapping")
	}
}

// ReadBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func ReadBodyRLP(db DatabaseReader, hash common.Hash, number uint64) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(number, hash))
	return data
}

// WriteBodyRLP stores an RLP encoded block body into the database.
func WriteBodyRLP(db DatabaseWriter, hash common.Hash, number uint64, rlp rlp.RawValue) {
	if err := db.Put(blockBodyKey(number, hash), rlp); err != nil {
		utils.Logger().Error().Msg("Failed to store block body")
	}
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
func WriteBody(db DatabaseWriter, hash common.Hash, number uint64, body *types.Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode body")
	}
	WriteBodyRLP(db, hash, number, data)
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBody(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(blockBodyKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block body")
	}
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
func WriteTd(db DatabaseWriter, hash common.Hash, number uint64, td *big.Int) {
	data, err := rlp.EncodeToBytes(td)
	if err != nil {
		utils.Logger().Error().Msg("Failed to RLP encode block total difficulty")
	}
	if err := db.Put(headerTDKey(number, hash), data); err != nil {
		utils.Logger().Error().Msg("Failed to store block total difficulty")
	}
}

// DeleteTd removes all block total difficulty data associated with a hash.
func DeleteTd(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(headerTDKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block total difficulty")
	}
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
func WriteReceipts(db DatabaseWriter, hash common.Hash, number uint64, receipts types.Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		utils.Logger().Error().Msg("Failed to encode block receipts")
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(number, hash), bytes); err != nil {
		utils.Logger().Error().Msg("Failed to store block receipts")
	}
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceipts(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(blockReceiptsKey(number, hash)); err != nil {
		utils.Logger().Error().Msg("Failed to delete block receipts")
	}
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
	return types.NewBlockWithHeader(header).WithBody(body.Transactions, body.Uncles, body.IncomingReceipts)
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteBlock(db DatabaseWriter, block *types.Block) {
	WriteBody(db, block.Hash(), block.NumberU64(), block.Body())
	WriteHeader(db, block.Header())
	// TODO ek – maybe roll the below into WriteHeader()
	epoch := block.Header().Epoch
	if epoch == nil {
		// backward compatibility
		return
	}
	epoch = new(big.Int).Set(epoch)
	epochBlockNum := block.Number()
	writeOne := func() {
		if err := WriteEpochBlockNumber(db, epoch, epochBlockNum); err != nil {
			utils.Logger().Error().Err(err).Msg("Failed to write epoch block number")
		}
	}
	// A block may be a genesis block AND end-of-epoch block at the same time.
	if epochBlockNum.Sign() == 0 {
		// Genesis block; record this block's epoch and block numbers.
		writeOne()
	}

	// TODO: don't change epoch based on shard state presence
	if len(block.Header().ShardState) > 0 && block.NumberU64() != 0 {
		// End-of-epoch block; record the next epoch after this block.
		epoch = new(big.Int).Add(epoch, common.Big1)
		epochBlockNum = new(big.Int).Add(epochBlockNum, common.Big1)
		writeOne()
	}
}

// DeleteBlock removes all block data associated with a hash.
func DeleteBlock(db DatabaseDeleter, hash common.Hash, number uint64) {
	DeleteReceipts(db, hash, number)
	DeleteHeader(db, hash, number)
	DeleteBody(db, hash, number)
	DeleteTd(db, hash, number)
}

// FindCommonAncestor returns the last common ancestor of two block headers
func FindCommonAncestor(db DatabaseReader, a, b *block.Header) *block.Header {
	for bn := b.Number.Uint64(); a.Number.Uint64() > bn; {
		a = ReadHeader(db, a.ParentHash, a.Number.Uint64()-1)
		if a == nil {
			return nil
		}
	}
	for an := a.Number.Uint64(); an < b.Number.Uint64(); {
		b = ReadHeader(db, b.ParentHash, b.Number.Uint64()-1)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadHeader(db, a.ParentHash, a.Number.Uint64()-1)
		if a == nil {
			return nil
		}
		b = ReadHeader(db, b.ParentHash, b.Number.Uint64()-1)
		if b == nil {
			return nil
		}
	}
	return a
}

// ReadShardState retrieves sharding state.
func ReadShardState(
	db DatabaseReader, epoch *big.Int,
) (shardState shard.ShardState, err error) {
	var data []byte
	data, err = db.Get(shardStateKey(epoch))
	if err != nil {
		return nil, ctxerror.New("cannot read sharding state from rawdb",
			"epoch", epoch,
		).WithCause(err)
	}
	if err = rlp.DecodeBytes(data, &shardState); err != nil {
		return nil, ctxerror.New("cannot decode sharding state",
			"epoch", epoch,
		).WithCause(err)
	}
	return shardState, nil
}

// WriteShardState stores sharding state into database.
func WriteShardState(
	db DatabaseWriter, epoch *big.Int, shardState shard.ShardState,
) (err error) {
	data, err := rlp.EncodeToBytes(shardState)
	if err != nil {
		return ctxerror.New("cannot encode sharding state",
			"epoch", epoch,
		).WithCause(err)
	}
	return WriteShardStateBytes(db, epoch, data)
}

// WriteShardStateBytes stores sharding state into database.
func WriteShardStateBytes(
	db DatabaseWriter, epoch *big.Int, data []byte,
) (err error) {
	if err = db.Put(shardStateKey(epoch), data); err != nil {
		return ctxerror.New("cannot write sharding state",
			"epoch", epoch,
		).WithCause(err)
	}
	utils.Logger().Info().Str("epoch", epoch.String()).Int("numShards", len(data)).Msg("wrote sharding state")
	return nil
}

// ReadLastCommits retrieves LastCommits.
func ReadLastCommits(db DatabaseReader) ([]byte, error) {
	var data []byte
	data, err := db.Get(lastCommitsKey)
	if err != nil {
		return nil, ctxerror.New("cannot read last commits from rawdb").WithCause(err)
	}
	return data, nil
}

// WriteLastCommits stores last commits into database.
func WriteLastCommits(
	db DatabaseWriter, data []byte,
) (err error) {
	if err = db.Put(lastCommitsKey, data); err != nil {
		return ctxerror.New("cannot write last commits").WithCause(err)
	}
	return nil
}

// ReadEpochBlockNumber retrieves the epoch block number for the given epoch,
// or nil if the given epoch is not found in the database.
func ReadEpochBlockNumber(db DatabaseReader, epoch *big.Int) (*big.Int, error) {
	data, err := db.Get(epochBlockNumberKey(epoch))
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(data), nil
}

// WriteEpochBlockNumber stores the given epoch-number-to-epoch-block-number in the database.
func WriteEpochBlockNumber(db DatabaseWriter, epoch, blockNum *big.Int) error {
	return db.Put(epochBlockNumberKey(epoch), blockNum.Bytes())
}

// ReadEpochVrfBlockNums retrieves the VRF block numbers for the given epoch
func ReadEpochVrfBlockNums(db DatabaseReader, epoch *big.Int) ([]byte, error) {
	return db.Get(epochVrfBlockNumbersKey(epoch))
}

// WriteEpochVrfBlockNums stores the VRF block numbers for the given epoch
func WriteEpochVrfBlockNums(db DatabaseWriter, epoch *big.Int, data []byte) error {
	return db.Put(epochVrfBlockNumbersKey(epoch), data)
}

// ReadEpochVdfBlockNum retrieves the VDF block number for the given epoch
func ReadEpochVdfBlockNum(db DatabaseReader, epoch *big.Int) ([]byte, error) {
	return db.Get(epochVdfBlockNumberKey(epoch))
}

// WriteEpochVdfBlockNum stores the VDF block number for the given epoch
func WriteEpochVdfBlockNum(db DatabaseWriter, epoch *big.Int, data []byte) error {
	return db.Put(epochVdfBlockNumberKey(epoch), data)
}

// ReadCrossLinkShardBlock retrieves the blockHash given shardID and blockNum
func ReadCrossLinkShardBlock(db DatabaseReader, shardID uint32, blockNum uint64, temp bool) ([]byte, error) {
	return db.Get(crosslinkKey(shardID, blockNum, temp))
}

// WriteCrossLinkShardBlock stores the blockHash given shardID and blockNum
func WriteCrossLinkShardBlock(db DatabaseWriter, shardID uint32, blockNum uint64, data []byte, temp bool) error {
	return db.Put(crosslinkKey(shardID, blockNum, temp), data)
}

// DeleteCrossLinkShardBlock deletes the blockHash given shardID and blockNum
func DeleteCrossLinkShardBlock(db DatabaseDeleter, shardID uint32, blockNum uint64, temp bool) error {
	return db.Delete(crosslinkKey(shardID, blockNum, temp))
}

// ReadShardLastCrossLink read the last cross link of a shard
func ReadShardLastCrossLink(db DatabaseReader, shardID uint32) ([]byte, error) {
	return db.Get(shardLastCrosslinkKey(shardID))
}

// WriteShardLastCrossLink stores the last cross link of a shard
func WriteShardLastCrossLink(db DatabaseWriter, shardID uint32, data []byte) error {
	return db.Put(shardLastCrosslinkKey(shardID), data)
}

// ReadCXReceipts retrieves all the transactions of receipts given destination shardID, number and blockHash
func ReadCXReceipts(db DatabaseReader, shardID uint32, number uint64, hash common.Hash, temp bool) (types.CXReceipts, error) {
	data, err := db.Get(cxReceiptKey(shardID, number, hash, temp))
	if len(data) == 0 || err != nil {
		utils.Logger().Info().Err(err).Uint64("number", number).Int("dataLen", len(data)).Msg("ReadCXReceipts")
		return nil, err
	}
	cxReceipts := types.CXReceipts{}
	if err := rlp.DecodeBytes(data, &cxReceipts); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid cross-shard tx receipt array RLP")
		return nil, err
	}
	return cxReceipts, nil
}

// WriteCXReceipts stores all the transaction receipts given destination shardID, blockNumber and blockHash
func WriteCXReceipts(db DatabaseWriter, shardID uint32, number uint64, hash common.Hash, receipts types.CXReceipts, temp bool) error {
	bytes, err := rlp.EncodeToBytes(receipts)
	if err != nil {
		utils.Logger().Error().Msg("[WriteCXReceipts] Failed to encode cross shard tx receipts")
	}
	// Store the receipt slice
	if err := db.Put(cxReceiptKey(shardID, number, hash, temp), bytes); err != nil {
		utils.Logger().Error().Msg("[WriteCXReceipts] Failed to store cxreceipts")
	}
	return err
}

// DeleteCXReceipts removes all receipt data associated with a block hash.
func DeleteCXReceipts(db DatabaseDeleter, shardID uint32, number uint64, hash common.Hash, temp bool) {
	if err := db.Delete(cxReceiptKey(shardID, number, hash, temp)); err != nil {
		utils.Logger().Error().Msg("Failed to delete cross shard tx receipts")
	}
}

// ReadCXReceiptsProofSpent check whether a CXReceiptsProof is unspent
func ReadCXReceiptsProofSpent(db DatabaseReader, shardID uint32, number uint64) (byte, error) {
	data, err := db.Get(cxReceiptSpentKey(shardID, number))
	if err != nil || len(data) == 0 {
		return NAByte, ctxerror.New("[ReadCXReceiptsProofSpent] Cannot find the key", "shardID", shardID, "number", number).WithCause(err)
	}
	return data[0], nil
}

// WriteCXReceiptsProofSpent write CXReceiptsProof as spent into database
func WriteCXReceiptsProofSpent(dbw DatabaseWriter, cxp *types.CXReceiptsProof) error {
	shardID := cxp.MerkleProof.ShardID
	blockNum := cxp.MerkleProof.BlockNum.Uint64()
	return dbw.Put(cxReceiptSpentKey(shardID, blockNum), []byte{SpentByte})
}

// DeleteCXReceiptsProofSpent removes unspent indicator of a given blockHash
func DeleteCXReceiptsProofSpent(db DatabaseDeleter, shardID uint32, number uint64) {
	if err := db.Delete(cxReceiptSpentKey(shardID, number)); err != nil {
		utils.Logger().Error().Msg("Failed to delete receipts unspent indicator")
	}
}

// ReadCXReceiptsProofUnspentCheckpoint returns the last unspent blocknumber
func ReadCXReceiptsProofUnspentCheckpoint(db DatabaseReader, shardID uint32) (uint64, error) {
	by, err := db.Get(cxReceiptUnspentCheckpointKey(shardID))
	if err != nil {
		return 0, ctxerror.New("[ReadCXReceiptsProofUnspent] Cannot Unspent Checkpoint", "shardID", shardID).WithCause(err)
	}
	lastCheckpoint := binary.BigEndian.Uint64(by[:])
	return lastCheckpoint, nil
}

// WriteCXReceiptsProofUnspentCheckpoint check whether a CXReceiptsProof is unspent, true means not spent
func WriteCXReceiptsProofUnspentCheckpoint(db DatabaseWriter, shardID uint32, blockNum uint64) error {
	by := make([]byte, 8)
	binary.BigEndian.PutUint64(by[:], blockNum)
	return db.Put(cxReceiptUnspentCheckpointKey(shardID), by)
}
