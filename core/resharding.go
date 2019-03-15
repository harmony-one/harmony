package core

import (
	"encoding/binary"
	"encoding/hex"
	"math/rand"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/contract"

	"github.com/harmony-one/harmony/core/types"
)

const (
	// InitialSeed is the initial random seed, a magic number to answer everything, remove later
	InitialSeed uint32 = 42
	// GenesisEpoch is the number of the genesis epoch.
	GenesisEpoch = 0
	// FirstEpoch is the number of the first epoch.
	FirstEpoch = 1
	// GenesisShardNum is the number of shard at genesis
	GenesisShardNum = 4
	// GenesisShardSize is the size of each shard at genesis
	GenesisShardSize = 10
	// CuckooRate is the percentage of nodes getting reshuffled in the second step of cuckoo resharding.
	CuckooRate = 0.1
)

// ShardingState is data structure hold the sharding state
type ShardingState struct {
	epoch      uint64 // current epoch
	rnd        uint64 // random seed for resharding
	numShards  int
	shardState types.ShardState
}

// sortedCommitteeBySize will sort shards by size
// Suppose there are N shards, the first N/2 larger shards are called active committees
// the rest N/2 smaller committees are called inactive committees
// actually they are all just normal shards
// TODO: sort the committee weighted by total staking instead of shard size
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
	numActiveShards := ss.numShards / 2
	kickedNodes := []types.NodeID{}
	for i := range ss.shardState {
		if i >= numActiveShards {
			break
		}
		numKicked := int(percent * float64(len(ss.shardState[i].NodeList)))
		if numKicked == 0 {
			numKicked++ // At least kick one node out
		}
		length := len(ss.shardState[i].NodeList)
		if length-numKicked <= 0 {
			continue // Never empty a shard
		}
		tmp := ss.shardState[i].NodeList[length-numKicked:]
		kickedNodes = append(kickedNodes, tmp...)
		ss.shardState[i].NodeList = ss.shardState[i].NodeList[:length-numKicked]
	}

	Shuffle(kickedNodes)
	numInactiveShards := ss.numShards - numActiveShards
	for i, nid := range kickedNodes {
		id := numActiveShards + i%numInactiveShards
		ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
	}
}

// assignLeaders will first add new nodes into shards, then use cuckoo rule to reshard to get new shard state
func (ss *ShardingState) assignLeaders() {
	for i := 0; i < ss.numShards; i++ {
		// At genesis epoch, the shards are empty.
		if len(ss.shardState[i].NodeList) > 0 {
			Shuffle(ss.shardState[i].NodeList)
			ss.shardState[i].Leader = ss.shardState[i].NodeList[0]
		}
	}
}

// Reshard will first add new nodes into shards, then use cuckoo rule to reshard to get new shard state
func (ss *ShardingState) Reshard(newNodeList []types.NodeID, percent float64) {
	rand.Seed(int64(ss.rnd))
	ss.sortCommitteeBySize()
	// TODO: separate shuffling and leader assignment
	ss.assignLeaders()
	ss.assignNewNodes(newNodeList)
	ss.cuckooResharding(percent)
}

// Shuffle will shuffle the list with result uniquely determined by seed, assuming there is no repeat items in the list
func Shuffle(list []types.NodeID) {
	// Sort to make sure everyone will generate the same with the same rand seed.
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

// GetShardingStateFromBlockChain will retrieve random seed and shard map from beacon chain for given a epoch
func GetShardingStateFromBlockChain(bc *BlockChain, epoch uint64) *ShardingState {
	number := GetBlockNumberFromEpoch(epoch)
	shardState := bc.GetShardStateByNumber(number)

	rndSeedBytes := bc.GetRandSeedByNumber(number)
	rndSeed := binary.BigEndian.Uint64(rndSeedBytes[:])

	return &ShardingState{epoch: epoch, rnd: rndSeed, shardState: shardState, numShards: len(shardState)}
}

// CalculateNewShardState get sharding state from previous epoch and calculate sharding state for new epoch
func CalculateNewShardState(bc *BlockChain, epoch uint64, stakeInfo *map[common.Address]*structs.StakeInfo) types.ShardState {
	if epoch == GenesisEpoch {
		return GetInitShardState()
	}
	ss := GetShardingStateFromBlockChain(bc, epoch-1)
	if epoch == FirstEpoch {
		newNodes := []types.NodeID{}
		for addr, stakeInfo := range *stakeInfo {
			newNodes = append(newNodes, types.NodeID{addr.Hex(), hex.EncodeToString(stakeInfo.BlsAddress[:])})
		}
		rand.Seed(int64(ss.rnd))
		Shuffle(newNodes)
		utils.GetLogInstance().Info("[resharding] New Nodes", "data", newNodes)
		for i, nid := range newNodes {
			id := i%(GenesisShardNum-1) + 1 // assign the node to one of the empty shard
			ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
		}
		utils.GetLogInstance().Info("State", "data", ss)
		return ss.shardState
	}
	newNodeList := ss.UpdateShardingState(stakeInfo)
	utils.GetLogInstance().Info("Cuckoo Rate", "percentage", CuckooRate)
	ss.Reshard(newNodeList, CuckooRate)
	return ss.shardState
}

// UpdateShardingState remove the unstaked nodes and returns the newly staked node Ids.
func (ss *ShardingState) UpdateShardingState(stakeInfo *map[common.Address]*structs.StakeInfo) []types.NodeID {
	oldAddresses := make(map[string]bool) // map of bls addresses
	for _, shard := range ss.shardState {
		newNodeList := shard.NodeList[:0]
		for _, nodeID := range shard.NodeList {
			oldAddresses[nodeID.BlsAddress] = true
			_, ok := (*stakeInfo)[common.HexToAddress(nodeID.EcdsaAddress)]
			if ok {
				newNodeList = append(newNodeList, nodeID)
			} else {
				// Remove the node if it's no longer staked
			}
		}
		shard.NodeList = newNodeList
	}

	newAddresses := []types.NodeID{}
	for addr, info := range *stakeInfo {
		_, ok := oldAddresses[addr.Hex()]
		if !ok {
			newAddresses = append(newAddresses, types.NodeID{hex.EncodeToString(info.BlsAddress[:]), addr.Hex()})
		}
	}
	return newAddresses
}

// GetInitShardState returns the initial shard state at genesis.
// TODO: make the deploy.sh config file in sync with genesis constants.
func GetInitShardState() types.ShardState {
	shardState := types.ShardState{}
	for i := 0; i < GenesisShardNum; i++ {
		com := types.Committee{ShardID: uint32(i)}
		if i == 0 {
			for j := 0; j < GenesisShardSize; j++ {
				priKey := bls.SecretKey{}
				priKey.SetHexString(contract.InitialBeaconChainBLSAccounts[j].Private)
				addrBytes := priKey.GetPublicKey().GetAddress()
				blsAddr := hex.EncodeToString(addrBytes[:])
				// TODO: directly read address for bls too
				com.NodeList = append(com.NodeList, types.NodeID{blsAddr, contract.InitialBeaconChainAccounts[j].Address})
			}
		}
		shardState = append(shardState, com)
	}
	return shardState
}
