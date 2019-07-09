package core

import (
	"encoding/binary"
	"errors"
	"math/big"
	"math/rand"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	// GenesisEpoch is the number of the genesis epoch.
	GenesisEpoch = 0
	// FirstEpoch is the number of the first epoch.
	FirstEpoch = 1
	// CuckooRate is the percentage of nodes getting reshuffled in the second step of cuckoo resharding.
	CuckooRate = 0.1
)

// ShardingState is data structure hold the sharding state
type ShardingState struct {
	epoch      uint64 // current epoch
	rnd        uint64 // random seed for resharding
	numShards  int    // TODO ek – equal to len(shardState); remove this
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
		id := 0
		if numActiveShards > 0 {
			id = i % numActiveShards
		}
		if id < len(ss.shardState) {
			ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
		} else {
			utils.Logger().Error().Int("id", id).Int("shardState Count", len(ss.shardState)).Msg("assignNewNodes index out of range")
		}
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
		id := numActiveShards
		if numInactiveShards > 0 {
			id += i % numInactiveShards
		}
		ss.shardState[id].NodeList = append(ss.shardState[id].NodeList, nid)
	}
}

// Reshard will first add new nodes into shards, then use cuckoo rule to reshard to get new shard state
func (ss *ShardingState) Reshard(newNodeList []types.NodeID, percent float64) {
	rand.Seed(int64(ss.rnd))
	ss.sortCommitteeBySize()

	// Take out and preserve leaders
	leaders := []types.NodeID{}
	for i := 0; i < ss.numShards; i++ {
		if len(ss.shardState[i].NodeList) > 0 {
			leaders = append(leaders, ss.shardState[i].NodeList[0])
			ss.shardState[i].NodeList = ss.shardState[i].NodeList[1:]
			// Also shuffle the rest of the nodes
			Shuffle(ss.shardState[i].NodeList)
		}
	}

	ss.assignNewNodes(newNodeList)
	ss.cuckooResharding(percent)

	// Put leader back
	if len(leaders) < ss.numShards {
		utils.Logger().Error().Msg("Not enough leaders to assign to shards")
	}
	for i := 0; i < ss.numShards; i++ {
		ss.shardState[i].NodeList = append([]types.NodeID{leaders[i]}, ss.shardState[i].NodeList...)
	}
}

// Shuffle will shuffle the list with result uniquely determined by seed, assuming there is no repeat items in the list
func Shuffle(list []types.NodeID) {
	// Sort to make sure everyone will generate the same with the same rand seed.
	sort.Slice(list, func(i, j int) bool {
		return types.CompareNodeIDByBLSKey(list[i], list[j]) == -1
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

// GetLastBlockNumberFromEpoch calculates the last block number for the given
// epoch.  TODO ek – this is a temp hack.
func GetLastBlockNumberFromEpoch(epoch uint64) uint64 {
	return (epoch+1)*BlocksPerEpoch - 1
}

// GetEpochFromBlockNumber calculates the epoch number the block belongs to
func GetEpochFromBlockNumber(blockNumber uint64) uint64 {
	return blockNumber / uint64(BlocksPerEpoch)
}

// GetShardingStateFromBlockChain will retrieve random seed and shard map from beacon chain for given a epoch
func GetShardingStateFromBlockChain(bc *BlockChain, epoch *big.Int) (*ShardingState, error) {
	if bc == nil {
		return nil, errors.New("no blockchain is supplied to get shard state")
	}
	shardState, err := bc.ReadShardState(epoch)
	if err != nil {
		return nil, err
	}
	shardState = shardState.DeepCopy()

	blockNumber := GetBlockNumberFromEpoch(epoch.Uint64())
	rndSeedBytes := bc.GetVdfByNumber(blockNumber)
	rndSeed := binary.BigEndian.Uint64(rndSeedBytes[:])

	return &ShardingState{epoch: epoch.Uint64(), rnd: rndSeed, shardState: shardState, numShards: len(shardState)}, nil
}

// CalculateNewShardState get sharding state from previous epoch and calculate sharding state for new epoch
func CalculateNewShardState(
	bc *BlockChain, epoch *big.Int,
	stakeInfo *map[common.Address]*structs.StakeInfo,
) (types.ShardState, error) {
	if epoch.Cmp(big.NewInt(GenesisEpoch)) == 0 {
		return GetInitShardState(), nil
	}
	prevEpoch := new(big.Int).Sub(epoch, common.Big1)
	ss, err := GetShardingStateFromBlockChain(bc, prevEpoch)
	if err != nil {
		return nil, ctxerror.New("cannot retrieve previous sharding state").
			WithCause(err)
	}
	newNodeList := ss.UpdateShardingState(stakeInfo)
	utils.Logger().Info().Float64("percentage", CuckooRate).Msg("Cuckoo Rate")
	ss.Reshard(newNodeList, CuckooRate)
	return ss.shardState, nil
}

// UpdateShardingState remove the unstaked nodes and returns the newly staked node Ids.
func (ss *ShardingState) UpdateShardingState(stakeInfo *map[common.Address]*structs.StakeInfo) []types.NodeID {
	oldBlsPublicKeys := make(map[types.BlsPublicKey]bool) // map of bls public keys
	for _, shard := range ss.shardState {
		newNodeList := shard.NodeList
		for _, nodeID := range shard.NodeList {
			oldBlsPublicKeys[nodeID.BlsPublicKey] = true
			_, ok := (*stakeInfo)[nodeID.EcdsaAddress]
			if ok {
				// newNodeList = append(newNodeList, nodeID)
			} else {
				// TODO: Remove the node if it's no longer staked
			}
		}
		shard.NodeList = newNodeList
	}

	newAddresses := []types.NodeID{}
	for addr, info := range *stakeInfo {
		_, ok := oldBlsPublicKeys[info.BlsPublicKey]
		if !ok {
			newAddresses = append(newAddresses, types.NodeID{addr, info.BlsPublicKey})
		}
	}
	return newAddresses
}

// TODO ek – shardingSchedule should really be part of a general-purpose network
//  configuration.  We are OK for the time being,
//  until the day we should let one node process join multiple networks.

// ShardingSchedule is the sharding configuration schedule.
// Depends on the type of the network.  Defaults to the mainnet schedule.
var ShardingSchedule shardingconfig.Schedule = shardingconfig.MainnetSchedule

// GetInitShardState returns the initial shard state at genesis.
func GetInitShardState() types.ShardState {
	utils.Logger().Info().Msg("Generating Genesis Shard State.")
	initShardingConfig := ShardingSchedule.InstanceForEpoch(
		big.NewInt(GenesisEpoch))
	genesisShardNum := int(initShardingConfig.NumShards())
	genesisShardHarmonyNodes := initShardingConfig.NumHarmonyOperatedNodesPerShard()
	genesisShardSize := initShardingConfig.NumNodesPerShard()
	shardState := types.ShardState{}
	for i := 0; i < genesisShardNum; i++ {
		com := types.Committee{ShardID: uint32(i)}
		for j := 0; j < genesisShardHarmonyNodes; j++ {
			index := i + j*genesisShardNum // The initial account to use for genesis nodes

			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(genesis.HarmonyAccounts[index].BlsPublicKey)
			pubKey := types.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := types.NodeID{common2.ParseAddr(genesis.HarmonyAccounts[index].Address), pubKey}
			com.NodeList = append(com.NodeList, curNodeID)
		}

		// add FN runner's key
		for j := genesisShardHarmonyNodes; j < genesisShardSize; j++ {
			index := i + (j-genesisShardHarmonyNodes)*genesisShardNum

			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(genesis.FoundationalNodeAccounts[index].BlsPublicKey)

			pubKey := types.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := types.NodeID{common2.ParseAddr(genesis.FoundationalNodeAccounts[index].Address), pubKey}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		shardState = append(shardState, com)
	}
	return shardState
}
