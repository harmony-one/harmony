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
// (TotalInitialTokens * percentReleased) + PreStakingBlockRewards + StakingBlockRewards
//
// Note that PreStakingBlockRewards is set to the amount of rewards given out by the
// LAST BLOCK of the pre-staking era regardless of what the current block height is
// if network is in the pre-staking era. This is for implementation reasons, reference
// stakingReward.GetTotalPreStakingTokens for more details.
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

	releasedInitSupply := stakingReward.TotalInitialTokens.Mul(
		reward.PercentageForTimeStamp(timestamp),
	)
	preStakingBlockRewards := stakingReward.GetTotalPreStakingTokens().Sub(stakingReward.TotalInitialTokens)
	return releasedInitSupply.Add(preStakingBlockRewards).Add(
		numeric.NewDecFromBigIntWithPrec(stakingBlockRewards, 18),
	), nil
}
