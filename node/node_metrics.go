package node

import (
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

// UpdateTxPoolSizeForMetrics updates tx pool size for metrics service.
func (node *Node) UpdateTxPoolSizeForMetrics(txPoolSize uint64) {
	utils.Logger().Info().Msgf("Updating metrics tx pool size %d", txPoolSize)
	metrics.UpdateTxPoolSize(txPoolSize)
}

// UpdateBalanceForMetrics uppdates node balance for metrics service.
func (node *Node) UpdateBalanceForMetrics() {
	curBalance, err := node.GetBalanceOfAddress(node.Consensus.SelfAddress)
	if err != nil {
		return
	}
	utils.Logger().Info().Msgf("Updating metrics node balance %d", curBalance.Uint64())
	metrics.UpdateNodeBalance(curBalance)
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

// UpdateIsLeaderForMetrics updates if node is a leader now for metrics serivce.
func (node *Node) UpdateIsLeaderForMetrics() {
	if node.Consensus.LeaderPubKey.SerializeToHexStr() == node.Consensus.PubKey.SerializeToHexStr() {
		utils.Logger().Info().Msgf("Node %s is a leader now", node.Consensus.PubKey.SerializeToHexStr())
		metrics.UpdateIsLeader(true)
	} else {
		utils.Logger().Info().Msgf("Node %s is not a leader now", node.Consensus.PubKey.SerializeToHexStr())
		metrics.UpdateIsLeader(false)
	}
}

// CollectMetrics collects metrics: block height, connections number, node balance, block reward, last consensus, accepted blocks.
func (node *Node) CollectMetrics() {
	utils.Logger().Info().Msg("[Metrics Service] Update metrics")
	prevNumPeers := 0
	prevBlockHeight := uint64(0)
	prevLastConsensusTime := int64(0)
	for range time.Tick(100 * time.Millisecond) {
		prevBlockHeight = node.UpdateBlockHeightForMetrics(prevBlockHeight)
		prevNumPeers = node.UpdateConnectionsNumberForMetrics(prevNumPeers)
		prevLastConsensusTime = node.UpdateLastConsensusTimeForMetrics(prevLastConsensusTime)
		node.UpdateBalanceForMetrics()
		node.UpdateTxPoolSizeForMetrics(node.TxPool.GetTxPoolSize())
		node.UpdateIsLeaderForMetrics()
	}
}
