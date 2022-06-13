package node

import (
	"bytes"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/internal/mmr"
	"github.com/zmitton/go-merklemountainrange/db"
)

func (node *Node) GetProofOfBlockFrom(mmrToLook *mmr.Mmr, blockNumber, withRespectTo uint64) mmr.MmrProof {
	proof := mmr.MmrProof{}
	// fetch the first blockHash and the corresponding blockNumber
	if firstBlockHash, exists := mmrToLook.GetUnverified(0); exists {
		firstBlock := node.Blockchain().GetBlockByHash(common.BytesToHash(firstBlockHash))
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
	return proof
}

func (node *Node) GetProof(hash common.Hash, blockHash common.Hash, blockNumber, withRespectTo uint64) mmr.MmrProof {
	block := node.Blockchain().GetBlock(blockHash, blockNumber)
	epoch := block.Epoch()
	shardID := block.ShardID()

	var dbMmr *mmr.Mmr
	// check if the current memDbMmr has the blockHash
	if node.MemDbMmr.Epoch.Cmp(epoch) == 0 && node.MemDbMmr.ShardID == block.ShardID() {
		dbMmr = node.MemDbMmr
	} else {
		// fetch from history
		dbMmr = mmr.New(
			mmr.Keccak256,
			db.OpenFilebaseddb(
				node.mmrDbFileName(
					shardID,
					epoch,
				),
			),
			epoch,
			shardID,
		)
		if dbMmr == nil {
			panic("could not load mmr db")
		}
	}
	return node.GetProofOfBlockFrom(dbMmr, blockNumber, withRespectTo)
}

func (node *Node) mmrDbFileName(shardID uint32, epoch *big.Int) string {
	return fmt.Sprintf(
		"%s/%s-%s-shard-%d-epoch-%s%s",
		node.NodeConfig.MmrDbDir,
		node.SelfPeer.IP,
		node.SelfPeer.Port,
		shardID,
		epoch,
		".mmr",
	)
}

func (node *Node) memDbMmrInSync(header *block.Header) bool {
	parent := node.Blockchain().GetHeaderByHash(header.ParentHash())
	parentOfParent := parent.ParentHash()

	leafLength := node.MemDbMmr.GetLeafLength()

	if leafLength == 0 || leafLength == 1 {
		return true
	}
	if lastLeaf, exists := node.MemDbMmr.GetUnverified(leafLength - 1); exists {
		if bytes.Equal(lastLeaf, parentOfParent.Bytes()) {
			return true
		}
	}
	return false
}

func (node *Node) fileDbMmrInSync(item FileDbMmrChanItem) bool {
	parent := node.Blockchain().GetHeaderByHash(item.ParentHash)
	parentOfParent := parent.ParentHash()

	leafLength := node.FileDbMmr.GetLeafLength()
	if leafLength == 0 || leafLength == 1 {
		return true
	}
	if lastLeaf, exists := node.FileDbMmr.GetUnverified(leafLength - 1); exists {
		if bytes.Equal(lastLeaf, parentOfParent.Bytes()) {
			return true
		}
	}
	return false
}

func (node *Node) initializeFileDBMmr(shardID uint32, epoch *big.Int) *mmr.Mmr {
	// check if the mmr directory exists
	if _, err := os.Stat(node.NodeConfig.MmrDbDir); os.IsNotExist(err) {
		err := os.MkdirAll(node.NodeConfig.MmrDbDir, 0777)
		if err != nil {
			panic("mmrDir is missing")
		}
	}

	return mmr.New(
		mmr.Keccak256,
		db.CreateFilebaseddb(
			node.mmrDbFileName(
				shardID,
				epoch,
			),
			32,
		),
		epoch,
		shardID,
	)
}

func (node *Node) initializeMemDBMmr(shardID uint32, epoch *big.Int) *mmr.Mmr {
	return mmr.New(
		mmr.Keccak256,
		db.NewMemorybaseddb(0, map[int64][]byte{}),
		epoch,
		shardID,
	)
}

func (node *Node) FileDbMmrSync() {
	for {
		item := <-node.MmrBlockHeaderChan
		parentHash := item.ParentHash

		// load FileDbMmr if nil
		if node.FileDbMmr == nil {
			node.FileDbMmr = mmr.New(
				mmr.Keccak256,
				db.OpenFilebaseddb(
					node.mmrDbFileName(
						item.ShardID,
						item.Epoch,
					),
				),
				item.Epoch,
				item.ShardID,
			)
		}
		// check if the FileDbMmr is in sync
		if !node.fileDbMmrInSync(item) {
			// sync first
			panic("fileDbMmr out of sync")
		}

		node.FileDbMmr.Append(parentHash.Bytes())

		// if last block of current epoch, create file db for next epoch
		if item.IsLastBlock {
			newEpoch := new(big.Int).Set(item.Epoch)
			// increment the epoch to create db for next epoch
			newEpoch = newEpoch.Add(newEpoch, common.Big1)
			node.FileDbMmr = node.initializeFileDBMmr(item.ShardID, newEpoch)
		}
	}
}

func (node *Node) resetMemDbMmr(header *block.Header) {
	newEpoch := new(big.Int).Set(header.Epoch())
	newEpoch = newEpoch.Add(newEpoch, common.Big1)

	node.MemDbMmr = node.initializeMemDBMmr(header.ShardID(), newEpoch)
}

func (node *Node) computeAndUpdateNewMMRRoot(header *block.Header, isLastBlockOfEpoch bool) error {
	// skip genesis
	if header.Number().Cmp(big.NewInt(0)) == 0 {
		return nil
	}

	// send the header for updating FileDbMmr
	go func() {
		node.MmrBlockHeaderChan <- FileDbMmrChanItem{
			header.Number(),
			header.ParentHash(),
			header.Epoch(),
			header.ShardID(),
			isLastBlockOfEpoch,
		}
	}()

	if node.MemDbMmr == nil || !node.memDbMmrInSync(header) {
		panic("memDbMmr out of sync")
	}

	// append the latest parent hash to mmr
	node.MemDbMmr.Append(header.ParentHash().Bytes())
	header.SetMMRRoot(node.MemDbMmr.GetRoot())

	// check if the memDbMmr needs to be reset or synced
	if isLastBlockOfEpoch {
		node.resetMemDbMmr(header)
	}

	return nil
}
