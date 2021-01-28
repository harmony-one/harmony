package chain

import (
	"context"
	"math/big"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	stakingReward "github.com/harmony-one/harmony/staking/reward"
)

// GetCirculatingSupply using the following formula:
// (initialSupply * percentReleased) + injectedNetworkRewards
//
// WARNING: only works on beaconchain if in staking era. If not in staking era, then
// pre-staking era 'injectedNetworkRewards' are NOT considered (for precision reasons).
func GetCirculatingSupply(
	ctx context.Context, chain engine.ChainReader,
) (numeric.Dec, error) {
	totalSupply, err := stakingReward.GetTotalTokens(chain)
	if err != nil {
		return numeric.Dec{}, err
	}

	initSupply, timestamp := big.NewInt(0), time.Now()
	currHeader := chain.CurrentHeader()
	numShards := shard.Schedule.InstanceForEpoch(currHeader.Epoch()).NumShards()
	for i := uint32(0); i < numShards; i++ {
		initSupply = new(big.Int).Add(core.GetInitialFunds(i), initSupply)
	}

	releasedInitSupply := numeric.NewDecFromBigInt(initSupply).Mul(
		reward.PercentageForTimeStamp(timestamp.Unix()),
	)
	injectedNetworkRewards := totalSupply.Sub(numeric.NewDecFromBigInt(initSupply))
	if !chain.Config().IsStaking(currHeader.Epoch()) {
		injectedNetworkRewards = numeric.NewDec(0)
	}
	return releasedInitSupply.Add(injectedNetworkRewards), nil
}
