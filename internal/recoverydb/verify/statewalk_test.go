package verify

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
)

// buildState creates a small on-disk state: two EOAs, one contract with
// multi-node storage, code under the requested location. Returns (db, root,
// codeHash).
func buildState(t *testing.T, codeLoc string) (ethdb.Database, common.Hash, common.Hash) {
	t.Helper()
	db := rawdb.NewMemoryDatabase()
	sdb := state.NewDatabaseWithConfig(db, &trie.Config{Preimages: true})
	st, err := state.New(common.Hash{}, sdb, nil)
	if err != nil {
		t.Fatal(err)
	}
	st.AddBalance(common.HexToAddress("0x01"), big.NewInt(1e18))
	st.AddBalance(common.HexToAddress("0x02"), big.NewInt(2e18))

	contract := common.HexToAddress("0xc0")
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52} // tiny but real-looking
	st.SetCode(contract, code, false)
	// Enough slots for a multi-node storage trie.
	for i := byte(1); i <= 8; i++ {
		st.SetState(contract, common.BytesToHash([]byte{i}), common.BytesToHash([]byte{0xf0, i}))
	}
	root, err := st.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := sdb.TrieDB().Commit(root, false); err != nil {
		t.Fatal(err)
	}
	if err := sdb.TrieDB().CommitPreimages(); err != nil {
		t.Fatal(err)
	}

	codeHash := crypto.Keccak256Hash(code)
	switch codeLoc {
	case CodeLocPrefixed:
		// state.SetCode wrote it under 'c' already.
	case CodeLocValidator:
		// Move the blob to the vc location only.
		if err := db.Delete(keys.CodeKey(codeHash)); err != nil {
			t.Fatal(err)
		}
		if err := db.Put(keys.ValidatorCodeKey(codeHash), code); err != nil {
			t.Fatal(err)
		}
	case CodeLocLegacy:
		if err := db.Delete(keys.CodeKey(codeHash)); err != nil {
			t.Fatal(err)
		}
		if err := db.Put(codeHash.Bytes(), code); err != nil {
			t.Fatal(err)
		}
	}
	return db, root, codeHash
}

func TestWalkStateHappyPath(t *testing.T) {
	db, root, _ := buildState(t, CodeLocPrefixed)
	res, err := WalkState(db, root, StateWalkOptions{CheckPreimages: true, RequirePreimages: true})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if res.AccountCount != 3 || res.StorageSlotCount != 8 || res.UniqueCodeCount != 1 {
		t.Fatalf("counts wrong: %+v", res)
	}
	if res.CodeLocationCounts[CodeLocPrefixed] != 1 {
		t.Fatalf("code location wrong: %+v", res.CodeLocationCounts)
	}
}

// TestCodeFallbackLocations: a validator-code-only blob and a legacy
// unprefixed blob both resolve through the fallback chain (round 6 finding
// 6, defect 2 fixed) with the location tag preserved in the digest.
func TestCodeFallbackLocations(t *testing.T) {
	for _, loc := range []string{CodeLocValidator, CodeLocLegacy} {
		db, root, codeHash := buildState(t, loc)
		res, err := WalkState(db, root, StateWalkOptions{})
		if err != nil {
			t.Fatalf("loc %s: %v", loc, err)
		}
		if res.CodeLocationCounts[loc] != 1 {
			t.Fatalf("loc %s not used: %+v", loc, res.CodeLocationCounts)
		}
		code, gotLoc, err := ResolveCode(db, codeHash)
		if err != nil || gotLoc != loc || len(code) == 0 {
			t.Fatalf("ResolveCode: %v %s", err, gotLoc)
		}
	}
}

// TestWrongContentCodeFatal: code whose keccak does not match its hash is
// fatal here — the stock state iterator performs no content verification
// (regression guard for the fail-closed contract).
func TestWrongContentCodeFatal(t *testing.T) {
	db, root, codeHash := buildState(t, CodeLocPrefixed)
	if err := db.Put(keys.CodeKey(codeHash), []byte{0xba, 0xad}); err != nil {
		t.Fatal(err)
	}
	if _, err := WalkState(db, root, StateWalkOptions{}); err == nil || !strings.Contains(err.Error(), "content verification") {
		t.Fatalf("wrong-content code must be fatal, got %v", err)
	}
	// Stock comparison: the stock iterator loads the tampered blob without
	// complaint (core/state/iterator.go has no keccak check) — this is why
	// the purpose-built traversal exists.
	sdb := state.NewDatabase(db)
	st, err := state.New(root, sdb, nil)
	if err != nil {
		t.Fatal(err)
	}
	stockIt := state.NewNodeIterator(st)
	for stockIt.Next() {
	}
	if stockIt.Error != nil {
		t.Fatalf("expected the stock iterator to accept tampered code silently; got %v (stock behavior changed?)", stockIt.Error)
	}
}

// TestDeletedCodeFatal covers the deleted c/vc/legacy code fixtures: the
// walk fails naming the hash (plan WS2 acceptance).
func TestDeletedCodeFatal(t *testing.T) {
	db, root, codeHash := buildState(t, CodeLocPrefixed)
	if err := db.Delete(keys.CodeKey(codeHash)); err != nil {
		t.Fatal(err)
	}
	_, err := WalkState(db, root, StateWalkOptions{})
	if err == nil || !strings.Contains(err.Error(), codeHash.Hex()) {
		t.Fatalf("deleted code must fail naming the hash, got %v", err)
	}
}

// TestDeletedStorageNodeFatal: deleting storage-trie nodes (root and inner)
// is a fatal traversal error, never silently treated as empty storage
// (round 6 finding 6, defect 1; in-place §C3).
func TestDeletedStorageNodeFatal(t *testing.T) {
	db, root, _ := buildState(t, CodeLocPrefixed)

	// Collect the storage trie's node hashes via a clean walk.
	var storageNodes []common.Hash
	accountNodes := map[common.Hash]bool{}
	// First: account-trie nodes only (walk with a broken OnNode that
	// records everything; account nodes come from the outer trie).
	res, err := WalkState(db, root, StateWalkOptions{OnNode: func(h common.Hash) error {
		storageNodes = append(storageNodes, h)
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.StorageSlotCount != 8 {
		t.Fatal("fixture must have storage")
	}
	_ = accountNodes

	// Deleting ANY reachable trie node must break the walk (root nodes fail
	// at construction, inner nodes during iteration — both fatal).
	broken := 0
	for _, h := range storageNodes {
		val, err := db.Get(h.Bytes())
		if err != nil {
			continue
		}
		if err := db.Delete(h.Bytes()); err != nil {
			t.Fatal(err)
		}
		if _, err := WalkState(db, root, StateWalkOptions{}); err == nil {
			t.Fatalf("deleting trie node %s did not break the walk", h.Hex())
		}
		broken++
		if err := db.Put(h.Bytes(), val); err != nil {
			t.Fatal(err)
		}
	}
	if broken < 3 {
		t.Fatalf("fixture too small: only %d nodes exercised", broken)
	}
}

// TestPreimageCoverage: deleted account/storage-slot preimages fail under
// --require-preimages and are counted exactly without it (plan WS2).
func TestPreimageCoverage(t *testing.T) {
	db, root, _ := buildState(t, CodeLocPrefixed)

	// Find one account preimage and one storage preimage to delete: the
	// hashed keys are keccak(addr) and keccak(slot).
	accountHash := crypto.Keccak256Hash(common.HexToAddress("0x01").Bytes())
	slotHash := crypto.Keccak256Hash(common.BytesToHash([]byte{1}).Bytes())
	if err := db.Delete(keys.PreimageKey(accountHash)); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(keys.PreimageKey(slotHash)); err != nil {
		t.Fatal(err)
	}

	res, err := WalkState(db, root, StateWalkOptions{CheckPreimages: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.MissingAccountPreimages != 1 || res.MissingStoragePreimages != 1 {
		t.Fatalf("preimage counts wrong: %+v", res)
	}
	if _, err := WalkState(db, root, StateWalkOptions{CheckPreimages: true, RequirePreimages: true}); err == nil ||
		!strings.Contains(err.Error(), "preimage") {
		t.Fatalf("missing preimage must be fatal under --require-preimages, got %v", err)
	}
}

// TestLogicalDigestMarkerExclusion: the §11.4 digest is identical with the
// excluded keys present or deleted — the recovery marker (round 7 finding 1
// regression) and the preimage bookkeeping pair the stock node rewrites on
// every clean Stop (round 14 finding 4; centralized per round 15 finding 1).
func TestLogicalDigestMarkerExclusion(t *testing.T) {
	db, _, _ := buildState(t, CodeLocPrefixed)
	before, err := ComputeLogicalDigest(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(keys.RecoveryMarkerKey, []byte(`{"any":"thing"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(keys.PreimageGenStartKey, []byte{0, 0, 0, 0, 0, 0, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.Put(keys.PreimageGenEndKey, []byte{0, 0, 0, 0, 0, 0, 0, 22}); err != nil {
		t.Fatal(err)
	}
	with, err := ComputeLogicalDigest(db)
	if err != nil {
		t.Fatal(err)
	}
	if before.Total != with.Total {
		t.Fatal("every DigestExcludedKey must be excluded from the logical digest by definition")
	}
	for _, k := range [][]byte{keys.RecoveryMarkerKey, keys.PreimageGenStartKey, keys.PreimageGenEndKey} {
		if !DigestExcludedKey(k) {
			t.Fatalf("DigestExcludedKey must cover %q", k)
		}
	}
	if DigestExcludedKey([]byte("LastBlock")) {
		t.Fatal("DigestExcludedKey must not cover ordinary meta keys")
	}
	// And a single flipped value byte elsewhere changes it.
	if err := db.Put([]byte("LastBlock"), []byte{1}); err != nil {
		t.Fatal(err)
	}
	after, err := ComputeLogicalDigest(db)
	if err != nil {
		t.Fatal(err)
	}
	if after.Total == before.Total {
		t.Fatal("digest must be sensitive to value changes")
	}
}
