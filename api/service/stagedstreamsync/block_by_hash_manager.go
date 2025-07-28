package stagedstreamsync

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
	sttypes "github.com/harmony-one/harmony/p2p/stream/types"
	"github.com/pkg/errors"
)

type getBlocksByHashManager struct {
	hashes    []common.Hash
	pendings  map[common.Hash]struct{}
	results   map[common.Hash]blockResult
	whitelist []sttypes.StreamID

	lock sync.RWMutex
}

func newGetBlocksByHashManager(hashes []common.Hash, whitelist []sttypes.StreamID) *getBlocksByHashManager {
	return &getBlocksByHashManager{
		hashes:    hashes,
		pendings:  make(map[common.Hash]struct{}),
		results:   make(map[common.Hash]blockResult),
		whitelist: whitelist,
	}
}

func (m *getBlocksByHashManager) getNextHashes() ([]common.Hash, []sttypes.StreamID, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	num := m.numBlocksPerRequest()
	hashes := make([]common.Hash, 0, num)
	if len(m.whitelist) == 0 {
		return nil, nil, ErrEmptyWhitelist
	}

	for _, hash := range m.hashes {
		if len(hashes) == num {
			break
		}
		_, ok1 := m.pendings[hash]
		_, ok2 := m.results[hash]
		if !ok1 && !ok2 {
			hashes = append(hashes, hash)
		}
	}
	sts := make([]sttypes.StreamID, len(m.whitelist))
	copy(sts, m.whitelist)
	return hashes, sts, nil
}

func (m *getBlocksByHashManager) numBlocksPerRequest() int {
	val := divideCeil(len(m.hashes), len(m.whitelist))
	if val < BlockByHashesLowerCap {
		val = BlockByHashesLowerCap
	}
	if val > BlockByHashesUpperCap {
		val = BlockByHashesUpperCap
	}
	return val
}

func (m *getBlocksByHashManager) numRequests() int {
	return divideCeil(len(m.hashes), m.numBlocksPerRequest())
}

func (m *getBlocksByHashManager) addResult(hashes []common.Hash, blocks []*types.Block, stid sttypes.StreamID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for i, hash := range hashes {
		block := blocks[i]
		delete(m.pendings, hash)
		m.results[hash] = blockResult{
			block: block,
			stid:  stid,
		}
	}
}

func (m *getBlocksByHashManager) handleResultError(hashes []common.Hash, stid sttypes.StreamID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.removeStreamFromWhitelist(stid)

	for _, hash := range hashes {
		delete(m.pendings, hash)
	}
}

func (m *getBlocksByHashManager) getResults() ([]*types.Block, []sttypes.StreamID, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	blocks := make([]*types.Block, 0, len(m.hashes))
	stids := make([]sttypes.StreamID, 0, len(m.hashes))
	for _, hash := range m.hashes {
		if m.results[hash].block == nil {
			return nil, nil, errors.New("SANITY: nil block found")
		}
		blocks = append(blocks, m.results[hash].block)
		stids = append(stids, m.results[hash].stid)
	}
	return blocks, stids, nil
}

func (m *getBlocksByHashManager) isDone() bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.results) == len(m.hashes)
}

func (m *getBlocksByHashManager) removeStreamFromWhitelist(target sttypes.StreamID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	newWhitelist := m.whitelist[:0]
	for _, stid := range m.whitelist {
		if stid != target {
			newWhitelist = append(newWhitelist, stid)
		}
	}
	m.whitelist = newWhitelist
}
