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

		lock sync.Mutex
	}
)

func newBlockHashResults(bns []uint64) *blockHashResults {
	results := make([]map[sttypes.StreamID]common.Hash, 0, len(bns))
	for range bns {
		results = append(results, make(map[sttypes.StreamID]common.Hash))
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
	return
}

func (res *blockHashResults) computeLongestHashChain() ([]common.Hash, []sttypes.StreamID) {
	var (
		whitelist map[sttypes.StreamID]struct{}
		hashChain []common.Hash
	)
	for _, result := range res.results {
		hash, nextWl := countHashMaxVote(result, whitelist)
		if hash == emptyHash {
			break
		}
		hashChain = append(hashChain, hash)
		whitelist = nextWl
	}

	sts := make([]sttypes.StreamID, 0, len(whitelist))
	for st := range whitelist {
		sts = append(sts, st)
	}
	return hashChain, sts
}

func (res *blockHashResults) numBlocksWithResults() int {
	res.lock.Lock()
	defer res.lock.Unlock()

	cnt := 0
	for _, result := range res.results {
		if len(result) != 0 {
			cnt++
		}
	}
	return cnt
}
