package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

type testnetSchedule struct{}

const (
	testnetV1Epoch = 1
	testnetV2Epoch = 2

	testnetEpochBlock1 = 78
	threeOne           = 111

	testnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	testnetFirstCrossLinkBlock = 100
)

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(big.NewInt(testnetV2Epoch)) >= 0:
		return testnetV2
	case epoch.Cmp(big.NewInt(testnetV1Epoch)) >= 0:
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	// 8 seconds per block, roughly 86400 blocks, around one day
	return threeOne
}

func (ts testnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	blocks := ts.BlocksPerEpoch()
	switch {
	case blockNum >= testnetEpochBlock1:
		return big.NewInt(int64((blockNum-testnetEpochBlock1)/blocks) + 1)
	default:
		return big.NewInt(0)
	}
}

func (ts testnetSchedule) IsLastBlock(blockNum uint64) bool {
	blocks := ts.BlocksPerEpoch()
	switch {
	case blockNum < testnetEpochBlock1-1:
		return false
	case blockNum == testnetEpochBlock1-1:
		return true
	default:
		return ((blockNum-testnetEpochBlock1)%blocks == blocks-1)
	}
}

func (ts testnetSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
}

func (ts testnetSchedule) FirstCrossLinkBlock() uint64 {
	return testnetFirstCrossLinkBlock
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ts testnetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ts testnetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

var testnetReshardingEpoch = []*big.Int{big.NewInt(0), big.NewInt(testnetV1Epoch), big.NewInt(testnetV2Epoch)}

var testnetV0 = MustNewInstance(2, 150, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV1 = MustNewInstance(2, 160, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
var testnetV2 = MustNewInstance(2, 170, 150, genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch)
