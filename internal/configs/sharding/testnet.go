package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
)

// TestnetSchedule is the long-running public testnet sharding
// configuration schedule.
var TestnetSchedule testnetSchedule

type testnetSchedule struct{}

const (
	// ~304 sec epochs for P2 of open staking
	testnetBlocksPerEpoch = 38

	testnetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// TestNetHTTPPattern is the http pattern for testnet.
	TestNetHTTPPattern = "https://api.s%d.tn.hmny.io"
	// TestNetWSPattern is the websocket pattern for testnet.
	TestNetWSPattern = "wss://ws.s%d.tn.hmny.io"
)

func (testnetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.TestnetChainConfig.StakingEpoch) >= 0:
		return testnetV1
	default: // genesis
		return testnetV0
	}
}

func (testnetSchedule) BlocksPerEpoch() uint64 {
	return testnetBlocksPerEpoch
}

func (ts testnetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ts.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ts testnetSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ts.BlocksPerEpoch() == 0
}

func (ts testnetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ts.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ts testnetSchedule) VdfDifficulty() int {
	return testnetVdfDifficulty
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

func (ts testnetSchedule) GetNetworkID() NetworkID {
	return TestNet
}

// GetShardingStructure is the sharding structure for testnet.
func (ts testnetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, TestNetHTTPPattern, TestNetWSPattern)
}

var testnetReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.TestnetChainConfig.StakingEpoch,
}

var testnetV0 = MustNewInstance(4, 30, 25, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
var testnetV1 = MustNewInstance(4, 50, 25, numeric.MustNewDecFromStr("0.68"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, testnetReshardingEpoch, TestnetSchedule.BlocksPerEpoch())
