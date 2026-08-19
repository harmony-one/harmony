// Package keys mirrors the raw database key schema of core/rawdb (which keeps
// most of its prefixes unexported) for the recovery tools' raw keyspace scans,
// deletions, and digest passes. A pinning test (keys_test.go) writes records
// through the stock rawdb accessors and asserts they land under these exact
// keys, so any upstream schema drift fails loudly.
//
// It also implements the longest-prefix, key-shape-aware classifier used by
// inventory-db, the logical KV digest, and verify-db's raw scans (plan WS2,
// §2.2.9): longer prefixes win (cxReceiptSpent > cxReceipt > cx > c; cl > c;
// vc over bare 32-byte), and ALL un-prefixed 32-byte keys land in the single
// physical bare-hash32 bucket — no per-namespace semantic split of that
// keyspace is ever claimed.
package keys

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Fixed singleton keys (core/rawdb/schema.go).
var (
	DatabaseVersionKey = []byte("DatabaseVersion")
	HeadHeaderKey      = []byte("LastHeader")
	HeadBlockKey       = []byte("LastBlock")
	HeadFastBlockKey   = []byte("LastFast")
	HeadFinalizedKey   = []byte("LastFinalized")
	LastPivotKey       = []byte("LastPivot")
	TrieSyncKey        = []byte("TrieSync")

	SnapshotDisabledKey   = []byte("SnapshotDisabled")
	SnapshotRootKey       = []byte("SnapshotRoot")
	SnapshotJournalKey    = []byte("SnapshotJournal")
	SnapshotGeneratorKey  = []byte("SnapshotGenerator")
	SnapshotRecoveryKey   = []byte("SnapshotRecovery")
	SnapshotSyncStatusKey = []byte("SnapshotSyncStatus")
	SkeletonSyncStatusKey = []byte("SkeletonSyncStatus")

	TxIndexTailKey       = []byte("TransactionIndexTail")
	FastTxLookupLimitKey = []byte("FastTransactionLookupLimit")
	BadBlockKey          = []byte("InvalidBlock")
	UncleanShutdownKey   = []byte("unclean-shutdown")
	Eth2TransitionKey    = []byte("eth2-transition")
	LastCommitsKey       = []byte("LastCommits")
	ContinuousKey        = []byte("continuous")
	SnapdbInfoKey        = []byte("SnapdbInfo")
	ValidatorListKey     = []byte("validator-list")

	PreimageImportKey   = []byte("preimage-import")
	PreimageGenStartKey = []byte("preimage-gen-start")
	PreimageGenEndKey   = []byte("preimage-gen-end")

	PendingCrosslinkKey = []byte("pendingCL")
	PendingSlashingKey  = []byte("pendingSC")
)

// Prefixes (core/rawdb/schema.go).
var (
	HeaderPrefix            = []byte("h") // h + num8 + hash32 -> header; h + num8 + 'n' -> canonical hash; h + num8 + hash32 + 't' -> TD
	HeaderNumberPrefix      = []byte("H") // H + hash32 -> num8
	BodyPrefix              = []byte("b") // b + num8 + hash32 -> body
	ReceiptsPrefix          = []byte("r") // r + num8 + hash32 -> receipts
	TxLookupPrefix          = []byte("l") // l + hash32 -> lookup entry
	CxLookupPrefix          = []byte("cx")
	BloomBitsPrefix         = []byte("B") // B + bit2 + section8 + hash32
	ShardStatePrefix        = []byte("ss")
	BlockSigPrefix          = []byte("block-sig-")
	PreimagePrefix          = []byte("secure-key-")
	ConfigPrefix            = []byte("ethereum-config-")
	GenesisPrefix           = []byte("ethereum-genesis-")
	CrosslinkPrefix         = []byte("cl") // cl + shard4 -> shard-last; cl + shard4 + num8 -> crosslink index
	DVLPrefix               = []byte("dvl")
	CxReceiptPrefix         = []byte("cxReceipt")      // + shard4 + num8 + hash32
	CxSpentPrefix           = []byte("cxReceiptSpent") // + shard4 + num8
	ValidatorSnapshotPrefix = []byte("validator-snapshot")
	ValidatorStatsPrefix    = []byte("validator-stats")
	EpochBlockNumberPrefix  = []byte("harmony-epoch-block-number")
	EpochVrfPrefix          = []byte("epoch-vrf-block-numbers")
	EpochVdfPrefix          = []byte("epoch-vdf-block-number")
	BloomBitsIndexPrefix    = []byte("iB")
	RewardAccumPrefix       = []byte("blk-rwd-")
	CodePrefix              = []byte("c")
	ValidatorCodePrefix     = []byte("vc")
	SkeletonHeaderPrefix    = []byte("S") // S + num8
	TrieNodeAccountPrefix   = []byte("A") // path scheme (not used by harmony hash-scheme DBs)
	TrieNodeStoragePrefix   = []byte("O")
	SnapshotAccountPrefix   = []byte("a")
	SnapshotStoragePrefix   = []byte("o")

	ChtPrefix            = []byte("chtRootV2-")
	ChtTablePrefix       = []byte("cht-")
	ChtIndexTablePrefix  = []byte("chtIndexV2-")
	BloomTriePrefix      = []byte("bltRoot-")
	BloomTrieTablePrefix = []byte("blt-")
	BloomTrieIndexPrefix = []byte("bltIndex-")
	CliqueSnapshotPrefix = []byte("clique-")
)

// RecoveryMarkerKey is the recovery-completion marker key written by
// compact-db (plan §2.2.4). The 0xff lead byte collides with no stock ASCII
// prefix and cannot be a hash-scheme trie node key (those are exactly 32
// bytes; this key is not). The key is inert to stock binaries: nothing in the
// stock tree reads or iterates it. The exact key/schema is this plan's
// documented schema until the in-place effort adopts the tool (plan §8).
var RecoveryMarkerKey = append([]byte{0xff}, []byte("hmy-recovery-complete-v1")...)

// Uint64BE encodes a block number the way the schema does.
func Uint64BE(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// CanonicalHashKey = h + num8 + 'n'.
func CanonicalHashKey(number uint64) []byte {
	return append(append(append([]byte{}, HeaderPrefix...), Uint64BE(number)...), 'n')
}

// HeaderKey = h + num8 + hash32.
func HeaderKey(number uint64, hash common.Hash) []byte {
	return append(append(append([]byte{}, HeaderPrefix...), Uint64BE(number)...), hash.Bytes()...)
}

// HeaderTDKey = h + num8 + hash32 + 't'.
func HeaderTDKey(number uint64, hash common.Hash) []byte {
	return append(HeaderKey(number, hash), 't')
}

// HeaderNumberKey = H + hash32.
func HeaderNumberKey(hash common.Hash) []byte {
	return append(append([]byte{}, HeaderNumberPrefix...), hash.Bytes()...)
}

// BodyKey = b + num8 + hash32.
func BodyKey(number uint64, hash common.Hash) []byte {
	return append(append(append([]byte{}, BodyPrefix...), Uint64BE(number)...), hash.Bytes()...)
}

// ReceiptsKey = r + num8 + hash32.
func ReceiptsKey(number uint64, hash common.Hash) []byte {
	return append(append(append([]byte{}, ReceiptsPrefix...), Uint64BE(number)...), hash.Bytes()...)
}

// TxLookupKey = l + hash32.
func TxLookupKey(hash common.Hash) []byte {
	return append(append([]byte{}, TxLookupPrefix...), hash.Bytes()...)
}

// CxLookupKey = cx + hash32.
func CxLookupKey(hash common.Hash) []byte {
	return append(append([]byte{}, CxLookupPrefix...), hash.Bytes()...)
}

// BlockSigKey = block-sig- + num8.
func BlockSigKey(number uint64) []byte {
	return append(append([]byte{}, BlockSigPrefix...), Uint64BE(number)...)
}

// RewardAccumKey = blk-rwd- + num8.
func RewardAccumKey(number uint64) []byte {
	return append(append([]byte{}, RewardAccumPrefix...), Uint64BE(number)...)
}

// ShardStateKey = ss + epoch.Bytes().
func ShardStateKey(epoch *big.Int) []byte {
	return append(append([]byte{}, ShardStatePrefix...), epoch.Bytes()...)
}

// EpochBlockNumberKey = harmony-epoch-block-number + epoch.Bytes().
func EpochBlockNumberKey(epoch *big.Int) []byte {
	return append(append([]byte{}, EpochBlockNumberPrefix...), epoch.Bytes()...)
}

// EpochVrfKey = epoch-vrf-block-numbers + epoch.Bytes().
func EpochVrfKey(epoch *big.Int) []byte {
	return append(append([]byte{}, EpochVrfPrefix...), epoch.Bytes()...)
}

// EpochVdfKey = epoch-vdf-block-number + epoch.Bytes().
func EpochVdfKey(epoch *big.Int) []byte {
	return append(append([]byte{}, EpochVdfPrefix...), epoch.Bytes()...)
}

// CrosslinkShardLastKey = cl + shard4.
func CrosslinkShardLastKey(shardID uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, shardID)
	return append(append([]byte{}, CrosslinkPrefix...), b...)
}

// CrosslinkIndexKey = cl + shard4 + num8.
func CrosslinkIndexKey(shardID uint32, number uint64) []byte {
	b := make([]byte, 12)
	binary.BigEndian.PutUint32(b, shardID)
	binary.BigEndian.PutUint64(b[4:], number)
	return append(append([]byte{}, CrosslinkPrefix...), b...)
}

// CxReceiptKey = cxReceipt + shard4 + num8 + hash32.
func CxReceiptKey(shardID uint32, number uint64, hash common.Hash) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, shardID)
	k := append(append([]byte{}, CxReceiptPrefix...), b...)
	k = append(k, Uint64BE(number)...)
	return append(k, hash.Bytes()...)
}

// CxSpentKey = cxReceiptSpent + shard4 + num8.
func CxSpentKey(shardID uint32, number uint64) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, shardID)
	k := append(append([]byte{}, CxSpentPrefix...), b...)
	return append(k, Uint64BE(number)...)
}

// ValidatorSnapshotKey = validator-snapshot + addr20 + epoch.Bytes().
func ValidatorSnapshotKey(addr common.Address, epoch *big.Int) []byte {
	k := append(append([]byte{}, ValidatorSnapshotPrefix...), addr.Bytes()...)
	return append(k, epoch.Bytes()...)
}

// ValidatorStatsKey = validator-stats + addr20.
func ValidatorStatsKey(addr common.Address) []byte {
	return append(append([]byte{}, ValidatorStatsPrefix...), addr.Bytes()...)
}

// DelegatorValidatorListKey = dvl + addr20.
func DelegatorValidatorListKey(addr common.Address) []byte {
	return append(append([]byte{}, DVLPrefix...), addr.Bytes()...)
}

// PreimageKey = secure-key- + hash32.
func PreimageKey(hash common.Hash) []byte {
	return append(append([]byte{}, PreimagePrefix...), hash.Bytes()...)
}

// ConfigKey = ethereum-config- + genesisHash32.
func ConfigKey(hash common.Hash) []byte {
	return append(append([]byte{}, ConfigPrefix...), hash.Bytes()...)
}

// GenesisSpecKey = ethereum-genesis- + genesisHash32.
func GenesisSpecKey(hash common.Hash) []byte {
	return append(append([]byte{}, GenesisPrefix...), hash.Bytes()...)
}

// CodeKey = c + hash32.
func CodeKey(hash common.Hash) []byte {
	return append(append([]byte{}, CodePrefix...), hash.Bytes()...)
}

// ValidatorCodeKey = vc + hash32.
func ValidatorCodeKey(hash common.Hash) []byte {
	return append(append([]byte{}, ValidatorCodePrefix...), hash.Bytes()...)
}

// BloomIndexCountKey / BloomIndexSectionHeadKey are the chain-indexer
// progress keys inside the iB table (geth core/chain_indexer.go
// setValidSections / setSectionHead).
func BloomIndexCountKey() []byte {
	return append(append([]byte{}, BloomBitsIndexPrefix...), []byte("count")...)
}

// BloomIndexSectionHeadKey = iB + "shead" + section8.
func BloomIndexSectionHeadKey(section uint64) []byte {
	k := append(append([]byte{}, BloomBitsIndexPrefix...), []byte("shead")...)
	return append(k, Uint64BE(section)...)
}

// HasPrefix is a tiny helper used by the classifier and scans.
func HasPrefix(key, prefix []byte) bool { return bytes.HasPrefix(key, prefix) }
