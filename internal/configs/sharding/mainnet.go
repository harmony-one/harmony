package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	mainnetEpochBlock1 = 344064 // 21 * 2^14
	blocksPerShard     = 16384  // 2^14
	mainnetV1Epoch     = 1
	mainnetV2Epoch     = 5

	mainnetVdfDifficulty = 50000 // This takes about 100s to finish the vdf
)

// MainnetSchedule is the mainnet sharding configuration schedule.
var MainnetSchedule mainnetSchedule

type mainnetSchedule struct{}

func (mainnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(mainnetV2Epoch)) >= 0:
		// fifth resharding epoch around 08/06/2019 2:30am PDT
		return mainnetV2
	case epoch.Cmp(big.NewInt(mainnetV1Epoch)) >= 0:
		// first resharding epoch around 07/30/2019 10:30pm PDT
		return mainnetV1
	default: // genesis
		return mainnetV0
	}
}

func (mainnetSchedule) BlocksPerEpoch() uint64 {
	return blocksPerShard
}

func (ms mainnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ms.BlocksPerEpoch()
	switch {
	case blockNum >= mainnetEpochBlock1:
		return big.NewInt(int64((blockNum-mainnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ms mainnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ms.BlocksPerEpoch()
	switch {
	case blockNum < mainnetEpochBlock1-1:
		return false
	case blockNum == mainnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-mainnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ms mainnetSchedule) VdfDifficulty() int {
	return mainnetVdfDifficulty
}

var mainnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(mainnetV1Epoch)}
var mainnetV0 = MustNewInstance(4, 150, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, mainnetReshardingEpoch)
var mainnetV1 = MustNewInstance(4, 152, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1, mainnetReshardingEpoch)
var mainnetV2 = MustNewInstance(4, 160, 112, genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV2, mainnetReshardingEpoch)

//var mainnetV2 = MustNewInstance(8, 200, 100)
//var mainnet6400 = MustNewInstance(16, 400, 50)
