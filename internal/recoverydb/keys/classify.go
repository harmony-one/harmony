package keys

import "bytes"

// Bucket names produced by Classify. These are the inventory namespaces and
// the logical-KV-digest domains. BucketBareHash32 is the single physical
// bucket for every un-prefixed 32-byte key (plan §2.2.9): legacy unprefixed
// contract/validator code and hash-scheme trie nodes are physically
// indistinguishable, so no per-namespace split of that keyspace is claimed
// anywhere.
const (
	BucketHeader             = "header"
	BucketCanonical          = "canonical"
	BucketTD                 = "td"
	BucketHeaderNumber       = "headerNumber"
	BucketBody               = "body"
	BucketReceipts           = "receipts"
	BucketTxLookup           = "txLookup"
	BucketCxLookup           = "cxLookup"
	BucketBloomBits          = "bloomBits"
	BucketBloomIndex         = "bloomIndex"
	BucketSkeletonHeader     = "skeletonHeader"
	BucketShardState         = "shardState"
	BucketBlockSig           = "blockSig"
	BucketPreimage           = "preimage"
	BucketConfig             = "config"
	BucketGenesisSpec        = "genesisSpec"
	BucketCrosslinkIndex     = "crosslinkIndex"
	BucketCrosslinkShardLast = "crosslinkShardLast"
	BucketDVL                = "dvl"
	BucketCxReceipt          = "cxReceipt"
	BucketCxSpent            = "cxSpent"
	BucketValidatorSnapshot  = "validatorSnapshot"
	BucketValidatorStats     = "validatorStats"
	BucketValidatorList      = "validatorList"
	BucketEpochBlockNumber   = "epochBlockNumber"
	BucketEpochVrf           = "epochVrf"
	BucketEpochVdf           = "epochVdf"
	BucketRewardAccum        = "rewardAccum"
	BucketCode               = "code"
	BucketValidatorCode      = "validatorCode"
	BucketSnapAccount        = "snapAccount"
	BucketSnapStorage        = "snapStorage"
	BucketPendingCrosslink   = "pendingCrosslink"
	BucketPendingSlashing    = "pendingSlashing"
	BucketRecoveryMarker     = "recoveryMarker"
	BucketBareHash32         = "bare-hash32"
	BucketMalformed          = "malformed"
	// BucketMeta covers singleton metadata keys; the full bucket name is
	// "meta." + the key string (e.g. "meta.LastBlock").
	BucketMetaPrefix = "meta."
)

var exactKeys = []struct {
	key    []byte
	bucket string
}{
	{DatabaseVersionKey, BucketMetaPrefix + "DatabaseVersion"},
	{HeadHeaderKey, BucketMetaPrefix + "LastHeader"},
	{HeadBlockKey, BucketMetaPrefix + "LastBlock"},
	{HeadFastBlockKey, BucketMetaPrefix + "LastFast"},
	{HeadFinalizedKey, BucketMetaPrefix + "LastFinalized"},
	{LastPivotKey, BucketMetaPrefix + "LastPivot"},
	{TrieSyncKey, BucketMetaPrefix + "TrieSync"},
	{SnapshotDisabledKey, BucketMetaPrefix + "SnapshotDisabled"},
	{SnapshotRootKey, BucketMetaPrefix + "SnapshotRoot"},
	{SnapshotJournalKey, BucketMetaPrefix + "SnapshotJournal"},
	{SnapshotGeneratorKey, BucketMetaPrefix + "SnapshotGenerator"},
	{SnapshotRecoveryKey, BucketMetaPrefix + "SnapshotRecovery"},
	{SnapshotSyncStatusKey, BucketMetaPrefix + "SnapshotSyncStatus"},
	{SkeletonSyncStatusKey, BucketMetaPrefix + "SkeletonSyncStatus"},
	{TxIndexTailKey, BucketMetaPrefix + "TransactionIndexTail"},
	{FastTxLookupLimitKey, BucketMetaPrefix + "FastTransactionLookupLimit"},
	{BadBlockKey, BucketMetaPrefix + "InvalidBlock"},
	{UncleanShutdownKey, BucketMetaPrefix + "unclean-shutdown"},
	{Eth2TransitionKey, BucketMetaPrefix + "eth2-transition"},
	{LastCommitsKey, BucketMetaPrefix + "LastCommits"},
	{ContinuousKey, BucketMetaPrefix + "continuous"},
	{SnapdbInfoKey, BucketMetaPrefix + "SnapdbInfo"},
	{ValidatorListKey, BucketValidatorList},
	{PreimageImportKey, BucketMetaPrefix + "preimage-import"},
	{PreimageGenStartKey, BucketMetaPrefix + "preimage-gen-start"},
	{PreimageGenEndKey, BucketMetaPrefix + "preimage-gen-end"},
	{PendingCrosslinkKey, BucketPendingCrosslink},
	{PendingSlashingKey, BucketPendingSlashing},
	{RecoveryMarkerKey, BucketRecoveryMarker},
}

// prefixRule is a prefix plus a payload-shape predicate (payload = key minus
// prefix). Rules are evaluated in order; within the table longer prefixes
// come before their own proper prefixes (cxReceiptSpent > cxReceipt > cx > c,
// cl > c, vc before bare, secure-key- before ss's 's', etc.).
type prefixRule struct {
	prefix []byte
	bucket string
	shape  func(payload []byte) bool
}

func anyShape([]byte) bool { return true }
func lenIs(n int) func([]byte) bool {
	return func(p []byte) bool { return len(p) == n }
}
func lenBetween(lo, hi int) func([]byte) bool {
	return func(p []byte) bool { return len(p) >= lo && len(p) <= hi }
}

var prefixRules = []prefixRule{
	// Long, unambiguous string prefixes first.
	{EpochBlockNumberPrefix, BucketEpochBlockNumber, lenBetween(0, 8)}, // "harmony-epoch-block-number"
	{EpochVrfPrefix, BucketEpochVrf, lenBetween(0, 8)},
	{EpochVdfPrefix, BucketEpochVdf, lenBetween(0, 8)},
	{ConfigPrefix, BucketConfig, lenIs(32)},
	{GenesisPrefix, BucketGenesisSpec, lenIs(32)},
	{ValidatorSnapshotPrefix, BucketValidatorSnapshot, lenBetween(20, 28)}, // addr20 + epoch(<=8)
	{ValidatorStatsPrefix, BucketValidatorStats, lenIs(20)},
	{CxSpentPrefix, BucketCxSpent, lenIs(12)},        // "cxReceiptSpent" before "cxReceipt"
	{CxReceiptPrefix, BucketCxReceipt, lenIs(44)},    // before "cx"
	{PreimagePrefix, BucketPreimage, lenIs(32)},      // "secure-key-" before "ss"
	{BlockSigPrefix, BucketBlockSig, lenIs(8)},       // "block-sig-" before "b"
	{RewardAccumPrefix, BucketRewardAccum, lenIs(8)}, // "blk-rwd-" before "b"
	// Geth light-client relics (cht-/blt-/clique-) are intentionally NOT
	// classified: harmony never writes them, and their 'c'-lead unbounded
	// shapes would swallow genuine 32-byte hash keys. Any such stray key
	// lands in bare-hash32 (if 32 bytes) or malformed, which is the honest
	// physical classification (plan §2.2.9).
	{DVLPrefix, BucketDVL, lenIs(20)},
	{ShardStatePrefix, BucketShardState, lenBetween(0, 8)},
	{CxLookupPrefix, BucketCxLookup, lenIs(32)},
	{CrosslinkPrefix, BucketCrosslinkShardLast, lenIs(4)},
	{CrosslinkPrefix, BucketCrosslinkIndex, lenIs(12)},
	{ValidatorCodePrefix, BucketValidatorCode, lenIs(32)},
	{BloomBitsIndexPrefix, BucketBloomIndex, bloomIndexShape}, // "iB"
	// Single-byte prefixes, tightly shape-bound.
	{HeaderNumberPrefix, BucketHeaderNumber, lenIs(32)}, // H
	{BloomBitsPrefix, BucketBloomBits, lenIs(42)},       // B + bit2+section8+hash32
	{SkeletonHeaderPrefix, BucketSkeletonHeader, lenIs(8)},
	{BodyPrefix, BucketBody, lenIs(40)},                        // b
	{ReceiptsPrefix, BucketReceipts, lenIs(40)},                // r
	{TxLookupPrefix, BucketTxLookup, lenIs(32)},                // l
	{SnapshotAccountPrefix, BucketSnapAccount, lenIs(32)}, // a
	{SnapshotStoragePrefix, BucketSnapStorage, lenIs(64)}, // o
	// Path-scheme trie-node prefixes ('A'/'O') are intentionally NOT
	// classified: harmony databases are hash-scheme only, and an unbounded
	// 'A'/'O' rule would intercept the ~2/256 of genuine 32-byte trie-node
	// hashes whose first byte happens to be 0x41/0x4f (round 13 finding 1).
	// A 32-byte key starting with 'A'/'O' therefore lands in bare-hash32;
	// any other 'A'/'O' key (a genuine path-scheme node would be one) lands
	// in malformed, which verify-db treats as fatal.
	{CodePrefix, BucketCode, lenIs(32)}, // c — after cl/cx*/cht/clique/continuous
}

func bloomIndexShape(payload []byte) bool {
	if bytes.Equal(payload, []byte("count")) {
		return true
	}
	return len(payload) == 5+8 && bytes.HasPrefix(payload, []byte("shead"))
}

// Classify returns the bucket name for a raw key. Header-family keys ('h')
// need dedicated shape dispatch because three shapes share the prefix.
func Classify(key []byte) string {
	for _, e := range exactKeys {
		if bytes.Equal(key, e.key) {
			return e.bucket
		}
	}
	// 'h' family: "harmony-epoch-block-number" was matched above (exact
	// table has no 'h' entries; prefixRules starts with it).
	for _, r := range prefixRules {
		if bytes.HasPrefix(key, r.prefix) && r.shape(key[len(r.prefix):]) {
			return r.bucket
		}
	}
	if len(key) > 0 && key[0] == 'h' {
		switch {
		case len(key) == 1+8+1 && key[9] == 'n':
			return BucketCanonical
		case len(key) == 1+8+32:
			return BucketHeader
		case len(key) == 1+8+32+1 && key[41] == 't':
			return BucketTD
		}
	}
	if len(key) == 32 {
		return BucketBareHash32
	}
	return BucketMalformed
}
