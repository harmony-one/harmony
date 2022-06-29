package core

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/internal/mmr"
	"github.com/harmony-one/harmony/internal/mmr/db"
	"github.com/harmony-one/harmony/shard"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

type OpenMmrDB func(epoch *big.Int, create bool) *mmr.Mmr

type BlockchainMMR struct {
	bc            *BlockChain
	cache         map[uint64]*mmr.Mmr
	cacheLock     sync.RWMutex
	MemMmrDB      *mmr.Mmr
	MmrDBLock     sync.RWMutex
	MmrCommitChan chan *mmr.Mmr
	quit          chan struct{}
	wait          sync.WaitGroup
}

func newMemMmrDB(epoch *big.Int) *mmr.Mmr {
	leafLength := shard.Schedule.InstanceForEpoch(epoch).BlocksPerEpoch()
	// for mainnet, mmrdb memory per epoch around 32 * (2*32768-1) = 2MB
	totalNodes := 2*leafLength - 1
	return mmr.New(db.NewMemorybaseddb(0, make(map[int64][]byte, totalNodes)), epoch)
}

func newMMR(bc *BlockChain) *BlockchainMMR {
	curHeader := bc.CurrentHeader()
	curEpoch := curHeader.Epoch()
	initEpoch := curEpoch
	if curEpoch.Cmp(bc.Config().MMRHeaderEpoch) < 0 {
		initEpoch = bc.Config().MMRHeaderEpoch
	}
	bm := &BlockchainMMR{
		bc:       bc,
		cache:    map[uint64]*mmr.Mmr{},
		MemMmrDB: newMemMmrDB(initEpoch),
		quit:     bc.quit,
	}
	if bc.Config().IsMMRHeaderEpoch(curEpoch) && !curHeader.IsLastBlockInEpoch() {
		headers := make([]*block.Header, 0)
		headers = append(headers, curHeader)
		for header := curHeader; header.Epoch().Cmp(curEpoch) == 0; {
			if header.Number().Sign() == 0 {
				continue
			}
			header = bc.GetHeaderByHash(header.ParentHash())
			headers = append(headers, header)
		}

		for i := 0; i < len(headers); i++ {
			bm.computeAndCheckNewMMRRoot(headers[len(headers)-i-1])
		}
	}

	if nodeconfig.GetDefaultConfig().MmrDbEnable {
		bm.cache = make(map[uint64]*mmr.Mmr, 4)
		bm.MmrCommitChan = make(chan *mmr.Mmr, 4)
		bm.wait.Add(1)
		go func() {
			bm.FileDbMmrSync()
			bm.wait.Done()
		}()
	}
	return bm
}

func (bm *BlockchainMMR) copyMemMmrDB(epoch *big.Int) *mmr.Mmr {
	var memdb *db.Memorybaseddb
	bm.MmrDBLock.RLock()
	inMem := epoch.Cmp(bm.MemMmrDB.Epoch) == 0
	if inMem {
		memdb = db.NewMemorybaseddb(bm.MemMmrDB.GetLeafLength(), make(map[int64][]byte, bm.MemMmrDB.GetNodeLength()))
		bm.MemMmrDB.Copy(memdb)
	}
	bm.MmrDBLock.RUnlock()
	bm.cacheLock.RLock()
	cacheDB, inCache := bm.cache[epoch.Uint64()]
	bm.cacheLock.RUnlock()
	if inCache {
		memdb = db.NewMemorybaseddb(cacheDB.GetLeafLength(), make(map[int64][]byte, cacheDB.GetNodeLength()))
		cacheDB.Copy(memdb)
	}
	if !inMem && !inCache {
		fileDb := bm.bc.mmrEpochDB(epoch, false)
		if fileDb == nil {
			return nil
		}
		memdb = db.NewMemorybaseddb(fileDb.GetLeafLength(), make(map[int64][]byte, fileDb.GetNodeLength()))
		fileDb.Copy(memdb)
		fileDb.Db().Close()
	}
	return mmr.New(memdb, epoch)
}

func (bm *BlockchainMMR) GetProofOfBlockFrom(mmrToLook *mmr.Mmr, blockNumber, withRespectTo uint64) *mmr.MmrProof {

	proof := mmr.MmrProof{}
	// fetch the first blockHash and the corresponding blockNumber
	if firstBlockHash, exists := mmrToLook.GetUnverified(0); exists {
		firstBlock := bm.bc.GetBlockByHash(common.BytesToHash(firstBlockHash))
		index := blockNumber - firstBlock.NumberU64()
		leafLength := uint64(mmrToLook.GetLeafLength())

		refIndex := withRespectTo - firstBlock.NumberU64() + 1
		if refIndex > leafLength {
			refIndex = leafLength
		}

		leafIndexes := []int64{int64(index)}
		prunedMmr := mmrToLook.GetProof(leafIndexes, int64(refIndex))
		proof.Root, proof.Width, proof.Index, proof.Peaks, proof.Siblings = prunedMmr.GetMerkleProof(leafIndexes[0])
	}
	return &proof
}

func (bm *BlockchainMMR) GetProof(blockHash common.Hash, withRespectTo uint64) (*mmr.MmrProof, error) {
	block := bm.bc.GetBlockByHash(blockHash)
	epoch := block.Epoch()
	dbMmr := bm.copyMemMmrDB(epoch)
	if dbMmr == nil {
		return nil, errors.New("not found mmr db")
	}
	return bm.GetProofOfBlockFrom(dbMmr, block.NumberU64(), withRespectTo), nil
}

// commit memdb to disk
func (bm *BlockchainMMR) FileDbMmrSync() {
	for {
		select {
		case memMmrDB := <-bm.MmrCommitChan:
			fileDbMmr := bm.bc.mmrEpochDB(memMmrDB.Epoch, true)
			memMmrDB.Copy(fileDbMmr.Db())
			fileDbMmr.Db().Close()
			bm.removeCache(memMmrDB)
		case <-bm.quit:
			return
		}
	}
}

func (bm *BlockchainMMR) computeNewMMRRoot(header *block.Header) (common.Hash, error) {
	// skip genesis
	if header.Number().Cmp(big.NewInt(0)) == 0 {
		return common.Hash{}, nil
	}
	memDb := bm.copyMemMmrDB(header.Epoch())
	if memDb == nil {
		return common.Hash{}, errors.New("no mmr db")
	}

	parent := bm.bc.GetHeaderByHash(header.ParentHash())
	if leafLength := memDb.GetLeafLength(); leafLength > 0 {
		lastBlockHash, _ := memDb.GetUnverified(leafLength - 1)
		lastHeader := bm.bc.GetHeaderByHash(common.BytesToHash(lastBlockHash))
		diff := new(big.Int).Sub(header.Number(), lastHeader.Number()).Int64() - 2
		if diff < 0 {
			newLeafLength := leafLength + diff
			memDb.Db().SetLeafLength(leafLength + diff)
			lastBlockHash, _ = memDb.GetUnverified(newLeafLength - 1)
		}
		if parent.ParentHash() != common.BytesToHash(lastBlockHash) {
			return common.Hash{}, fmt.Errorf("BlockchainMMR block mismatch! insert.Number:%d insert.Parent:%x lastMMRBlock:%x", header.Number().Uint64(), header.ParentHash(), lastBlockHash)
		}
	}
	memDb.Append(header.ParentHash().Bytes())
	return common.BytesToHash(memDb.GetRoot()), nil
}

func (bm *BlockchainMMR) addCache(memMmrDB *mmr.Mmr) {
	bm.cacheLock.Lock()
	bm.cache[memMmrDB.Epoch.Uint64()] = memMmrDB
	bm.cacheLock.Unlock()
}

func (bm *BlockchainMMR) removeCache(memMmrDB *mmr.Mmr) {
	bm.cacheLock.Lock()
	delete(bm.cache, memMmrDB.Epoch.Uint64())
	bm.cacheLock.Unlock()
}

// only this function can change MemMmrDB(except NewMMR)
func (bm *BlockchainMMR) computeAndCheckNewMMRRoot(header *block.Header) error {
	// skip genesis
	if header.Number().Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	resetMemDb := func(epoch *big.Int) {
		bm.MmrDBLock.Lock()
		defer bm.MmrDBLock.Unlock()
		bm.MemMmrDB = newMemMmrDB(epoch)
	}

	parent := bm.bc.GetHeaderByHash(header.ParentHash())

	if leafLength := bm.MemMmrDB.GetLeafLength(); leafLength > 0 {
		if lastBlockHash, _ := bm.MemMmrDB.GetUnverified(leafLength - 1); parent.ParentHash() != common.BytesToHash(lastBlockHash) {
			return fmt.Errorf("BlockchainMMR block mismatch! insert.Number:%d insert.Parent:%x lastMMRBlock:%x", header.Number().Uint64(), header.Hash(), lastBlockHash)
		}
	}

	bm.MmrDBLock.Lock()
	// append the latest parent hash to mmr
	bm.MemMmrDB.Append(header.ParentHash().Bytes())
	if header.MMRRoot() != common.BytesToHash(bm.MemMmrDB.GetRoot()) {
		bm.MemMmrDB.Delete(bm.MemMmrDB.GetLeafLength() - 1)
		bm.MmrDBLock.Unlock()
		return errors.New("MMRRoot not equal")
	}
	bm.MmrDBLock.Unlock()

	isLastBlockInEpoch := header.IsLastBlockInEpoch()
	if bm.MmrCommitChan != nil {
		memMmrDB := bm.copyMemMmrDB(header.Epoch())
		bm.addCache(memMmrDB)
		// send the header for updating FileDbMmr
		bm.MmrCommitChan <- memMmrDB
	}
	// check if the memDbMmr needs to be reset or synced
	if isLastBlockInEpoch {
		newEpoch := new(big.Int).Add(header.Epoch(), common.Big1)
		resetMemDb(newEpoch)
	}

	return nil
}

func (bm *BlockchainMMR) Close() {
	bm.wait.Wait()
}
