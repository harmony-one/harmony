package keys

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
)

// TestSchemaPinning writes records through the stock rawdb accessors and
// asserts they land under exactly this package's keys, so upstream schema
// drift fails loudly (plan WS1).
func TestSchemaPinning(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	hash := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	addr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	epoch := big.NewInt(3002)

	if err := rawdb.WriteCanonicalHash(db, hash, 42); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(CanonicalHashKey(42)); common.BytesToHash(got) != hash {
		t.Fatalf("canonical key mismatch")
	}
	if err := rawdb.WriteBlockCommitSig(db, 42, []byte{1, 2, 3}); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(BlockSigKey(42)); len(got) != 3 {
		t.Fatalf("block-sig key mismatch")
	}
	if err := rawdb.WriteShardStateBytes(db, epoch, []byte{9}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(ShardStateKey(epoch)); !ok {
		t.Fatalf("shard state key mismatch")
	}
	if err := rawdb.WriteCrossLinkShardBlock(db, 1, 7, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(CrosslinkIndexKey(1, 7)); !ok {
		t.Fatalf("crosslink index key mismatch")
	}
	if err := rawdb.WriteShardLastCrossLink(db, 1, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(CrosslinkShardLastKey(1)); !ok {
		t.Fatalf("crosslink shard-last key mismatch")
	}
	if err := rawdb.WriteCXReceiptsProofSpentWithKey(db, 1, 9); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(CxSpentKey(1, 9)); !ok {
		t.Fatalf("cx spent key mismatch")
	}
	if err := rawdb.WriteEpochBlockNumber(db, epoch, big.NewInt(5)); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(EpochBlockNumberKey(epoch)); !ok {
		t.Fatalf("epoch block number key mismatch")
	}
	if err := rawdb.WriteEpochVrfBlockNums(db, epoch, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(EpochVrfKey(epoch)); !ok {
		t.Fatalf("epoch vrf key mismatch")
	}
	if err := rawdb.WriteEpochVdfBlockNum(db, epoch, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(EpochVdfKey(epoch)); !ok {
		t.Fatalf("epoch vdf key mismatch")
	}
	if err := rawdb.WriteValidatorList(db, []common.Address{addr}); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(ValidatorListKey); !ok {
		t.Fatalf("validator list key mismatch")
	}
	if err := rawdb.WriteDelegationsByDelegator(db, addr, nil); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(DelegatorValidatorListKey(addr)); !ok {
		t.Fatalf("dvl key mismatch")
	}
	if err := rawdb.WriteBlockRewardAccumulator(db, big.NewInt(77), 42); err != nil {
		t.Fatal(err)
	}
	if ok, _ := db.Has(RewardAccumKey(42)); !ok {
		t.Fatalf("reward accumulator key mismatch")
	}
	rawdb.WriteCode(db, hash, []byte{0xde})
	if ok, _ := db.Has(CodeKey(hash)); !ok {
		t.Fatalf("code key mismatch")
	}
	rawdb.WriteValidatorCode(db, hash, []byte{0xad})
	if ok, _ := db.Has(ValidatorCodeKey(hash)); !ok {
		t.Fatalf("validator code key mismatch")
	}
	if err := rawdb.WriteHeadBlockHash(db, hash); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Get(HeadBlockKey); common.BytesToHash(got) != hash {
		t.Fatalf("LastBlock key mismatch")
	}
}

// TestClassifier covers the longest-prefix, shape-aware collision basics
// (plan WS2 acceptance: planted cl/cx* keys never counted as code; a legacy
// code key and an orphan trie node both land in bare-hash32; malformed
// itemized; 32-byte hashes leading with prefix bytes stay bare).
func TestClassifier(t *testing.T) {
	hash := common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")
	addr := common.HexToAddress("0x4444444444444444444444444444444444444444")
	epoch := big.NewInt(3)

	cases := []struct {
		name   string
		key    []byte
		bucket string
	}{
		{"canonical", CanonicalHashKey(5), BucketCanonical},
		{"header", HeaderKey(5, hash), BucketHeader},
		{"td", HeaderTDKey(5, hash), BucketTD},
		{"headerNumber", HeaderNumberKey(hash), BucketHeaderNumber},
		{"body", BodyKey(5, hash), BucketBody},
		{"receipts", ReceiptsKey(5, hash), BucketReceipts},
		{"txLookup", TxLookupKey(hash), BucketTxLookup},
		{"cxLookup", CxLookupKey(hash), BucketCxLookup},
		{"blockSig", BlockSigKey(5), BucketBlockSig},
		{"rewardAccum", RewardAccumKey(5), BucketRewardAccum},
		{"shardState", ShardStateKey(epoch), BucketShardState},
		{"epochBlockNumber", EpochBlockNumberKey(epoch), BucketEpochBlockNumber},
		{"epochVrf", EpochVrfKey(epoch), BucketEpochVrf},
		{"epochVdf", EpochVdfKey(epoch), BucketEpochVdf},
		{"crosslinkIndex", CrosslinkIndexKey(1, 5), BucketCrosslinkIndex},
		{"crosslinkShardLast", CrosslinkShardLastKey(1), BucketCrosslinkShardLast},
		{"cxReceipt", CxReceiptKey(1, 5, hash), BucketCxReceipt},
		{"cxSpent", CxSpentKey(1, 5), BucketCxSpent},
		{"validatorSnapshot", ValidatorSnapshotKey(addr, epoch), BucketValidatorSnapshot},
		{"validatorStats", ValidatorStatsKey(addr), BucketValidatorStats},
		{"validatorList", ValidatorListKey, BucketValidatorList},
		{"dvl", DelegatorValidatorListKey(addr), BucketDVL},
		{"preimage", PreimageKey(hash), BucketPreimage},
		{"config", ConfigKey(hash), BucketConfig},
		{"genesisSpec", GenesisSpecKey(hash), BucketGenesisSpec},
		{"code", CodeKey(hash), BucketCode},
		{"validatorCode", ValidatorCodeKey(hash), BucketValidatorCode},
		{"bloomIndexCount", BloomIndexCountKey(), BucketBloomIndex},
		{"bloomIndexShead", BloomIndexSectionHeadKey(3), BucketBloomIndex},
		{"marker", RecoveryMarkerKey, BucketRecoveryMarker},
		{"pendingCL", PendingCrosslinkKey, BucketPendingCrosslink},
		{"pendingSC", PendingSlashingKey, BucketPendingSlashing},
		{"lastBlock", HeadBlockKey, BucketMetaPrefix + "LastBlock"},
		{"lastFinalized", HeadFinalizedKey, BucketMetaPrefix + "LastFinalized"},
		{"snapdbInfo", SnapdbInfoKey, BucketMetaPrefix + "SnapdbInfo"},
		{"uncleanShutdown", UncleanShutdownKey, BucketMetaPrefix + "unclean-shutdown"},
		{"invalidBlock", BadBlockKey, BucketMetaPrefix + "InvalidBlock"},

		// The single physical bare-hash32 bucket (plan §2.2.9): a legacy
		// unprefixed code blob and an orphan trie node are indistinguishable.
		{"bareTrieNode", hash.Bytes(), BucketBareHash32},

		// 32-byte hashes that HAPPEN to lead with prefix bytes must stay
		// bare, never mis-bucketed (shape bounds).
		{"bareLeadingCl", append([]byte("cl"), make([]byte, 30)...), BucketBareHash32},
		{"bareLeadingCx", append([]byte("cx"), make([]byte, 30)...), BucketBareHash32},
		{"bareLeadingSs", append([]byte("ss"), make([]byte, 30)...), BucketBareHash32},
		{"bareLeadingVc", append([]byte("vc"), make([]byte, 30)...), BucketBareHash32},
		{"bareLeadingH", append([]byte("h"), make([]byte, 31)...), BucketBareHash32},
		{"bareLeadingIB", append([]byte("iB"), make([]byte, 30)...), BucketBareHash32},
		// Round 13 finding 1: 32-byte hash-scheme trie nodes whose first
		// byte is 'A' (0x41) or 'O' (0x4f) must stay bare-hash32; the old
		// unbounded path-scheme rules intercepted them and verify-db then
		// rejected valid artifacts. Non-32-byte 'A'/'O' keys (a genuine
		// path-scheme node would be one) are malformed.
		{"bareLeadingA", append([]byte("A"), make([]byte, 31)...), BucketBareHash32},
		{"bareLeadingO", append([]byte("O"), make([]byte, 31)...), BucketBareHash32},
		{"pathSchemeAcctMalformed", append([]byte("A"), make([]byte, 10)...), BucketMalformed},
		{"pathSchemeStoreMalformed", append([]byte("O"), make([]byte, 40)...), BucketMalformed},

		// Malformed keys are itemized, never guessed.
		{"malformedShortB", []byte("blockXYZ"), BucketMalformed},
		{"malformedLongCl", append([]byte("cl"), make([]byte, 40)...), BucketMalformed},
		{"malformed33", make([]byte, 33), BucketMalformed},
	}
	for _, c := range cases {
		if got := Classify(c.key); got != c.bucket {
			t.Errorf("%s: Classify(%x) = %s, want %s", c.name, c.key, got, c.bucket)
		}
	}
}
