package state

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/harmony-one/harmony/internal/shardchain/tikv_manage"
	"sync"
	"sync/atomic"
)

var secureKeyPrefix = []byte("secure-key-")

func (s *DB) DiffAndCleanCache(shardId uint32, to *DB) (int, error) {
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
			atomic.AddUint64(&count, 1)
			_ = batch.Delete(it.Hash().Bytes())
		} else {
			addrBytes := s.trie.GetKey(it.LeafKey())
			addr := common.BytesToAddress(addrBytes)

			var fromAccount, toAccount Account
			if err := rlp.DecodeBytes(it.LeafBlob(), &fromAccount); err != nil {
				continue
			}

			if toByte, err := to.trie.TryGet(addrBytes); err != nil {
				continue
			} else if err := rlp.DecodeBytes(toByte, &toAccount); err != nil {
				continue
			}

			if bytes.Compare(fromAccount.Root.Bytes(), toAccount.Root.Bytes()) == 0 {
				continue
			}

			fromAccountTrie := newObject(s, addr, fromAccount).getTrie(s.db)
			toAccountTrie := newObject(to, addr, toAccount).getTrie(to.db)
			accountIt, _ := trie.NewDifferenceIterator(toAccountTrie.NodeIterator(nil), fromAccountTrie.NodeIterator(nil))

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
