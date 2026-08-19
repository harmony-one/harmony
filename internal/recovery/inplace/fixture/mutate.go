package fixture

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
)

// Mutate opens the fixture database read-write (the tool under test is
// never running at this point) and applies fn. Used to derive corruption
// variants from the pristine fixture.
func Mutate(dir string, fn func(db *leveldb.DB) error) error {
	db, err := leveldb.OpenFile(dir, nil)
	if err != nil {
		return fmt.Errorf("open fixture for mutation: %w", err)
	}
	defer db.Close()
	return fn(db)
}

// DeleteKey removes one exact key.
func DeleteKey(dir string, key []byte) error {
	return Mutate(dir, func(db *leveldb.DB) error { return db.Delete(key, nil) })
}

// PutKey writes one exact key.
func PutKey(dir string, key, value []byte) error {
	return Mutate(dir, func(db *leveldb.DB) error { return db.Put(key, value, nil) })
}

// GetKey reads one exact key from the (stopped) fixture.
func GetKey(dir string, key []byte) ([]byte, error) {
	var out []byte
	err := Mutate(dir, func(db *leveldb.DB) error {
		v, err := db.Get(key, nil)
		if err != nil {
			return err
		}
		out = append([]byte(nil), v...)
		return nil
	})
	return out, err
}

// CopyDB copies a fixture database directory (file-level copy of a stopped
// DB) so tests can derive mutation variants without rebuilding.
func CopyDB(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, ent.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dst, ent.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// TrieNodes describes the physical trie-node keys of the fixture state,
// classified for targeted deletion/corruption.
type TrieNodes struct {
	AccountRoot     common.Hash
	AccountInternal []common.Hash // non-root standalone nodes that are not leaf-carrying
	AccountAll      []common.Hash // every standalone account-trie node incl. root

	StorageRoot     common.Hash   // storage root node of the probe account
	StorageInternal []common.Hash // non-root standalone storage nodes
	StorageAll      []common.Hash
}

// EnumerateTrieNodes walks the pristine fixture read-only and returns the
// standalone node hashes of the account trie and of probeAddr's storage
// trie, in deterministic iteration order.
func EnumerateTrieNodes(dir string, stateRoot common.Hash, probeAddr common.Address) (*TrieNodes, error) {
	db, err := rodb.Open(dir, rodb.Options{})
	if err != nil {
		return nil, err
	}
	defer db.Close()
	latch := &rodb.Latch{}
	sdb := state.NewDatabase(rawdb.NewDatabase(db.KV(latch)))

	out := &TrieNodes{AccountRoot: stateRoot}

	accountTrie, err := sdb.OpenTrie(stateRoot)
	if err != nil {
		return nil, err
	}
	var probeStorageRoot common.Hash
	probeKey := crypto.Keccak256(probeAddr.Bytes())
	it := accountTrie.NodeIterator(nil)
	for it.Next(true) {
		if it.Hash() != (common.Hash{}) {
			out.AccountAll = append(out.AccountAll, it.Hash())
			if it.Hash() != stateRoot && !it.Leaf() {
				out.AccountInternal = append(out.AccountInternal, it.Hash())
			}
		}
		if it.Leaf() && string(it.LeafKey()) == string(probeKey) {
			var acct state.Account
			if err := decodeAccount(it.LeafBlob(), &acct); err != nil {
				return nil, err
			}
			probeStorageRoot = acct.Root
		}
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	if probeStorageRoot == (common.Hash{}) {
		return nil, fmt.Errorf("probe account %s not found or without storage", probeAddr.Hex())
	}
	out.StorageRoot = probeStorageRoot

	storageTrie, err := sdb.OpenStorageTrie(stateRoot, common.BytesToHash(probeKey), probeStorageRoot)
	if err != nil {
		return nil, err
	}
	sit := storageTrie.NodeIterator(nil)
	for sit.Next(true) {
		if sit.Hash() != (common.Hash{}) {
			out.StorageAll = append(out.StorageAll, sit.Hash())
			if sit.Hash() != probeStorageRoot {
				out.StorageInternal = append(out.StorageInternal, sit.Hash())
			}
		}
	}
	if err := sit.Error(); err != nil {
		return nil, err
	}
	if latch.First() != nil {
		return nil, latch.First()
	}
	return out, nil
}

func decodeAccount(blob []byte, acct *state.Account) error {
	return rlp.DecodeBytes(blob, acct)
}
