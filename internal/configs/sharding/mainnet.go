package shardingconfig

import (
	"math/big"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
)

const (
	mainnetEpochBlock1 = 344064 // 21 * 2^14
	blocksPerEpoch     = 16384  // 2^14
	blocksPerEpochV2   = 32768  // 2^15

	mainnetVdfDifficulty = 50000 // This takes about 100s to finish the vdf

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

	feeCollectorsMainnet = FeeCollectors{
		// Infrastructure
		mustAddress("0xa0c395A83503ad89613E43397e9fE1f8E93B6384"): numeric.MustNewDecFromStr("0.5"),
		// Community
		mustAddress("0xbdFeE8587d347Cd8df002E6154763325265Fa84c"): numeric.MustNewDecFromStr("0.5"),
	}

	// Emission DAO
	hip30CollectionAddress = mustAddress("0xD8194284df879f465ed61DBA6fa8300940cacEA3")
)

func mustAddress(addrStr string) ethCommon.Address {
	addr, err := common.ParseAddr(addrStr)
	if err != nil {
		panic("invalid address")
	}
	return addr
}

// MainnetSchedule is the mainnet sharding configuration schedule.
var MainnetSchedule mainnetSchedule

type mainnetSchedule struct{}

func (ms mainnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case params.MainnetChainConfig.IsHIP30(epoch):
		return mainnetV4
	case params.MainnetChainConfig.IsFeeCollectEpoch(epoch):
		return mainnetV3_4
	case params.MainnetChainConfig.IsSlotsLimited(epoch):
		return mainnetV3_3
	case params.MainnetChainConfig.IsHIP6And8Epoch(epoch):
		// Decrease internal voting power from 60% to 49%
		// Increase external nodes from 800 to 900
		// which happens around 10/11/2021 22:00 PDT
		return mainnetV3_2
	case params.MainnetChainConfig.IsSixtyPercent(epoch):
		// Decrease internal voting power from 68% to 60%
		// which happens around 1/27/2021 22:00 PDT
		return mainnetV3_1
	case params.MainnetChainConfig.IsTwoSeconds(epoch):
		// Enable 2s block time and change blocks/epoch to 32768
		// which happens around 12/08/2020 08:00 PDT
		return mainnetV3
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

func (ms mainnetSchedule) BlocksPerEpochOld() uint64 {
	return blocksPerEpoch
}

func (ms mainnetSchedule) BlocksPerEpoch() uint64 {
	return blocksPerEpochV2
}

func (ms mainnetSchedule) twoSecondsFirstBlock() uint64 {
	if params.MainnetChainConfig.TwoSecondsEpoch.Uint64() == 0 {
		return 0
	}
	return (params.MainnetChainConfig.TwoSecondsEpoch.Uint64()-1)*ms.BlocksPerEpochOld() + mainnetEpochBlock1
}

func (ms mainnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	var oldEpochNumber int64
	switch {
	case blockNum >= mainnetEpochBlock1:
		oldEpochNumber = int64((blockNum-mainnetEpochBlock1)/ms.BlocksPerEpochOld()) + 1
	default:
		oldEpochNumber = 0
	}

	firstBlock2s := ms.twoSecondsFirstBlock()

	switch {
	case params.MainnetChainConfig.IsTwoSeconds(big.NewInt(oldEpochNumber)):
		return big.NewInt(int64((blockNum-firstBlock2s)/ms.BlocksPerEpoch() + params.MainnetChainConfig.TwoSecondsEpoch.Uint64()))
	default: // genesis
		return big.NewInt(int64(oldEpochNumber))
	}
}

func (ms mainnetSchedule) IsLastBlock(blockNum uint64) bool {
	switch {
	case blockNum < mainnetEpochBlock1-1:
		return false
	case blockNum == mainnetEpochBlock1-1:
		return true
	default:
		firstBlock2s := ms.twoSecondsFirstBlock()
		switch {
		case blockNum >= firstBlock2s:
			return ((blockNum-firstBlock2s)%ms.BlocksPerEpoch() == ms.BlocksPerEpoch()-1)
		default: // genesis
			return ((blockNum-mainnetEpochBlock1)%ms.BlocksPerEpochOld() == ms.BlocksPerEpochOld()-1)
		}
	}
}

func (ms mainnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	switch {
	case epochNum == 0:
		return mainnetEpochBlock1 - 1
	default:
		firstBlock2s := ms.twoSecondsFirstBlock()
		switch {
		case params.MainnetChainConfig.IsTwoSeconds(big.NewInt(int64(epochNum))):
			return firstBlock2s - 1 + ms.BlocksPerEpoch()*(epochNum-params.MainnetChainConfig.TwoSecondsEpoch.Uint64()+1)
		default: // genesis
			return mainnetEpochBlock1 - 1 + ms.BlocksPerEpochOld()*epochNum
		}
	}
}

func (ms mainnetSchedule) VdfDifficulty() int {
	return mainnetVdfDifficulty
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

var mainnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(mainnetV0_1Epoch), big.NewInt(mainnetV0_2Epoch), big.NewInt(mainnetV0_3Epoch), big.NewInt(mainnetV0_4Epoch), big.NewInt(mainnetV1Epoch), big.NewInt(mainnetV1_1Epoch), big.NewInt(mainnetV1_2Epoch), big.NewInt(mainnetV1_3Epoch), big.NewInt(mainnetV1_4Epoch), big.NewInt(mainnetV1_5Epoch), big.NewInt(mainnetV2_0Epoch), big.NewInt(mainnetV2_1Epoch), big.NewInt(mainnetV2_2Epoch), params.MainnetChainConfig.TwoSecondsEpoch, params.MainnetChainConfig.SixtyPercentEpoch, params.MainnetChainConfig.HIP6And8Epoch}

var (
	mainnetV0 = MustNewInstance(
		4, 150, 112, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccounts, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV0_1 = MustNewInstance(
		4, 152, 112, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV0_1, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV0_2 = MustNewInstance(
		4, 200, 148, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV0_2, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV0_3 = MustNewInstance(
		4, 210, 148, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV0_3, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV0_4 = MustNewInstance(
		4, 216, 148, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV0_4, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1_1 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_1, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1_2 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_2, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1_3 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_3, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1_4 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_4, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV1_5 = MustNewInstance(
		4, 250, 170, 0,
		numeric.OneDec(), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV2_0 = MustNewInstance(
		4, 250, 170, 0,
		numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV2_1 = MustNewInstance(
		4, 250, 130, 0,
		numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV2_2 = MustNewInstance(
		4, 250, 90, 0,
		numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpochOld(),
	)
	mainnetV3 = MustNewInstance(
		4, 250, 90, 0,
		numeric.MustNewDecFromStr("0.68"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(),
	)
	mainnetV3_1 = MustNewInstance(
		4, 250, 50, 0,
		numeric.MustNewDecFromStr("0.60"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(),
	)
	mainnetV3_2 = MustNewInstance(
		4, 250, 25, 0,
		numeric.MustNewDecFromStr("0.49"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(),
	)
	mainnetV3_3 = MustNewInstance(
		4, 250, 25, 0.06,
		numeric.MustNewDecFromStr("0.49"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist, nil,
		numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(),
	)
	mainnetV3_4 = MustNewInstance(
		4, 250, 25, 0.06,
		numeric.MustNewDecFromStr("0.49"), genesis.HarmonyAccounts,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist,
		feeCollectorsMainnet, numeric.ZeroDec(), ethCommon.Address{},
		mainnetReshardingEpoch, MainnetSchedule.BlocksPerEpoch(),
	)
	mainnetV4 = MustNewInstance(
		// internal slots are 10% of total slots
		2, 200, 20, 0.06,
		numeric.MustNewDecFromStr("0.49"),
		genesis.HarmonyAccountsPostHIP30,
		genesis.FoundationalNodeAccountsV1_5, emptyAllowlist,
		feeCollectorsMainnet, numeric.MustNewDecFromStr("0.25"),
		hip30CollectionAddress, mainnetReshardingEpoch,
		MainnetSchedule.BlocksPerEpoch(),
	)
)
