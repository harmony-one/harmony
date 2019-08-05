package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	metrics "github.com/harmony-one/harmony/api/service/metrics"
	"github.com/harmony-one/harmony/core"
	"github.com/rs/zerolog/log"
)

// UpdateBlockHeightForMetrics updates block height for metrics service.
func (node *Node) UpdateBlockHeightForMetrics(prevBlockHeight uint64) uint64 {
	curBlock := node.Blockchain().CurrentBlock()
	curBlockHeight := curBlock.NumberU64()
	if curBlockHeight == prevBlockHeight {
		return prevBlockHeight
	}
	log.Info().Msgf("Updating metrics block height %d", curBlockHeight)
	metrics.UpdateBlockHeight(curBlockHeight)
	return curBlockHeight
}

// UpdateConnectionsNumberForMetrics uppdates connections number for metrics service.
func (node *Node) UpdateConnectionsNumberForMetrics(prevNumPeers int) int {
	curNumPeers := node.numPeers
	if curNumPeers == prevNumPeers {
		return prevNumPeers
	}
	log.Info().Msgf("Updating metrics connections number %d", curNumPeers)
	metrics.UpdateConnectionsNumber(curNumPeers)
	return curNumPeers
}

// UpdateBalanceForMetrics uppdates node balance for metrics service.
func (node *Node) UpdateBalanceForMetrics(prevBalance *big.Int) *big.Int {
	_, account := core.ShardingSchedule.InstanceForEpoch(big.NewInt(core.GenesisEpoch)).FindAccount(node.NodeConfig.ConsensusPubKey.SerializeToHexStr())
	if account == nil {
		return prevBalance
	}
	curBalance, err := node.GetBalanceOfAddress(common.HexToAddress(account.Address))
	if err != nil || curBalance.Cmp(prevBalance) == 0 {
		return prevBalance
	}
	log.Info().Msgf("Updating metrics node balance %d", curBalance.Uint64())
	metrics.UpdateNodeBalance(curBalance)
	return curBalance
}

// UpdateLastConsensusTimeForMetrics uppdates last consensus reached time for metrics service.
func (node *Node) UpdateLastConsensusTimeForMetrics(prevLastConsensusTime int64) int64 {
	lastConsensusTime := node.lastConsensusTime
	if lastConsensusTime == prevLastConsensusTime {
		return prevLastConsensusTime
	}
	log.Info().Msgf("Updating metrics last consensus time reached %d", lastConsensusTime)
	metrics.UpdateLastConsensus(lastConsensusTime)
	return lastConsensusTime
}

// CollectMetrics collects metrics: block height, connections number, node balance, block reward, last consensus, accepted blocks.
func (node *Node) CollectMetrics() {
	log.Info().Msg("[Metrics Service] Update metrics")
	prevNumPeers := 0
	prevBlockHeight := uint64(0)
	prevBalance := big.NewInt(0)
	prevLastConsensusTime := int64(0)
	for range time.Tick(100 * time.Millisecond) {
		prevBlockHeight = node.UpdateBlockHeightForMetrics(prevBlockHeight)
		prevNumPeers = node.UpdateConnectionsNumberForMetrics(prevNumPeers)
		prevBalance = node.UpdateBalanceForMetrics(prevBalance)
		prevLastConsensusTime = node.UpdateLastConsensusTimeForMetrics(prevLastConsensusTime)
	}
}
