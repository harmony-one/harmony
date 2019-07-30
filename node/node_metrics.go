package node

import (
	"time"

	metrics	"github.com/harmony-one/harmony/api/service/monitoringservice"
	"github.com/harmony-one/harmony/internal/utils"
)

// UpdateBlockHeightForMetrics updates block height for monitoring service.
func (node *Node) UpdateBlockHeightForMetrics() {
	utils.GetLogInstance().Info("[Monitoring Service] Update block height for metrics")
	prevBlockHeight := uint64(0)
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

// UpdateConnectionsNumberForMetrics uppdates connections number for monitoring service.
func (node *Node) UpdateConnectionsNumberForMetrics() {
	utils.GetLogInstance().Info("[Monitoring Service] Update connections number for metrics")
	prevNumPeers := 0
	for range time.Tick(1000 * time.Millisecond) {
		curNumPeers := node.numPeers
		if curNumPeers == prevNumPeers {
			continue
		}

		metrics.UpdateConnectionsNumber(curNumPeers)
		prevNumPeers = curNumPeers
	}
}
