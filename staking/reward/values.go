package reward

import (
	"math/big"
	"sync"

	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
)

var (
	// PreStakedBlocks is the block reward, to be split evenly among block signers in pre-staking era.
	// 24 ONE per block
	PreStakedBlocks = new(big.Int).Mul(big.NewInt(24), big.NewInt(denominations.One))
	// StakedBlocks is the flat-rate block reward for epos staking launch.
	// 28 ONE per block.
	StakedBlocks = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(28), big.NewInt(denominations.One),
	))
	// FiveSecStakedBlocks is the flat-rate block reward after epoch 230.
	// 17.5 ONE per block
	FiveSecStakedBlocks = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(17.5*denominations.Nano), big.NewInt(denominations.Nano),
	))
	// TwoSecStakedBlocks is the flat-rate block reward after epoch 360.
	// 7 ONE per block
	TwoSecStakedBlocks = numeric.NewDecFromBigInt(new(big.Int).Mul(
		big.NewInt(7*denominations.Nano), big.NewInt(denominations.Nano),
	))
	// TotalPreStakingTokens is the total amount of tokens in the network at the the last block of the
	// pre-staking era (epoch < staking epoch). This should be set/change on the node's init according
	// to the core.GenesisSpec.
	TotalPreStakingTokens = numeric.NewDecFromBigInt(
		new(big.Int).Mul(big.NewInt(12600000000), big.NewInt(denominations.One)),
	)
	// None ..
	None = big.NewInt(0)

	// testnetLastPreStakingBlock according to beacon-chain logic.
	// May result in inaccuracies for testnet calc involving total tokens.
	testnetLastPreStakingBlock = new(big.Int).SetUint64(shardingconfig.TestnetSchedule.EpochLastBlock(
		params.TestnetChainConfig.StakingEpoch.Uint64() - 1,
	))

	once sync.Once
)

// WARNING: the data collected here are calculated from a consumer of the Rosetta API.
// If data becomes mission critical, implement a cross-link based approach.
//
// Data Source: https://github.com/harmony-one/jupyter
//
// TODO (dm): use first crosslink of all shards to compute rewards on network instead of relying on constants.
var (
	totalPreStakingNetworkRewards = map[shardingconfig.NetworkID][]*big.Int{
		shardingconfig.MainNet: {
			// Below are all of the last blocks of pre-staking era for mainnet.
			getPreStakingRewardsFromBlockNumber(shardingconfig.MainNet, big.NewInt(3375103)),
			getPreStakingRewardsFromBlockNumber(shardingconfig.MainNet, big.NewInt(3286737)),
			getPreStakingRewardsFromBlockNumber(shardingconfig.MainNet, big.NewInt(3326153)),
			getPreStakingRewardsFromBlockNumber(shardingconfig.MainNet, big.NewInt(3313572)),
		},
		shardingconfig.TestNet: {
			getPreStakingRewardsFromBlockNumber(shardingconfig.TestNet, testnetLastPreStakingBlock),
			getPreStakingRewardsFromBlockNumber(shardingconfig.TestNet, testnetLastPreStakingBlock),
			getPreStakingRewardsFromBlockNumber(shardingconfig.TestNet, testnetLastPreStakingBlock),
			getPreStakingRewardsFromBlockNumber(shardingconfig.TestNet, testnetLastPreStakingBlock),
		},
	}
)

// getPreStakingRewardsFromBlockNumber returns the number of tokens injected into the network
// in the pre-staking era (epoch < staking epoch).
func getPreStakingRewardsFromBlockNumber(id shardingconfig.NetworkID, blockNum *big.Int) *big.Int {
	lastBlockInEpoch := blockNum

	switch id {
	case shardingconfig.MainNet:
		lastBlockInEpoch = new(big.Int).SetUint64(shardingconfig.MainnetSchedule.EpochLastBlock(
			params.MainnetChainConfig.StakingEpoch.Uint64() - 1,
		))
	case shardingconfig.TestNet:
		lastBlockInEpoch = new(big.Int).SetUint64(shardingconfig.TestnetSchedule.EpochLastBlock(
			params.TestChainConfig.StakingEpoch.Uint64() - 1,
		))
	}

	if blockNum.Cmp(lastBlockInEpoch) == 1 {
		blockNum = lastBlockInEpoch
	}
	return new(big.Int).Mul(PreStakedBlocks, blockNum)
}

func getTotalPreStakingNetworkRewards(id shardingconfig.NetworkID) *big.Int {
	totalRewards := big.NewInt(0)
	if allRewards, ok := totalPreStakingNetworkRewards[id]; ok {
		for _, reward := range allRewards {
			totalRewards = new(big.Int).Add(reward, totalRewards)
		}
	}
	return totalRewards
}

// GetTotalTokens in the network. If it is not staking era or it is not beacon chain, best
// value that can be returned in the total tokens prior to the staking epoch, i.e: TotalPreStakingTokens.
func GetTotalTokens(chain engine.ChainReader) numeric.Dec {
	if chain.ShardID() != shard.BeaconChainShardID || !chain.Config().IsStaking(chain.CurrentHeader().Epoch()) {
		return TotalPreStakingTokens
	}
	// TODO: implement reading accumulator and adding it to TotalPreStakingTokens...
	return numeric.Dec{}
}

// SetTotalPreStakingTokens with the given initial tokens (from genesis).
// Warning: May result in an inaccurate number if network is not in the staking era.
func SetTotalPreStakingTokens(initTokens *big.Int) {
	once.Do(func() {
		totalTokens := new(big.Int).Add(initTokens, getTotalPreStakingNetworkRewards(shard.Schedule.GetNetworkID()))
		TotalPreStakingTokens = numeric.NewDecFromBigInt(totalTokens)
	})
}
