package shardingconfig

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/internal/params"
)

func TestLocalnetEpochCalculation(t *testing.T) {
	check := func(epoch, expected uint64) {
		if got := LocalnetSchedule.EpochLastBlock(epoch); got != expected {
			t.Fatalf("wrong EpochLastBlock at epoch %d. oneSecondsEpoch: %s. expected: %d got: %d.", epoch, params.LocalnetChainConfig.oneSecondsEpoch.String(), expected, got)
		}
		if !LocalnetSchedule.IsLastBlock(expected) {
			t.Fatalf("%d is not LastBlock. oneSecondsEpoch: %s", expected, params.LocalnetChainConfig.oneSecondsEpoch.String())
		}
		epochStart := uint64(0)
		if epoch > 0 {
			epochStart = LocalnetSchedule.EpochLastBlock(epoch-1) + 1
		}
		for blockNo := epochStart; blockNo <= expected; blockNo++ {
			if isLastBlock := LocalnetSchedule.IsLastBlock(blockNo); isLastBlock != (blockNo == expected) {
				t.Fatalf("IsLastBlock for %d is wrong. oneSecondsEpoch: %s. expected %v got %v", blockNo, params.LocalnetChainConfig.oneSecondsEpoch.String(), blockNo == expected, isLastBlock)
			}
			got := LocalnetSchedule.CalcEpochNumber(blockNo).Uint64()
			if got != epoch {
				t.Fatalf("CalcEpochNumber for %d is wrong. oneSecondsEpoch: %s. expected %d got %d", blockNo, params.LocalnetChainConfig.oneSecondsEpoch.String(), epoch, got)
			}
		}
	}
	backup := params.LocalnetChainConfig.oneSecondsEpoch
	params.LocalnetChainConfig.oneSecondsEpoch = big.NewInt(0)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBlocksPerEpochV2-1)
	check(2, localnetEpochBlock1+localnetBlocksPerEpochV2*2-1)

	params.LocalnetChainConfig.oneSecondsEpoch = big.NewInt(1)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBlocksPerEpochV2-1)
	check(2, localnetEpochBlock1+localnetBlocksPerEpochV2*2-1)

	params.LocalnetChainConfig.oneSecondsEpoch = big.NewInt(2)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBlocksPerEpoch-1)
	check(2, localnetEpochBlock1+localnetBlocksPerEpoch+localnetBlocksPerEpochV2-1)
	check(3, localnetEpochBlock1+localnetBlocksPerEpoch+localnetBlocksPerEpochV2*2-1)

	params.LocalnetChainConfig.oneSecondsEpoch = backup
}
