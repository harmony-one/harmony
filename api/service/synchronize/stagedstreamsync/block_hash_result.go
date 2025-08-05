package stagedstreamsync

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
)

type (
	blockHashResults struct {
		bns     []uint64
		results []map[sttypes.StreamID]common.Hash

		lock sync.RWMutex
	}
)

func newBlockHashResults(bns []uint64) *blockHashResults {
	results := make([]map[sttypes.StreamID]common.Hash, len(bns))
	for i := range bns {
		results[i] = make(map[sttypes.StreamID]common.Hash)
	}
	return &blockHashResults{
		bns:     bns,
		results: results,
	}
}

func (res *blockHashResults) addResult(hashes []common.Hash, stid sttypes.StreamID) {
	res.lock.Lock()
	defer res.lock.Unlock()

	for i, h := range hashes {
		if h == emptyHash {
			return // nil block hash reached
		}
		res.results[i][stid] = h
	}
}

func (res *blockHashResults) computeLongestHashChain() ([]common.Hash, []sttypes.StreamID) {
	res.lock.RLock()
	defer res.lock.RUnlock()

	var (
		whitelist map[sttypes.StreamID]struct{}
		hashChain []common.Hash
	)

	for _, result := range res.results {
		hash, nextWl := countHashMaxVote(result, whitelist)
		if hash == emptyHash {
			break // Exit if emptyHash is encountered
		}
		hashChain = append(hashChain, hash)
		// add nextWl stream IDs to whitelist
		if len(nextWl) > 0 {
			whitelist = make(map[sttypes.StreamID]struct{}, len(nextWl))
			for st := range nextWl {
				whitelist[st] = struct{}{}
			}
		}
	}

	sts := make([]sttypes.StreamID, 0)
	for st := range whitelist {
		sts = append(sts, st)
	}
	return hashChain, sts
}

func (res *blockHashResults) numBlocksWithResults() int {
	res.lock.RLock()
	defer res.lock.RUnlock()

	cnt := 0
	for _, result := range res.results {
		if len(result) != 0 {
			cnt++
		}
	}
	return cnt
}
