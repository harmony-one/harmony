package chain

import (
	"context"
	"math/big"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
)

// GetCirculatingSupply using the following formula:
// (TotalPreStakingTokens * percentReleased) + StakingBlockRewards
//
// WARNING: only works on beaconchain if in staking era.
func GetCirculatingSupply(
	ctx context.Context, chain engine.ChainReader,
) (ret numeric.Dec, err error) {
	currHeader, timestamp := chain.CurrentHeader(), time.Now().Unix()
	stakingBlockRewards := big.NewInt(0)

	if chain.Config().IsStaking(currHeader.Epoch()) {
		if chain.ShardID() != shard.BeaconChainShardID {
			return numeric.Dec{}, stakingReward.ErrInvalidBeaconChain
		}
		if stakingBlockRewards, err = chain.ReadBlockRewardAccumulator(currHeader.Number().Uint64()); err != nil {
			return numeric.Dec{}, err
		}
	}

	releasedPreStakingSupply := stakingReward.TotalPreStakingTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	)
	return releasedPreStakingSupply.Add(numeric.Dec{Int: stakingBlockRewards}), nil
}
