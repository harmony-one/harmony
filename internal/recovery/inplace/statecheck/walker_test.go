package statecheck_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	bls "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/recovery/inplace/report"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
	"github.com/harmony-one/harmony/internal/recovery/inplace/statecheck"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

// buildToyState writes the 3-account toy state (EOA, contract with storage,
// vc-namespace validator) and returns its root. tweak lets variants change
// one value before commit.
func buildToyState(t *testing.T, dir string, tweak func(st *state.DB)) common.Hash {
	t.Helper()
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	sdb := state.NewDatabase(db)
	st, err := state.New(common.Hash{}, sdb, nil)
	if err != nil {
		t.Fatal(err)
	}

	eoa := common.HexToAddress("0x1000000000000000000000000000000000000001")
	st.SetBalance(eoa, big.NewInt(12345))
	st.SetNonce(eoa, 7)

	contract := common.HexToAddress("0x2000000000000000000000000000000000000002")
	st.SetCode(contract, []byte("toy contract code"), false)
	for j := 0; j < 40; j++ {
		st.SetState(contract,
			crypto.Keccak256Hash([]byte(fmt.Sprintf("k%d", j))),
			crypto.Keccak256Hash([]byte(fmt.Sprintf("v%d", j))))
	}

	validator := common.HexToAddress("0x3000000000000000000000000000000000000003")
	w := staketest.GetDefaultValidatorWrapperWithAddr(validator, []bls.SerializedPublicKey{{0x0a}})
	if err := st.UpdateValidatorWrapper(validator, &w); err != nil {
		t.Fatal(err)
	}
	st.SetValidatorFlag(validator)

	if tweak != nil {
		tweak(st)
	}
	root, err := st.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	if err := sdb.TrieDB().Commit(root, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func walkDir(t *testing.T, dir string, root common.Hash, workers int) (*statecheck.Result, error) {
	t.Helper()
	db, err := rodb.Open(dir, rodb.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	latch := &rodb.Latch{}
	res, err := statecheck.Walk(statecheck.Config{
		KV:        db.KV(latch),
		StateRoot: root,
		Workers:   workers,
	})
	if latch.First() != nil && err == nil {
		t.Fatalf("latch dirty on success: %v", latch.First())
	}
	return res, err
}

func decodeRLP(blob []byte, out interface{}) error { return rlp.DecodeBytes(blob, out) }

const goldenPath = "../../../../testdata/recovery/preflight/golden/toy_state_digest.txt"

// TestDigestGoldenVector: the 3-account toy state digest is pinned to a
// committed golden vector (regenerate with UPDATE_GOLDEN=1).
func TestDigestGoldenVector(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "toy")
	root := buildToyState(t, dir, nil)
	res, err := walkDir(t, dir, root, 2)
	if err != nil {
		t.Fatal(err)
	}
	got := hex.EncodeToString(res.Digest[:])
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden updated: %s", got)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden vector missing (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("digest %s != golden %s", got, strings.TrimSpace(string(want)))
	}
	if res.Counts.Accounts != 3 || res.Counts.StorageTries != 2 || res.Counts.UniqueCodeContract != 1 || res.Counts.UniqueCodeValidator != 1 {
		t.Fatalf("toy counts %+v", res.Counts)
	}
}

// TestDigestWorkerInvariance: byte-identical digests across worker counts.
func TestDigestWorkerInvariance(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "toy")
	root := buildToyState(t, dir, nil)
	r1, err := walkDir(t, dir, root, 1)
	if err != nil {
		t.Fatal(err)
	}
	r8, err := walkDir(t, dir, root, 8)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Digest != r8.Digest || r1.Counts != r8.Counts {
		t.Fatalf("worker variance: %x/%x", r1.Digest, r8.Digest)
	}
}

// TestDigestSensitivity: one flipped storage value changes the digest.
func TestDigestSensitivity(t *testing.T) {
	dirA := filepath.Join(t.TempDir(), "a")
	rootA := buildToyState(t, dirA, nil)
	dirB := filepath.Join(t.TempDir(), "b")
	rootB := buildToyState(t, dirB, func(st *state.DB) {
		st.SetState(common.HexToAddress("0x2000000000000000000000000000000000000002"),
			crypto.Keccak256Hash([]byte("k0")), common.HexToHash("0xff"))
	})
	if rootA == rootB {
		t.Fatal("tweak did not change the root")
	}
	ra, err := walkDir(t, dirA, rootA, 2)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := walkDir(t, dirB, rootB, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ra.Digest == rb.Digest {
		t.Fatal("digest insensitive to a changed storage value")
	}
}

// TestDigestRematerialization: copying every key into a fresh LevelDB in
// reverse order (different physical layout) leaves the digest unchanged.
func TestDigestRematerialization(t *testing.T) {
	dirA := filepath.Join(t.TempDir(), "a")
	rootA := buildToyState(t, dirA, nil)
	ra, err := walkDir(t, dirA, rootA, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Re-materialize: read all pairs, write them in descending key order.
	src, err := leveldb.OpenFile(dirA, nil)
	if err != nil {
		t.Fatal(err)
	}
	type kv struct{ k, v []byte }
	var pairs []kv
	it := src.NewIterator(nil, nil)
	for it.Next() {
		pairs = append(pairs, kv{
			k: append([]byte(nil), it.Key()...),
			v: append([]byte(nil), it.Value()...),
		})
	}
	it.Release()
	src.Close()

	dirB := filepath.Join(t.TempDir(), "b")
	dst, err := leveldb.OpenFile(dirB, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(pairs) - 1; i >= 0; i-- {
		if err := dst.Put(pairs[i].k, pairs[i].v, nil); err != nil {
			t.Fatal(err)
		}
	}
	dst.Close()

	rb, err := walkDir(t, dirB, rootA, 4)
	if err != nil {
		t.Fatal(err)
	}
	if ra.Digest != rb.Digest || ra.Counts != rb.Counts {
		t.Fatalf("digest not layout-invariant: %x vs %x", ra.Digest, rb.Digest)
	}
}

// TestStockIteratorDefectDifferential documents that the stock
// core/state/iterator.go hard-fails on a vc-namespace validator account
// (defect 1: the ValidatorCode fallback is unreachable because
// ContractCode errors on a miss), while this walker passes the same state.
func TestStockIteratorDefectDifferential(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "toy")
	root := buildToyState(t, dir, nil)

	// Our walker passes.
	if _, err := walkDir(t, dir, root, 2); err != nil {
		t.Fatalf("walker failed: %v", err)
	}

	// The stock iterator fails on the validator account's vc-only code.
	db, err := rawdb.NewLevelDBDatabase(dir, 16, 64, "", true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stDB, err := state.New(root, state.NewDatabase(db), nil)
	if err != nil {
		t.Fatal(err)
	}
	it := state.NewNodeIterator(stDB)
	for it.Next() {
	}
	if it.Error == nil {
		t.Fatal("stock iterator unexpectedly succeeded on a vc-only validator account (defect 1 fixed upstream? re-evaluate the bypass)")
	}
	if !strings.Contains(it.Error.Error(), "code") {
		t.Fatalf("stock iterator error %q does not look like the missing-code defect", it.Error)
	}
}

// TestWalkerStorageDeletions: unit-level defect-2 geometry - the storage
// root resolves but a child node is missing; the walk must FAIL, never
// silently treat the trie as empty. Companion: deleting the root itself
// fails on the open path.
func TestWalkerStorageDeletions(t *testing.T) {
	contractKey := crypto.Keccak256(common.HexToAddress("0x2000000000000000000000000000000000000002").Bytes())

	enumerate := func(t *testing.T, dir string, root common.Hash) (storageRoot common.Hash, internal []common.Hash) {
		db, err := rodb.Open(dir, rodb.Options{})
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		sdb := state.NewDatabase(rawdb.NewDatabase(db.KV(&rodb.Latch{})))
		tr, err := sdb.OpenTrie(root)
		if err != nil {
			t.Fatal(err)
		}
		it := tr.NodeIterator(nil)
		var acct state.Account
		for it.Next(true) {
			if it.Leaf() && string(it.LeafKey()) == string(contractKey) {
				if err := decodeRLP(it.LeafBlob(), &acct); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := it.Error(); err != nil {
			t.Fatal(err)
		}
		if acct.Root == (common.Hash{}) {
			t.Fatal("contract account not found")
		}
		stTrie, err := sdb.OpenStorageTrie(root, common.BytesToHash(contractKey), acct.Root)
		if err != nil {
			t.Fatal(err)
		}
		sit := stTrie.NodeIterator(nil)
		for sit.Next(true) {
			if sit.Hash() != (common.Hash{}) && sit.Hash() != acct.Root {
				internal = append(internal, sit.Hash())
			}
		}
		if err := sit.Error(); err != nil {
			t.Fatal(err)
		}
		return acct.Root, internal
	}

	t.Run("child-node-deleted", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "toy")
		root := buildToyState(t, dir, nil)
		_, internal := enumerate(t, dir, root)
		if len(internal) == 0 {
			t.Fatal("storage trie too small for a child deletion")
		}
		del, err := leveldb.OpenFile(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := del.Delete(internal[0].Bytes(), nil); err != nil {
			t.Fatal(err)
		}
		del.Close()
		_, err = walkDir(t, dir, root, 2)
		f, ok := err.(*report.Failure)
		if !ok || !strings.Contains(f.Reason, "missing trie node") {
			t.Fatalf("want missing-node failure, got %v", err)
		}
	})
	t.Run("root-node-deleted", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "toy")
		root := buildToyState(t, dir, nil)
		storageRoot, _ := enumerate(t, dir, root)
		del, err := leveldb.OpenFile(dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := del.Delete(storageRoot.Bytes(), nil); err != nil {
			t.Fatal(err)
		}
		del.Close()
		_, err = walkDir(t, dir, root, 2)
		f, ok := err.(*report.Failure)
		if !ok || !strings.Contains(f.Reason, "missing trie node") {
			t.Fatalf("want missing-node failure on the open path, got %v", err)
		}
	})
}
