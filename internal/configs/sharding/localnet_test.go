package shardingconfig

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/internal/params"
)

func TestLocalnetEpochCalculation(t *testing.T) {
	InitLocalnetConfig(16, 16)

	//test config init
	lnConfig := GetLocalnetConfig()
	localnetBlocksPerEpochV2 := lnConfig.BlocksPerEpochV2

	//test epoch calculations
	check := func(epoch, expected uint64) {
		if got := LocalnetSchedule.EpochLastBlock(epoch); got != expected {
			t.Fatalf("wrong EpochLastBlock at epoch %d. TwoSecondsEpoch: %s. expected: %d got: %d.", epoch, params.LocalnetChainConfig.TwoSecondsEpoch.String(), expected, got)
		}
		if !LocalnetSchedule.IsLastBlock(expected) {
			t.Fatalf("%d is not LastBlock. TwoSecondsEpoch: %s", expected, params.LocalnetChainConfig.TwoSecondsEpoch.String())
		}
		epochStart := uint64(0)
		if epoch > 0 {
			epochStart = LocalnetSchedule.EpochLastBlock(epoch-1) + 1
		}
		for blockNo := epochStart; blockNo <= expected; blockNo++ {
			if isLastBlock := LocalnetSchedule.IsLastBlock(blockNo); isLastBlock != (blockNo == expected) {
				t.Fatalf("IsLastBlock for %d is wrong. TwoSecondsEpoch: %s. expected %v got %v", blockNo, params.LocalnetChainConfig.TwoSecondsEpoch.String(), blockNo == expected, isLastBlock)
			}
			got := LocalnetSchedule.CalcEpochNumber(blockNo).Uint64()
			if got != epoch {
				t.Fatalf("CalcEpochNumber for %d is wrong. TwoSecondsEpoch: %s. expected %d got %d", blockNo, params.LocalnetChainConfig.TwoSecondsEpoch.String(), epoch, got)
			}
		}
	}
	backup := params.LocalnetChainConfig.TwoSecondsEpoch
	params.LocalnetChainConfig.TwoSecondsEpoch = big.NewInt(0)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBootstrapEpochBlocks-1)
	check(2, localnetEpochBlock1+localnetBootstrapEpochBlocks*2-1)
	check(3, localnetEpochBlock1+localnetBootstrapEpochBlocks*3-1)
	check(4, localnetEpochBlock1+localnetBootstrapEpochBlocks*3+localnetBlocksPerEpochV2-1)

	params.LocalnetChainConfig.TwoSecondsEpoch = big.NewInt(1)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBootstrapEpochBlocks-1)
	check(2, localnetEpochBlock1+localnetBootstrapEpochBlocks*2-1)
	check(3, localnetEpochBlock1+localnetBootstrapEpochBlocks*3-1)
	check(4, localnetEpochBlock1+localnetBootstrapEpochBlocks*3+localnetBlocksPerEpochV2-1)

	params.LocalnetChainConfig.TwoSecondsEpoch = big.NewInt(2)
	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBootstrapEpochBlocks-1)
	check(2, localnetEpochBlock1+localnetBootstrapEpochBlocks*2-1)
	check(3, localnetEpochBlock1+localnetBootstrapEpochBlocks*3-1)
	check(4, localnetEpochBlock1+localnetBootstrapEpochBlocks*3+localnetBlocksPerEpochV2-1)

	params.LocalnetChainConfig.TwoSecondsEpoch = backup
}

func TestLocalnetBootstrapEpochCalculation(t *testing.T) {
	InitLocalnetConfig(16, 16)

	backupTwoSecondsEpoch := params.LocalnetChainConfig.TwoSecondsEpoch
	params.LocalnetChainConfig.TwoSecondsEpoch = big.NewInt(2)
	defer func() { params.LocalnetChainConfig.TwoSecondsEpoch = backupTwoSecondsEpoch }()

	check := func(epoch, expected uint64) {
		if got := LocalnetSchedule.EpochLastBlock(epoch); got != expected {
			t.Fatalf("wrong EpochLastBlock at epoch %d. expected: %d got: %d.", epoch, expected, got)
		}
		if !LocalnetSchedule.IsLastBlock(expected) {
			t.Fatalf("%d is not LastBlock", expected)
		}
		epochStart := uint64(0)
		if epoch > 0 {
			epochStart = LocalnetSchedule.EpochLastBlock(epoch-1) + 1
		}
		for blockNo := epochStart; blockNo <= expected; blockNo++ {
			if isLastBlock := LocalnetSchedule.IsLastBlock(blockNo); isLastBlock != (blockNo == expected) {
				t.Fatalf("IsLastBlock for %d is wrong. expected %v got %v", blockNo, blockNo == expected, isLastBlock)
			}
			if got := LocalnetSchedule.CalcEpochNumber(blockNo).Uint64(); got != epoch {
				t.Fatalf("CalcEpochNumber for %d is wrong. expected %d got %d", blockNo, epoch, got)
			}
		}
	}

	check(0, localnetEpochBlock1-1)
	check(1, localnetEpochBlock1+localnetBootstrapEpochBlocks-1)
	check(2, localnetEpochBlock1+localnetBootstrapEpochBlocks*2-1)
	check(3, localnetEpochBlock1+localnetBootstrapEpochBlocks*3-1)
	check(4, localnetEpochBlock1+localnetBootstrapEpochBlocks*3+16-1)
}
