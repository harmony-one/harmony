package shardingconfig

import (
	"fmt"
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// LocalnetSchedule is the local testnet sharding
// configuration schedule.
var LocalnetSchedule localnetSchedule

type localnetSchedule struct{}

const (
	localnetV1Epoch = 1
	localnetV2Epoch = 2

	localnetEpochBlock1 = 10
	twoOne              = 5

	localnetVdfDifficulty  = 5000 // This takes about 10s to finish the vdf
	localnetConsensusRatio = float64(0.1)

	localnetRandomnessStartingEpoch = 0
)

func (localnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(localnetV2Epoch)) >= 0:
		return localnetV2
	case epoch.Cmp(big.NewInt(localnetV1Epoch)) >= 0:
		return localnetV1
	default: // genesis
		return localnetV0
	}
}

func (ls localnetSchedule) BlocksPerEpoch() uint64 {
	return twoOne
}

func (ls localnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ls.BlocksPerEpoch()
	switch {
	case blockNum >= localnetEpochBlock1:
		return big.NewInt(int64((blockNum-localnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ls localnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ls.BlocksPerEpoch()
	switch {
	case blockNum < localnetEpochBlock1-1:
		return false
	case blockNum == localnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-localnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ls localnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ls.BlocksPerEpoch()
	switch {
	case epochNum == 0:
		return localnetEpochBlock1 - 1
	default:
		return localnetEpochBlock1 - 1 + blocks*epochNum
	}
}

func (ls localnetSchedule) VdfDifficulty() int {
	return localnetVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ls localnetSchedule) ConsensusRatio() float64 {
	return localnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ls localnetSchedule) RandomnessStartingEpoch() uint64 {
	return localnetRandomnessStartingEpoch
}

func (ls localnetSchedule) GetNetworkID() NetworkID {
	return LocalNet
}

// GetShardingStructure is the sharding structure for localnet.
func (ls localnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	res := []map[string]interface{}{}
	for i := 0; i < numShard; i++ {
		res = append(res, map[string]interface{}{
			"current": int(shardID) == i,
			"shardID": i,
			"http":    fmt.Sprintf("http://127.0.0.1:%d", 9500+i),
			"ws":      fmt.Sprintf("ws://127.0.0.1:%d", 9800+i),
		})
	}
	return res
}

var (
	localnetReshardingEpoch = []*big.Int{
		big.NewInt(0), big.NewInt(localnetV1Epoch), big.NewInt(localnetV2Epoch),
	}
	// Number of shards, how many slots on each , how many slots owned by Harmony
	localnetV0 = MustNewInstance(2, 7, 5, genesis.LocalHarmonyAccounts, genesis.LocalFnAccounts, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
	localnetV1 = MustNewInstance(2, 8, 5, genesis.LocalHarmonyAccountsV1, genesis.LocalFnAccountsV1, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
	localnetV2 = MustNewInstance(2, 9, 6, genesis.LocalHarmonyAccountsV2, genesis.LocalFnAccountsV2, localnetReshardingEpoch, LocalnetSchedule.BlocksPerEpoch())
)
