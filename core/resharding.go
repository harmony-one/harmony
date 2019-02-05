package core

import (
	"math/rand"
	"sort"
	"strconv"

	"github.com/harmony-one/harmony/core/types"
)

const (
	// InitialSeed is the initial random seed, a magic number to answer everything, remove later
	InitialSeed int64 = 42
)

// ShardingState is data structure hold the sharding state
type ShardingState struct {
	epoch      uint64 // current epoch
	rnd        int64  // random seed for resharding
	numShards  int
	shardState types.ShardState
}

// sortedCommitteeBySize will sort shards by size
// Suppose there are N shards, the first N/2 larger shards are called active committees
// the rest N/2 smaller committees are called inactive committees
// actually they are all just normal shards
func (ss *ShardingState) sortCommitteeBySize() {
	sort.Slice(ss.shardState, func(i, j int) bool {
		return len(ss.shardState[i].NodeList) > len(ss.shardState[j].NodeList)
	})
}

// assignNewNodes add new nodes into the N/2 active committees evenly
func (ss *ShardingState) assignNewNodes(newNodeList []types.NodeID) {
	ss.sortCommitteeBySize()
	numActiveShards := ss.numShards / 2
	Shuffle(newNodeList)
	for i, nid := range newNodeList {
		id := i % numActiveShards
		ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
	}
}

// cuckooResharding uses cuckoo rule to reshard X% of active committee(shards) into inactive committee(shards)
func (ss *ShardingState) cuckooResharding(percent float64) {
	ss.sortCommitteeBySize()
	numActiveShards := ss.numShards / 2
	kickedNodes := []types.NodeID{}
	for i := range ss.shardState {
		if i >= numActiveShards {
			break
		}
		Shuffle(ss.shardState[i].NodeList)
		numKicked := int(percent * float64(len(ss.shardState[i].NodeList)))
		tmp := ss.shardState[i].NodeList[:numKicked]
		kickedNodes = append(kickedNodes, tmp...)
		ss.shardState[i].NodeList = ss.shardState[i].NodeList[numKicked:]
	}

	Shuffle(kickedNodes)
	for i, nid := range kickedNodes {
		id := numActiveShards + i%(ss.numShards-numActiveShards)
		ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
	}
}

// UpdateShardState will first add new nodes into shards, then use cuckoo rule to reshard to get new shard state
func (ss *ShardingState) UpdateShardState(newNodeList []types.NodeID, percent float64) {
	rand.Seed(ss.rnd)
	ss.cuckooResharding(percent)
	ss.assignNewNodes(newNodeList)
}

// Shuffle will shuffle the list with result uniquely determined by seed, assuming there is no repeat items in the list
func Shuffle(list []types.NodeID) {
	sort.Slice(list, func(i, j int) bool {
		return types.CompareNodeID(list[i], list[j]) == -1
	})
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}

// GetBlockNumberFromEpoch calculates the block number where epoch sharding information is stored
func GetBlockNumberFromEpoch(epoch uint64) uint64 {
	number := epoch * uint64(BlocksPerEpoch) // currently we use the first block in each epoch
	return number
}

// GetEpochFromBlockNumber calculates the epoch number the block belongs to
func GetEpochFromBlockNumber(blockNumber uint64) uint64 {
	return blockNumber / uint64(BlocksPerEpoch)
}

// CheckEpochBlock check whethere a given block number is the one to store epoch information
func CheckEpochBlock(blockNumber uint64) bool {
	return blockNumber%uint64(BlocksPerEpoch) == 0
}

// GetPreviousEpochBlockNumber gets the epoch block number of previous epoch
func GetPreviousEpochBlockNumber(blockNumber uint64) uint64 {
	epoch := GetEpochFromBlockNumber(blockNumber)
	if epoch == 1 {
		// no previous epoch
		return epoch
	}
	return GetBlockNumberFromEpoch(epoch - 1)
}

// GetShardingStateFromBlockChain will retrieve random seed and shard map from beacon chain for given a epoch
func GetShardingStateFromBlockChain(bc *BlockChain, epoch uint64) *ShardingState {
	number := GetBlockNumberFromEpoch(epoch)
	shardState := bc.GetShardStateByNumber(number)
	rnd := bc.GetRandSeedByNumber(number)

	return &ShardingState{epoch: epoch, rnd: rnd, shardState: shardState, numShards: len(shardState)}
}

// CalculateNewShardState get sharding state from previous epoch and calcualte sharding state for new epoch
// TODO: currently, we just mock everything
func CalculateNewShardState(bc *BlockChain, epoch uint64) types.ShardState {
	if epoch == 1 {
		return fakeGetInitShardState()
	}
	ss := GetShardingStateFromBlockChain(bc, epoch-1)
	newNodeList := fakeNewNodeList(ss.rnd)
	percent := ss.calculateKickoutRate(newNodeList)
	ss.UpdateShardState(newNodeList, percent)
	return ss.shardState
}

// calculateKickoutRate calculates the cuckoo rule kick out rate in order to make committee balanced
func (ss *ShardingState) calculateKickoutRate(newNodeList []types.NodeID) float64 {
	numActiveCommittees := ss.numShards / 2
	newNodesPerShard := len(newNodeList) / numActiveCommittees
	ss.sortCommitteeBySize()
	return float64(newNodesPerShard) / float64(len(ss.shardState[numActiveCommittees].NodeList))
}

// FakeGenRandSeed generate random seed based on previous rnd seed; remove later after VRF implemented
func FakeGenRandSeed(seed int64) int64 {
	rand.Seed(seed)
	return rand.Int63()
}

// remove later after bootstrap codes ready
func fakeGetInitShardState() types.ShardState {
	rand.Seed(InitialSeed)
	shardState := types.ShardState{}
	for i := 0; i < 6; i++ {
		sid := uint32(i)
		com := types.Committee{ShardID: sid}
		for j := 0; j < 10; j++ {
			nid := strconv.Itoa(int(rand.Int63()))
			com.NodeList = append(com.NodeList, types.NodeID(nid))
		}
		shardState = append(shardState, com)
	}
	return shardState
}

// remove later after new nodes list generation ready
func fakeNewNodeList(seed int64) []types.NodeID {
	rand.Seed(seed)
	numNewNodes := rand.Intn(10)
	nodeList := []types.NodeID{}
	for i := 0; i < numNewNodes; i++ {
		nid := strconv.Itoa(int(rand.Int63()))
		nodeList = append(nodeList, types.NodeID(nid))
	}
	return nodeList
}
