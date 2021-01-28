package reward

import (
	"math/big"
	"testing"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/numeric"
)

func TestFiveSecondsBaseStakedReward(t *testing.T) {
	expectedNewReward := StakedBlocks.Mul(numeric.MustNewDecFromStr("5")).Quo(numeric.MustNewDecFromStr("8"))

	if !expectedNewReward.Equal(FiveSecStakedBlocks) {
		t.Errorf(
			"Expected: %s, Got: %s", FiveSecStakedBlocks.String(), expectedNewReward.String(),
		)
	}

	expectedNewReward = StakedBlocks.Mul(numeric.MustNewDecFromStr("2")).Quo(numeric.MustNewDecFromStr("8"))
	if !expectedNewReward.Equal(TwoSecStakedBlocks) {
		t.Errorf(
			"Expected: %s, Got: %s", TwoSecStakedBlocks.String(), expectedNewReward.String(),
		)
	}
}

func TestGetPreStakingRewardsFromBlockNumber(t *testing.T) {
	refMainnetRewards, _ := new(big.Int).SetString("319237464000000000000000000", 10)
	mainnetRewards := getTotalPreStakingNetworkRewards(shardingconfig.MainNet)
	if refMainnetRewards.Cmp(mainnetRewards) != 0 {
		t.Errorf("Expected mainnet rewards to be %v NOT %v", refMainnetRewards, mainnetRewards)
	}

	refTestnetRewards, _ := new(big.Int).SetString("7104000000000000000000", 10)
	testnetRewards := getTotalPreStakingNetworkRewards(shardingconfig.TestNet)
	if refTestnetRewards.Cmp(testnetRewards) != 0 {
		t.Errorf("Expected testnet rewards to be %v NOT %v", refTestnetRewards, testnetRewards)
	}
}
