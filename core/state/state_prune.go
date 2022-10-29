package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/core/rawdb"
)

// DiffAndPrune deletes data exists in oldDB, but not in newDB
func DiffAndPrune(oldDB *DB, newDB *DB, batch rawdb.DatabaseDeleter) (int, error) {
	// create difference iterator
	differIt, _ := trie.NewDifferenceIterator(newDB.trie.NodeIterator(nil), oldDB.trie.NodeIterator(nil))

	count := 0
	for differIt.Next(true) {
		if !differIt.Leaf() {
			count++
			batch.Delete(differIt.Hash().Bytes())
		} else {
			// build account data
			addrBytes := oldDB.trie.GetKey(differIt.LeafKey())
			addr := common.BytesToAddress(addrBytes)

			var oldAccount, newAccount Account
			if err := rlp.DecodeBytes(differIt.LeafBlob(), &oldAccount); err != nil {
				continue
			}

			if accountBytes, err := newDB.trie.TryGet(addrBytes); err != nil {
				continue
			} else if err := rlp.DecodeBytes(accountBytes, &newAccount); err != nil {
				continue
			}

			// if account not changed, skip
			if oldAccount.Root == newAccount.Root {
				continue
			}
			// if old state is empty, skip
			if oldAccount.Root == emptyRoot || oldAccount.Root == (common.Hash{}) {
				continue
			}

			// create account difference iterator
			oldAccountTrie := newObject(oldDB, addr, oldAccount).getTrie(oldDB.db)
			newAccountTrie := newObject(newDB, addr, newAccount).getTrie(newDB.db)
			newTrieIt := newAccountTrie.NodeIterator(nil)
			oldTrieIt := oldAccountTrie.NodeIterator(nil)
			accountDifferIt, _ := trie.NewDifferenceIterator(newTrieIt, oldTrieIt)

			for accountDifferIt.Next(true) {
				if !accountDifferIt.Leaf() {
					count++
					batch.Delete(accountDifferIt.Hash().Bytes())
				} else if !newTrieIt.Leaf() {
					batch.Delete(append(append([]byte{}, secureKeyPrefix...), accountDifferIt.LeafKey()...))
				}
			}
		}
	}
	return count, nil
}
