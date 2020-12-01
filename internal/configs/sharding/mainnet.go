package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	mainnetEpochBlock1 = 344064 // 21 * 2^14
	blocksPerEpoch     = 16384  // 2^14

	mainnetVdfDifficulty  = 50000 // This takes about 100s to finish the vdf
	mainnetConsensusRatio = float64(0.1)

	// TODO: remove it after randomness feature turned on mainnet
	mainnetRandomnessStartingEpoch = 100000

	mainnetV0_1Epoch = 1
	mainnetV0_2Epoch = 5
	mainnetV0_3Epoch = 8
	mainnetV0_4Epoch = 10
	mainnetV1Epoch   = 12
	mainnetV1_1Epoch = 19
	mainnetV1_2Epoch = 25
	mainnetV1_3Epoch = 36
	mainnetV1_4Epoch = 46
	mainnetV1_5Epoch = 54
	mainnetV2_0Epoch = 185 // prestaking epoch
	mainnetV2_1Epoch = 208 // open slots increase from 320 - 480
	mainnetV2_2Epoch = 231 // open slots increase from 480 - 640

	// MainNetHTTPPattern is the http pattern for mainnet.
	MainNetHTTPPattern = "https://api.s%d.t.hmny.io"
	// MainNetWSPattern is the websocket pattern for mainnet.
	MainNetWSPattern = "wss://ws.s%d.t.hmny.io"
)

var (
	// map of epochs skipped due to staking launch on mainnet
	skippedEpochs = map[uint32][]*big.Int{
		1: []*big.Int{big.NewInt(181), big.NewInt(182), big.NewInt(183), big.NewInt(184), big.NewInt(185)},
		2: []*big.Int{big.NewInt(184), big.NewInt(185)},
		3: []*big.Int{big.NewInt(183), big.NewInt(184), big.NewInt(185)},
	}
)

// MainnetSchedule is the mainnet sharding configuration schedule.
var MainnetSchedule mainnetSchedule

type mainnetSchedule struct{}

func (ms mainnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(mainnetV2_2Epoch)) >= 0:
		return mainnetV2_2
	case epoch.Cmp(big.NewInt(mainnetV2_1Epoch)) >= 0:
		return mainnetV2_1
	case epoch.Cmp(big.NewInt(mainnetV2_0Epoch)) >= 0:
		// 185 resharding epoch (for shard 0) around 14/05/2020 ~15:00 PDT
		return mainnetV2_0
	case epoch.Cmp(big.NewInt(mainnetV1_5Epoch)) >= 0:
		// 54 resharding epoch (for shard 0) around 23/10/2019 ~10:05 PDT
		return mainnetV1_5
	case epoch.Cmp(big.NewInt(mainnetV1_4Epoch)) >= 0:
		// forty-sixth resharding epoch around 10/10/2019 8:06pm PDT
		return mainnetV1_4
	case epoch.Cmp(big.NewInt(mainnetV1_3Epoch)) >= 0:
		// thirty-sixth resharding epoch around 9/25/2019 5:44am PDT
		return mainnetV1_3
	case epoch.Cmp(big.NewInt(mainnetV1_2Epoch)) >= 0:
		// twenty-fifth resharding epoch around 09/06/2019 5:31am PDT
		return mainnetV1_2
	case epoch.Cmp(big.NewInt(mainnetV1_1Epoch)) >= 0:
		// nineteenth resharding epoch around 08/27/2019 9:07pm PDT
		return mainnetV1_1
	case epoch.Cmp(big.NewInt(mainnetV1Epoch)) >= 0:
		// twelfth resharding epoch around 08/16/2019 11:00pm PDT
		return mainnetV1
	case epoch.Cmp(big.NewInt(mainnetV0_4Epoch)) >= 0:
		// tenth resharding epoch around 08/13/2019 9:00pm PDT
		return mainnetV0_4
	case epoch.Cmp(big.NewInt(mainnetV0_3Epoch)) >= 0:
		// eighth resharding epoch around 08/10/2019 6:00pm PDT
		return mainnetV0_3
	case epoch.Cmp(big.NewInt(mainnetV0_2Epoch)) >= 0:
		// fifth resharding epoch around 08/06/2019 2:30am PDT
		return mainnetV0_2
	case epoch.Cmp(big.NewInt(mainnetV0_1Epoch)) >= 0:
		// first resharding epoch around 07/30/2019 10:30pm PDT
		return mainnetV0_1
	default: // genesis
		return mainnetV0
	}
}

func (ms mainnetSchedule) BlocksPerEpoch() uint64 {
	return blocksPerEpoch
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

func (ms mainnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	blocks := ms.BlocksPerEpoch()
	switch {
	case epochNum == 0:
		return mainnetEpochBlock1 - 1
	default:
		return mainnetEpochBlock1 - 1 + blocks*epochNum
	}
}

func (ms mainnetSchedule) VdfDifficulty() int {
	return mainnetVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ms mainnetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ms mainnetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ms mainnetSchedule) GetNetworkID() NetworkID {
	return MainNet
}

// GetShardingStructure is the sharding structure for mainnet.
func (ms mainnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, MainNetHTTPPattern, MainNetWSPattern)
}

// IsSkippedEpoch returns if an epoch was skipped on shard due to staking epoch
func (ms mainnetSchedule) IsSkippedEpoch(shardID uint32, epoch *big.Int) bool {
	if skipped, exists := skippedEpochs[shardID]; exists {
		for _, e := range skipped {
			if epoch.Cmp(e) == 0 {
				return true
			}
		}
	}
	return false
}

var mainnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(mainnetV0_1Epoch), big.NewInt(mainnetV0_2Epoch), big.NewInt(mainnetV0_3Epoch), big.NewInt(mainnetV0_4Epoch), big.NewInt(mainnetV1Epoch), big.NewInt(mainnetV1_1Epoch), big.NewInt(mainnetV1_2Epoch), big.NewInt(mainnetV1_3Epoch), big.NewInt(mainnetV1_4Epoch), big.NewInt(mainnetV1_5Epoch), big.NewInt(mainnetV2_0Epoch), big.NewInt(mainnetV2_1Epoch), big.NewInt(mainnetV2_2Epoch)}

var (
	mainnetV0   = MustNewInstance(4, 150, 112, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccounts, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV0_1 = MustNewInstance(4, 152, 112, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_1, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV0_2 = MustNewInstance(4, 200, 148, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_2, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV0_3 = MustNewInstance(4, 210, 148, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_3, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV0_4 = MustNewInstance(4, 216, 148, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV0_4, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1   = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1_1 = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_1, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1_2 = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_2, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1_3 = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_3, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1_4 = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_4, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV1_5 = MustNewInstance(4, 250, 170, numeric.OneDec(), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_5, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV2_0 = MustNewInstance(4, 250, 170, numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_5, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV2_1 = MustNewInstance(4, 250, 130, numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_5, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
	mainnetV2_2 = MustNewInstance(4, 250, 90, numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts, genesis.FoundationalNodeAccountsV1_5, mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(), MainnetSchedule.BlocksPerEpoch())
)
