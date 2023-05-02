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

// Package rawdb contains a collection of low level database accessors.
package rawdb

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVersionKey tracks the current database version.
	databaseVersionKey = []byte("DatabaseVersion")

	// headHeaderKey tracks the latest known header's hash.
	headHeaderKey = []byte("LastHeader")

	// headBlockKey tracks the latest known full block's hash.
	headBlockKey = []byte("LastBlock")

	// headFastBlockKey tracks the latest known incomplete block's hash during fast sync.
	headFastBlockKey = []byte("LastFast")

	// headFinalizedBlockKey tracks the latest known finalized block hash.
	headFinalizedBlockKey = []byte("LastFinalized")

	// lastPivotKey tracks the last pivot block used by fast sync (to reenable on sethead).
	lastPivotKey = []byte("LastPivot")

	// fastTrieProgressKey tracks the number of trie entries imported during fast sync.
	fastTrieProgressKey = []byte("TrieSync")

	// snapshotDisabledKey flags that the snapshot should not be maintained due to initial sync.
	snapshotDisabledKey = []byte("SnapshotDisabled")

	// SnapshotRootKey tracks the hash of the last snapshot.
	SnapshotRootKey = []byte("SnapshotRoot")

	// snapshotJournalKey tracks the in-memory diff layers across restarts.
	snapshotJournalKey = []byte("SnapshotJournal")

	// snapshotGeneratorKey tracks the snapshot generation marker across restarts.
	snapshotGeneratorKey = []byte("SnapshotGenerator")

	// snapshotRecoveryKey tracks the snapshot recovery marker across restarts.
	snapshotRecoveryKey = []byte("SnapshotRecovery")

	// snapshotSyncStatusKey tracks the snapshot sync status across restarts.
	snapshotSyncStatusKey = []byte("SnapshotSyncStatus")

	// skeletonSyncStatusKey tracks the skeleton sync status across restarts.
	skeletonSyncStatusKey = []byte("SkeletonSyncStatus")

	// txIndexTailKey tracks the oldest block whose transactions have been indexed.
	txIndexTailKey = []byte("TransactionIndexTail")

	// fastTxLookupLimitKey tracks the transaction lookup limit during fast sync.
	fastTxLookupLimitKey = []byte("FastTransactionLookupLimit")

	// badBlockKey tracks the list of bad blocks seen by local
	badBlockKey = []byte("InvalidBlock")

	// uncleanShutdownKey tracks the list of local crashes
	uncleanShutdownKey = []byte("unclean-shutdown") // config prefix for the db

	// transitionStatusKey tracks the eth2 transition status.
	transitionStatusKey = []byte("eth2-transition")

	headerPrefix                 = []byte("h")  // headerPrefix + num (uint64 big endian) + hash -> header
	headerTDSuffix               = []byte("t")  // headerPrefix + num (uint64 big endian) + hash + headerTDSuffix -> td
	headerHashSuffix             = []byte("n")  // headerPrefix + num (uint64 big endian) + headerHashSuffix -> hash
	headerNumberPrefix           = []byte("H")  // headerNumberPrefix + hash -> num (uint64 big endian)
	blockBodyPrefix              = []byte("b")  // blockBodyPrefix + num (uint64 big endian) + hash -> block body
	blockReceiptsPrefix          = []byte("r")  // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts
	txLookupPrefix               = []byte("l")  // txLookupPrefix + hash -> transaction/receipt lookup metadata
	cxLookupPrefix               = []byte("cx") // cxLookupPrefix + hash -> cxReceipt lookup metadata
	bloomBitsPrefix              = []byte("B")  // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits
	shardStatePrefix             = []byte("ss") // shardStatePrefix + num (uint64 big endian) + hash -> shardState
	lastCommitsKey               = []byte("LastCommits")
	blockCommitSigPrefix         = []byte("block-sig-")
	pendingCrosslinkKey          = []byte("pendingCL")        // prefix for shard last pending crosslink
	pendingSlashingKey           = []byte("pendingSC")        // prefix for shard last pending slashing record
	preimagePrefix               = []byte("secure-key-")      // preimagePrefix + hash -> preimage
	continuousBlocksCountKey     = []byte("continuous")       // key for continuous blocks count
	configPrefix                 = []byte("ethereum-config-") // config prefix for the db
	crosslinkPrefix              = []byte("cl")               // prefix for crosslink
	delegatorValidatorListPrefix = []byte("dvl")              // prefix for delegator's validator list
	// TODO: shorten the key prefix so we don't waste db space
	cxReceiptPrefix         = []byte("cxReceipt")          // prefix for cross shard transaction receipt
	cxReceiptSpentPrefix    = []byte("cxReceiptSpent")     // prefix for indicator of unspent of cxReceiptsProof
	validatorSnapshotPrefix = []byte("validator-snapshot") // prefix for staking validator's snapshot information
	validatorStatsPrefix    = []byte("validator-stats")    // prefix for staking validator's stats information
	validatorListKey        = []byte("validator-list")     // key for all validators list
	// epochBlockNumberPrefix + epoch (big.Int.Bytes())
	// -> epoch block number (big.Int.Bytes())
	epochBlockNumberPrefix = []byte("harmony-epoch-block-number")
	// epochVrfBlockNumbersPrefix  + epoch (big.Int.Bytes())
	epochVrfBlockNumbersPrefix = []byte("epoch-vrf-block-numbers")
	// epochVdfBlockNumberPrefix  + epoch (big.Int.Bytes())
	epochVdfBlockNumberPrefix = []byte("epoch-vdf-block-number")
	// Chain index prefixes (use `i` + single byte to avoid mixing data types).
	BloomBitsIndexPrefix        = []byte("iB") // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress
	preimageCounter             = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter          = metrics.NewRegisteredCounter("db/preimage/hits", nil)
	currentRewardGivenOutPrefix = []byte("blk-rwd-")
	// key of SnapdbInfo
	snapdbInfoKey = []byte("SnapdbInfo")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	SnapshotAccountPrefix = []byte("a")  // SnapshotAccountPrefix + account hash -> account trie value
	SnapshotStoragePrefix = []byte("o")  // SnapshotStoragePrefix + account hash + storage hash -> storage trie value
	CodePrefix            = []byte("c")  // CodePrefix + code hash -> account code
	ValidatorCodePrefix   = []byte("vc") // ValidatorCodePrefix + code hash -> validator code
	skeletonHeaderPrefix  = []byte("S")  // skeletonHeaderPrefix + num (uint64 big endian) -> header

	// Path-based trie node scheme.
	trieNodeAccountPrefix = []byte("A") // trieNodeAccountPrefix + hexPath -> trie node
	trieNodeStoragePrefix = []byte("O") // trieNodeStoragePrefix + accountHash + hexPath -> trie node

	PreimagePrefix = []byte("secure-key-")       // PreimagePrefix + hash -> preimage
	genesisPrefix  = []byte("ethereum-genesis-") // genesis state prefix for the db

	ChtPrefix           = []byte("chtRootV2-") // ChtPrefix + chtNum (uint64 big endian) -> trie root hash
	ChtTablePrefix      = []byte("cht-")
	ChtIndexTablePrefix = []byte("chtIndexV2-")

	BloomTriePrefix      = []byte("bltRoot-") // BloomTriePrefix + bloomTrieNum (uint64 big endian) -> trie root hash
	BloomTrieTablePrefix = []byte("blt-")
	BloomTrieIndexPrefix = []byte("bltIndex-")

	CliqueSnapshotPrefix = []byte("clique-")
)

// LegacyTxLookupEntry is the legacy TxLookupEntry definition with some unnecessary
// fields.
type LegacyTxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// headerKeyPrefix = headerPrefix + num (uint64 big endian)
func headerKeyPrefix(number uint64) []byte {
	return append(headerPrefix, encodeBlockNumber(number)...)
}

// accountSnapshotKey = SnapshotAccountPrefix + hash
func accountSnapshotKey(hash common.Hash) []byte {
	return append(SnapshotAccountPrefix, hash.Bytes()...)
}

// storageSnapshotKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotKey(accountHash, storageHash common.Hash) []byte {
	return append(append(SnapshotStoragePrefix, accountHash.Bytes()...), storageHash.Bytes()...)
}

// storageSnapshotsKey = SnapshotStoragePrefix + account hash + storage hash
func storageSnapshotsKey(accountHash common.Hash) []byte {
	return append(SnapshotStoragePrefix, accountHash.Bytes()...)
}

// skeletonHeaderKey = skeletonHeaderPrefix + num (uint64 big endian)
func skeletonHeaderKey(number uint64) []byte {
	return append(skeletonHeaderPrefix, encodeBlockNumber(number)...)
}

// codeKey = CodePrefix + hash
func codeKey(hash common.Hash) []byte {
	return append(CodePrefix, hash.Bytes()...)
}

// IsCodeKey reports whether the given byte slice is the key of contract code,
// if so return the raw code hash as well.
func IsCodeKey(key []byte) (bool, []byte) {
	if bytes.HasPrefix(key, CodePrefix) && len(key) == common.HashLength+len(CodePrefix) {
		return true, key[len(CodePrefix):]
	}
	return false, nil
}

// validatorCodeKey = ValidatorCodePrefix + hash
func validatorCodeKey(hash common.Hash) []byte {
	return append(ValidatorCodePrefix, hash.Bytes()...)
}

// IsValidatorCodeKey reports whether the given byte slice is the key of validator code,
// if so return the raw code hash as well.
func IsValidatorCodeKey(key []byte) (bool, []byte) {
	if bytes.HasPrefix(key, ValidatorCodePrefix) && len(key) == common.HashLength+len(ValidatorCodePrefix) {
		return true, key[len(ValidatorCodePrefix):]
	}
	return false, nil
}

// genesisStateSpecKey = genesisPrefix + hash
func genesisStateSpecKey(hash common.Hash) []byte {
	return append(genesisPrefix, hash.Bytes()...)
}

// accountTrieNodeKey = trieNodeAccountPrefix + nodePath.
func accountTrieNodeKey(path []byte) []byte {
	return append(trieNodeAccountPrefix, path...)
}

// storageTrieNodeKey = trieNodeStoragePrefix + accountHash + nodePath.
func storageTrieNodeKey(accountHash common.Hash, path []byte) []byte {
	return append(append(trieNodeStoragePrefix, accountHash.Bytes()...), path...)
}

// TxLookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxLookupEntry struct {
	BlockHash  common.Hash
	BlockIndex uint64
	Index      uint64
}

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

// decodeBlockNumber decodes a block number as big endian uint64
func decodeBlockNumber(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// headerKey = headerPrefix + num (uint64 big endian) + hash
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// headerTDKey = headerPrefix + num (uint64 big endian) + hash + headerTDSuffix
func headerTDKey(number uint64, hash common.Hash) []byte {
	return append(headerKey(number, hash), headerTDSuffix...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(number uint64) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), headerHashSuffix...)
}

// headerNumberKey = headerNumberPrefix + hash
func headerNumberKey(hash common.Hash) []byte {
	return append(headerNumberPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockBodyKey(number uint64, hash common.Hash) []byte {
	return append(append(blockBodyPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append(blockReceiptsPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

// txLookupKey = txLookupPrefix + hash
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

// cxLookupKey = cxLookupPrefix + hash
func cxLookupKey(hash common.Hash) []byte {
	return append(cxLookupPrefix, hash.Bytes()...)
}

// bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash
func bloomBitsKey(bit uint, section uint64, hash common.Hash) []byte {
	key := append(append(bloomBitsPrefix, make([]byte, 10)...), hash.Bytes()...)

	binary.BigEndian.PutUint16(key[1:], uint16(bit))
	binary.BigEndian.PutUint64(key[3:], section)

	return key
}

// preimageKey = preimagePrefix + hash
func preimageKey(hash common.Hash) []byte {
	return append(preimagePrefix, hash.Bytes()...)
}

func leaderContinuousBlocksCountKey() []byte {
	return continuousBlocksCountKey
}

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}

func shardStateKey(epoch *big.Int) []byte {
	return append(shardStatePrefix, epoch.Bytes()...)
}

func epochBlockNumberKey(epoch *big.Int) []byte {
	return append(epochBlockNumberPrefix, epoch.Bytes()...)
}

func epochVrfBlockNumbersKey(epoch *big.Int) []byte {
	return append(epochVrfBlockNumbersPrefix, epoch.Bytes()...)
}

func epochVdfBlockNumberKey(epoch *big.Int) []byte {
	return append(epochVdfBlockNumberPrefix, epoch.Bytes()...)
}

func shardLastCrosslinkKey(shardID uint32) []byte {
	sbKey := make([]byte, 4)
	binary.BigEndian.PutUint32(sbKey, shardID)
	key := append(crosslinkPrefix, sbKey...)
	return key
}

func crosslinkKey(shardID uint32, blockNum uint64) []byte {
	prefix := crosslinkPrefix
	sbKey := make([]byte, 12)
	binary.BigEndian.PutUint32(sbKey, shardID)
	binary.BigEndian.PutUint64(sbKey[4:], blockNum)
	key := append(prefix, sbKey...)
	return key
}

func delegatorValidatorListKey(delegator common.Address) []byte {
	return append(delegatorValidatorListPrefix, delegator.Bytes()...)
}

// cxReceiptKey = cxReceiptsPrefix + shardID + num (uint64 big endian) + hash
func cxReceiptKey(shardID uint32, number uint64, hash common.Hash) []byte {
	prefix := cxReceiptPrefix
	sKey := make([]byte, 4)
	binary.BigEndian.PutUint32(sKey, shardID)
	tmp := append(prefix, sKey...)
	tmp1 := append(tmp, encodeBlockNumber(number)...)
	return append(tmp1, hash.Bytes()...)
}

// cxReceiptSpentKey = cxReceiptsSpentPrefix + shardID + num (uint64 big endian)
func cxReceiptSpentKey(shardID uint32, number uint64) []byte {
	prefix := cxReceiptSpentPrefix
	sKey := make([]byte, 4)
	binary.BigEndian.PutUint32(sKey, shardID)
	tmp := append(prefix, sKey...)
	return append(tmp, encodeBlockNumber(number)...)
}

func validatorSnapshotKey(addr common.Address, epoch *big.Int) []byte {
	prefix := validatorSnapshotPrefix
	tmp := append(prefix, addr.Bytes()...)
	return append(tmp, epoch.Bytes()...)
}

func validatorStatsKey(addr common.Address) []byte {
	prefix := validatorStatsPrefix
	return append(prefix, addr.Bytes()...)
}

func blockRewardAccumKey(number uint64) []byte {
	return append(currentRewardGivenOutPrefix, encodeBlockNumber(number)...)
}

func blockCommitSigKey(number uint64) []byte {
	return append(blockCommitSigPrefix, encodeBlockNumber(number)...)
}
