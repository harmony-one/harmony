package shardingconfig

import (
	"math/big"

	"github.com/harmony-one/harmony/numeric"

	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/params"
)

// P2PNetSchedule is a temporary testing network for p2p changes
// configuration schedule.
var P2PNetSchedule p2pNetSchedule

type p2pNetSchedule struct{}

const (
	// ~304 sec epochs for P2 of open staking
	p2pNetBlocksPerEpoch = 38

	p2pNetVdfDifficulty = 10000 // This takes about 20s to finish the vdf

	// P2PNetHTTPPattern is the http pattern for p2pNet.
	P2PNetHTTPPattern = "https://api.s%d.p2p.hmny.io"
	// P2PNetWSPattern is the websocket pattern for p2pNet.
	P2PNetWSPattern = "wss://ws.s%d.p2p.hmny.io"
)

func (p2pNetSchedule) InstanceForEpoch(epoch *big.Int) Instance {
	switch {
	case epoch.Cmp(params.P2PNetChainConfig.StakingEpoch) >= 0:
		return p2pNetV1
	default: // genesis
		return p2pNetV0
	}
}

func (p2pNetSchedule) BlocksPerEpoch() uint64 {
	return p2pNetBlocksPerEpoch
}

func (ss p2pNetSchedule) CalcEpochNumber(blockNum uint64) *big.Int {
	epoch := blockNum / ss.BlocksPerEpoch()
	return big.NewInt(int64(epoch))
}

func (ss p2pNetSchedule) IsLastBlock(blockNum uint64) bool {
	return (blockNum+1)%ss.BlocksPerEpoch() == 0
}

func (ss p2pNetSchedule) EpochLastBlock(epochNum uint64) uint64 {
	return ss.BlocksPerEpoch()*(epochNum+1) - 1
}

func (ss p2pNetSchedule) VdfDifficulty() int {
	return p2pNetVdfDifficulty
}

// ConsensusRatio ratio of new nodes vs consensus total nodes
func (ss p2pNetSchedule) ConsensusRatio() float64 {
	return mainnetConsensusRatio
}

// TODO: remove it after randomness feature turned on mainnet
//RandonnessStartingEpoch returns starting epoch of randonness generation
func (ss p2pNetSchedule) RandomnessStartingEpoch() uint64 {
	return mainnetRandomnessStartingEpoch
}

func (ss p2pNetSchedule) GetNetworkID() NetworkID {
	return P2PNet
}

// GetShardingStructure is the sharding structure for p2pNet.
func (ss p2pNetSchedule) GetShardingStructure(numShard, shardID int) []map[string]interface{} {
	return genShardingStructure(numShard, shardID, P2PNetHTTPPattern, P2PNetWSPattern)
}

var p2pNetReshardingEpoch = []*big.Int{
	big.NewInt(0),
	params.P2PNetChainConfig.StakingEpoch,
}

var p2pNetV0 = MustNewInstance(4, 75, 75, numeric.OneDec(), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, p2pNetReshardingEpoch, P2PNetSchedule.BlocksPerEpoch())
var p2pNetV1 = MustNewInstance(4, 100, 75, numeric.MustNewDecFromStr("0.9"), genesis.TNHarmonyAccounts, genesis.TNFoundationalAccounts, p2pNetReshardingEpoch, P2PNetSchedule.BlocksPerEpoch())
