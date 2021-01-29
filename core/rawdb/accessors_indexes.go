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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

// ReadTxLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadTxLookupEntry(db DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(txLookupKey(hash))
	if len(data) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid transaction lookup entry RLP")
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// WriteBlockTxLookUpEntries writes all look up entries of block's transactions
func WriteBlockTxLookUpEntries(db DatabaseWriter, block *types.Block) error {
	for i, tx := range block.Transactions() {
		entry := TxLookupEntry{
			BlockHash:  block.Hash(),
			BlockIndex: block.NumberU64(),
			Index:      uint64(i),
		}
		val, err := rlp.EncodeToBytes(entry)
		if err != nil {
			return err
		}
		key := txLookupKey(tx.Hash())
		if err := db.Put(key, val); err != nil {
			return err
		}
		if tx.IsEthCompatible() {
			// Also put a lookup entry for eth transaction's hash
			key := txLookupKey(tx.HashByType())
			if err := db.Put(key, val); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteBlockStxLookUpEntries writes all look up entries of block's staking transactions
func WriteBlockStxLookUpEntries(db DatabaseWriter, block *types.Block) error {
	for i, stx := range block.StakingTransactions() {
		entry := TxLookupEntry{
			BlockHash:  block.Hash(),
			BlockIndex: block.NumberU64(),
			Index:      uint64(i),
		}
		val, err := rlp.EncodeToBytes(entry)
		if err != nil {
			return err
		}
		key := txLookupKey(stx.Hash())
		if err := db.Put(key, val); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTxLookupEntry removes all transaction data associated with a hash.
func DeleteTxLookupEntry(db DatabaseDeleter, hash common.Hash) error {
	return db.Delete(txLookupKey(hash))
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db DatabaseReader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}

	body := ReadBody(db, blockHash, blockNumber)
	if body == nil {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", txIndex).
			Msg("block Body referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	tx := body.TransactionAt(int(txIndex))
	missing := false
	if tx == nil {
		missing = true
	} else {
		hmyHash := tx.Hash()
		ethHash := tx.HashByType()

		if !bytes.Equal(hash.Bytes(), hmyHash.Bytes()) && !bytes.Equal(hash.Bytes(), ethHash.Bytes()) {
			missing = true
		}

	}

	if missing {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", txIndex).
			Msg("Transaction referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	return tx, blockHash, blockNumber, txIndex
}

// ReadStakingTransaction retrieves a specific staking transaction from the database, along with
// its added positional metadata.
func ReadStakingTransaction(db DatabaseReader, hash common.Hash) (*staking.StakingTransaction, common.Hash, uint64, uint64) {
	blockHash, blockNumber, txIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, blockNumber)
	if body == nil {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", txIndex).
			Msg("block Body referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	tx := body.StakingTransactionAt(int(txIndex))
	if tx == nil || !bytes.Equal(hash.Bytes(), tx.Hash().Bytes()) {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", txIndex).
			Msg("StakingTransaction referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	return tx, blockHash, blockNumber, txIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func ReadReceipt(db DatabaseReader, hash common.Hash) (*types.Receipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, receiptIndex := ReadTxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}

	receipts := ReadReceipts(db, blockHash, blockNumber)
	if len(receipts) <= int(receiptIndex) {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", receiptIndex).
			Msg("Receipt refereced missing")
		return nil, common.Hash{}, 0, 0
	}
	return receipts[receiptIndex], blockHash, blockNumber, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db DatabaseReader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	return db.Get(bloomBitsKey(bit, section, head))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db DatabaseWriter, bit uint, section uint64, head common.Hash, bits []byte) error {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to store bloom bits")
		return err
	}
	return nil
}

// ReadCxLookupEntry retrieves the positional metadata associated with a transaction hash
// to allow retrieving cross shard receipt by hash in destination shard
// not the original transaction in source shard
// return nil if not found
func ReadCxLookupEntry(db DatabaseReader, hash common.Hash) (common.Hash, uint64, uint64) {
	data, _ := db.Get(cxLookupKey(hash))
	if len(data) == 0 {
		return common.Hash{}, 0, 0
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		utils.Logger().Error().Err(err).Str("hash", hash.Hex()).Msg("Invalid transaction lookup entry RLP")
		return common.Hash{}, 0, 0
	}
	return entry.BlockHash, entry.BlockIndex, entry.Index
}

// WriteCxLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteCxLookupEntries(db DatabaseWriter, block *types.Block) error {
	previousSum := 0
	for _, cxp := range block.IncomingReceipts() {
		for j, cx := range cxp.Receipts {
			entry := TxLookupEntry{
				BlockHash:  block.Hash(),
				BlockIndex: block.NumberU64(),
				Index:      uint64(j + previousSum),
			}
			data, err := rlp.EncodeToBytes(entry)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to encode transaction lookup entry")
				return err
			}
			if err := db.Put(cxLookupKey(cx.TxHash), data); err != nil {
				utils.Logger().Error().Err(err).Msg("Failed to store transaction lookup entry")
				return err
			}
		}
		previousSum += len(cxp.Receipts)
	}
	return nil
}

// DeleteCxLookupEntry removes all transaction data associated with a hash.
func DeleteCxLookupEntry(db DatabaseDeleter, hash common.Hash) error {
	return db.Delete(cxLookupKey(hash))
}

// ReadCXReceipt retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadCXReceipt(db DatabaseReader, hash common.Hash) (*types.CXReceipt, common.Hash, uint64, uint64) {
	blockHash, blockNumber, cxIndex := ReadCxLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0, 0
	}
	body := ReadBody(db, blockHash, blockNumber)
	if body == nil {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", cxIndex).
			Msg("block Body referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	cx := body.CXReceiptAt(int(cxIndex))
	if cx == nil {
		utils.Logger().Error().
			Uint64("number", blockNumber).
			Str("hash", blockHash.Hex()).
			Uint64("index", cxIndex).
			Msg("CXReceipt referenced missing")
		return nil, common.Hash{}, 0, 0
	}
	return cx, blockHash, blockNumber, cxIndex
}
