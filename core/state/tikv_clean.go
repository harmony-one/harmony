package state

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/internal/shardchain/tikv_manage"
)

var secureKeyPrefix = []byte("secure-key-")

// DiffAndCleanCache clean block tire data from redis, Used to reduce redis storage and increase hit rate
func (s *DB) DiffAndCleanCache(shardId uint32, to *DB) (int, error) {
	// create difference iterator
	it, _ := trie.NewDifferenceIterator(to.trie.NodeIterator(nil), s.trie.NodeIterator(nil))
	db, err := tikv_manage.GetDefaultTiKVFactory().NewCacheStateDB(shardId)
	if err != nil {
		return 0, err
	}

	batch := db.NewBatch()
	wg := &sync.WaitGroup{}
	count := uint64(0)
	for it.Next(true) {
		if !it.Leaf() {
			// delete it if trie leaf node
			atomic.AddUint64(&count, 1)
			_ = batch.Delete(it.Hash().Bytes())
		} else {

			// build account data
			addrBytes := s.trie.GetKey(it.LeafKey())
			addr := common.BytesToAddress(addrBytes)

			var fromAccount, toAccount types.StateAccount
			if err := rlp.DecodeBytes(it.LeafBlob(), &fromAccount); err != nil {
				continue
			}

			if toByte, err := to.trie.TryGet(addrBytes); err != nil {
				continue
			} else if err := rlp.DecodeBytes(toByte, &toAccount); err != nil {
				continue
			}

			// if account not changed, skip
			if bytes.Compare(fromAccount.Root.Bytes(), toAccount.Root.Bytes()) == 0 {
				continue
			}

			// create account difference iterator
			fromAccountTrie, errFromAcc := newObject(s, addr, fromAccount).getTrie(s.db)
			if errFromAcc != nil {
				continue
			}
			toAccountTrie, errToAcc := newObject(to, addr, toAccount).getTrie(to.db)
			if errToAcc != nil {
				continue
			}
			accountIt, _ := trie.NewDifferenceIterator(toAccountTrie.NodeIterator(nil), fromAccountTrie.NodeIterator(nil))

			// parallel to delete data
			wg.Add(1)
			go func() {
				defer wg.Done()
				for accountIt.Next(true) {
					atomic.AddUint64(&count, 1)

					if !accountIt.Leaf() {
						_ = batch.Delete(accountIt.Hash().Bytes())
					} else {
						_ = batch.Delete(append(append([]byte{}, secureKeyPrefix...), accountIt.LeafKey()...))
					}
				}
			}()
		}
	}

	wg.Wait()

	return int(count), db.ReplayCache(batch)
}
