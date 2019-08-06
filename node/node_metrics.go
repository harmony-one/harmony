package node

import (
	"math/big"
	"time"

	metrics "github.com/harmony-one/harmony/api/service/metrics"
	"github.com/harmony-one/harmony/internal/utils"
)

// UpdateBlockHeightForMetrics updates block height for metrics service.
func (node *Node) UpdateBlockHeightForMetrics(prevBlockHeight uint64) uint64 {
	curBlock := node.Blockchain().CurrentBlock()
	curBlockHeight := curBlock.NumberU64()
	if curBlockHeight == prevBlockHeight {
		return prevBlockHeight
	}
	utils.Logger().Info().Msgf("Updating metrics block height %d", curBlockHeight)
	metrics.UpdateBlockHeight(curBlockHeight)
	blockReward := node.Consensus.GetBlockReward()
	if blockReward != nil {
		utils.Logger().Info().Msgf("Updating metrics block reward %d", blockReward.Uint64())
		metrics.UpdateBlockReward(blockReward)
	}
	return curBlockHeight
}

// UpdateConnectionsNumberForMetrics uppdates connections number for metrics service.
func (node *Node) UpdateConnectionsNumberForMetrics(prevNumPeers int) int {
	curNumPeers := node.numPeers
	if curNumPeers == prevNumPeers {
		return prevNumPeers
	}
	utils.Logger().Info().Msgf("Updating metrics connections number %d", curNumPeers)
	metrics.UpdateConnectionsNumber(curNumPeers)
	return curNumPeers
}

// UpdateBalanceForMetrics uppdates node balance for metrics service.
func (node *Node) UpdateBalanceForMetrics(prevBalance *big.Int) *big.Int {
	curBalance, err := node.GetBalanceOfAddress(node.Consensus.SelfAddress)
	if err != nil || curBalance.Cmp(prevBalance) == 0 {
		return prevBalance
	}
	utils.Logger().Info().Msgf("Updating metrics node balance %d", curBalance.Uint64())
	metrics.UpdateNodeBalance(curBalance)
	return curBalance
}

// UpdateLastConsensusTimeForMetrics uppdates last consensus reached time for metrics service.
func (node *Node) UpdateLastConsensusTimeForMetrics(prevLastConsensusTime int64) int64 {
	lastConsensusTime := node.lastConsensusTime
	if lastConsensusTime == prevLastConsensusTime {
		return prevLastConsensusTime
	}
	utils.Logger().Info().Msgf("Updating metrics last consensus time reached %d", lastConsensusTime)
	metrics.UpdateLastConsensus(lastConsensusTime)
	return lastConsensusTime
}

// CollectMetrics collects metrics: block height, connections number, node balance, block reward, last consensus, accepted blocks.
func (node *Node) CollectMetrics() {
	utils.Logger().Info().Msg("[Metrics Service] Update metrics")
	prevNumPeers := 0
	prevBlockHeight := uint64(0)
	prevBalance := big.NewInt(0)
	prevLastConsensusTime := int64(0)
	for range time.Tick(1000 * time.Millisecond) {
		prevBlockHeight = node.UpdateBlockHeightForMetrics(prevBlockHeight)
		prevNumPeers = node.UpdateConnectionsNumberForMetrics(prevNumPeers)
		prevBalance = node.UpdateBalanceForMetrics(prevBalance)
		prevLastConsensusTime = node.UpdateLastConsensusTimeForMetrics(prevLastConsensusTime)
	}
}
