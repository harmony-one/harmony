package core

import (
	"encoding/hex"
	"errors"
	"math/big"
	"math/rand"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

const (
	// GenesisEpoch is the number of the genesis epoch.
	GenesisEpoch = 0
	// CuckooRate is the percentage of nodes getting reshuffled in the second step of cuckoo resharding.
	CuckooRate = 0.1
)

// ShardingState is data structure hold the sharding state
type ShardingState struct {
	epoch      uint64 // current epoch
	rnd        uint64 // random seed for resharding
	numShards  int    // TODO ek – equal to len(shardState); remove this
	shardState shard.State
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
func (ss *ShardingState) assignNewNodes(newNodeList []shard.NodeID) {
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
	kickedNodes := []shard.NodeID{}
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
func (ss *ShardingState) Reshard(newNodeList []shard.NodeID, percent float64) {
	rand.Seed(int64(ss.rnd))
	ss.sortCommitteeBySize()

	// Take out and preserve leaders
	leaders := []shard.NodeID{}
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
		ss.shardState[i].NodeList = append([]shard.NodeID{leaders[i]}, ss.shardState[i].NodeList...)
	}
}

// Shuffle will shuffle the list with result uniquely determined by seed, assuming there is no repeat items in the list
func Shuffle(list []shard.NodeID) {
	// Sort to make sure everyone will generate the same with the same rand seed.
	sort.Slice(list, func(i, j int) bool {
		return shard.CompareNodeIDByBLSKey(list[i], list[j]) == -1
	})
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})
}

// GetEpochFromBlockNumber calculates the epoch number the block belongs to
func GetEpochFromBlockNumber(blockNumber uint64) uint64 {
	return ShardingSchedule.CalcEpochNumber(blockNumber).Uint64()
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

	// TODO(RJ,HB): use real randomness for resharding
	//blockNumber := GetBlockNumberFromEpoch(epoch.Uint64())
	//rndSeedBytes := bc.GetVdfByNumber(blockNumber)
	rndSeed := uint64(0)

	return &ShardingState{epoch: epoch.Uint64(), rnd: rndSeed, shardState: shardState, numShards: len(shardState)}, nil
}

// CalculateNewShardState get sharding state from previous epoch and calculate sharding state for new epoch
func CalculateNewShardState(bc *BlockChain, epoch *big.Int) (shard.State, error) {
	if epoch.Cmp(big.NewInt(GenesisEpoch)) == 0 {
		return CalculateInitShardState(), nil
	}
	prevEpoch := new(big.Int).Sub(epoch, common.Big1)
	ss, err := GetShardingStateFromBlockChain(bc, prevEpoch)
	if err != nil {
		return nil, ctxerror.New("cannot retrieve previous sharding state").
			WithCause(err)
	}
	utils.Logger().Info().Float64("percentage", CuckooRate).Msg("Cuckoo Rate")
	return ss.shardState, nil
}

// TODO ek – shardingSchedule should really be part of a general-purpose network
//  configuration.  We are OK for the time being,
//  until the day we should let one node process join multiple networks.

// ShardingSchedule is the sharding configuration schedule.
// Depends on the type of the network.  Defaults to the mainnet schedule.
var ShardingSchedule shardingconfig.Schedule = shardingconfig.MainnetSchedule

// CalculateInitShardState returns the initial shard state at genesis.
func CalculateInitShardState() shard.State {
	return CalculateShardState(big.NewInt(GenesisEpoch))
}

// CalculateShardState returns the shard state based on epoch number
// This api for getting shard state is what should be used to get shard state regardless of
// current chain dependency (ex. getting shard state from block header received during cross-shard transaction)
func CalculateShardState(epoch *big.Int) shard.State {
	utils.Logger().Info().Int64("epoch", epoch.Int64()).Msg("Get Shard State of Epoch.")
	shardingConfig := ShardingSchedule.InstanceForEpoch(epoch)
	shardNum := int(shardingConfig.NumShards())
	shardHarmonyNodes := shardingConfig.NumHarmonyOperatedNodesPerShard()
	shardSize := shardingConfig.NumNodesPerShard()
	hmyAccounts := shardingConfig.HmyAccounts()
	fnAccounts := shardingConfig.FnAccounts()

	shardState := shard.State{}
	for i := 0; i < shardNum; i++ {
		com := shard.Committee{ShardID: uint32(i)}
		for j := 0; j < shardHarmonyNodes; j++ {
			index := i + j*shardNum // The initial account to use for genesis nodes

			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(hmyAccounts[index].BlsPublicKey)
			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				EcdsaAddress: common2.ParseAddr(hmyAccounts[index].Address),
				BlsPublicKey: pubKey,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}

		// add FN runner's key
		for j := shardHarmonyNodes; j < shardSize; j++ {
			index := i + (j-shardHarmonyNodes)*shardNum

			pub := &bls.PublicKey{}
			pub.DeserializeHexStr(fnAccounts[index].BlsPublicKey)

			pubKey := shard.BlsPublicKey{}
			pubKey.FromLibBLSPublicKey(pub)
			// TODO: directly read address for bls too
			curNodeID := shard.NodeID{
				EcdsaAddress: common2.ParseAddr(fnAccounts[index].Address),
				BlsPublicKey: pubKey,
			}
			com.NodeList = append(com.NodeList, curNodeID)
		}
		shardState = append(shardState, com)
	}
	return shardState
}

// CalculatePublicKeys returns the publickeys given epoch and shardID
func CalculatePublicKeys(epoch *big.Int, shardID uint32) []*bls.PublicKey {
	shardState := CalculateShardState(epoch)

	// Update validator public keys
	committee := shardState.FindCommitteeByID(shardID)
	if committee == nil {
		utils.Logger().Warn().Uint32("shardID", shardID).Uint64("epoch", epoch.Uint64()).Msg("Cannot find committee")
		return nil
	}
	pubKeys := []*bls.PublicKey{}
	for _, node := range committee.NodeList {
		pubKey := &bls.PublicKey{}
		pubKeyBytes := node.BlsPublicKey[:]
		err := pubKey.Deserialize(pubKeyBytes)
		if err != nil {
			utils.Logger().Warn().Str("pubKeyBytes", hex.EncodeToString(pubKeyBytes)).Msg("Cannot Deserialize pubKey")
			return nil
		}
		pubKeys = append(pubKeys, pubKey)
	}
	return pubKeys
}
