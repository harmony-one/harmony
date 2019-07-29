package node

import (
	"time"
    metrics	"github.com/harmony-one/harmony/api/service/monitoringservice"
	"github.com/harmony-one/harmony/internal/utils"
)

// UpdateConnectionsNumberForMetrics uppdates connections number for monitoring service.
func (node *Node) UpdateConnectionsNumberForMetrics() {
	utils.GetLogInstance().Info("[Monitoring Service] Update connections number for metrics")
	prevNumPeers := node.numPeers
	for range time.Tick(1000 * time.Millisecond) {
		curNumPeers := node.numPeers
		if curNumPeers == prevNumPeers {
			continue
		}

		metrics.UpdateConnectionsNumber(curNumPeers)
		prevNumPeers = curNumPeers
	}
}

// UpdateBlockHeightForMetrics updates block height for monitoring service.
func (node *Node) UpdateBlockHeightForMetrics() {
	utils.GetLogInstance().Info("[Monitoring Service] Update block height for metrics")
	prevBlockHeight := node.Blockchain().CurrentBlock().NumberU64()
	for range time.Tick(3000 * time.Millisecond) {
		curBlock := node.Blockchain().CurrentBlock()
		curBlockHeight := curBlock.NumberU64()
		if curBlockHeight == prevBlockHeight {
			continue
		}

		utils.GetLogInstance().Info("Updating metrics block height", "blockHeight", curBlockHeight)

		metrics.UpdateBlockHeight(curBlockHeight, curBlock.Header().Time.Int64())
		prevBlockHeight = curBlockHeight
	}
}

// Collects metrics.
func (node *Node) CollectMetrics() {
	// utils.Logger().Info().Msg("Init metrics db.")
	//node.metricsStorage = utils.GetMetricsStorageInstance(node.ClientPeer.IP, node.ClientPeer.Port, true)
	// flush peers number each second
}



// AddNewBlockForExplorer add new block for explorer.
/*
func (node *Node) UpdateNodeBalance() {
	utils.GetLogInstance().Info("[Monitoring Service] Update block reward for metrics")
	// Search for the next block in PbftLog and commit the block into blockchain for explorer node.
	prevBlockHeight := node.CurrentStakes[node.StackingAccount]
	for {
		curBlock := node.Blockchain().CurrentBlock()
		curBlockHeight := curBlock.NumberU64()
		if curBlockHeight == prevBlockHeight {
			continue
		}

		utils.GetLogInstance().Info("Updating metrics block height", "blockHeight", curBlockHeight)
		pushedTime := time.Now().Unix()

		metrics.UpdateBlockHeight(curBlockHeight, curBlock.Header().Time, pushedTime)
		prevBlockHeight = curBlockHeight
	}
}

// ExplorerMessageHandler passes received message in node_handler to explorer service
func (node *Node) commitBlockForExplorer(block *types.Block) {
	if block.ShardID() != node.NodeConfig.ShardID {
		return
	}
	// Dump new block into level db.
	utils.GetLogInstance().Info("[Explorer] Committing block into explorer DB", "blockNum", block.NumberU64())
	explorer.GetStorageInstance(node.SelfPeer.IP, node.SelfPeer.Port, true).Dump(block, block.NumberU64())

	curNum := block.NumberU64()
	if curNum-100 > 0 {
		node.Consensus.PbftLog.DeleteBlocksLessThan(curNum - 100)
		node.Consensus.PbftLog.DeleteMessagesLessThan(curNum - 100)
	}
}*/

