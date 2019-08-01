package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	metrics "github.com/harmony-one/harmony/api/service/metrics"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
)

// Metrics events types
const (
	BlockReward      = 1
	ConsensusReached = 2
)

// UpdateBlockHeightForMetrics updates block height for metrics service.
func (node *Node) UpdateBlockHeightForMetrics(prevBlockHeight uint64) uint64 {
	curBlock := node.Blockchain().CurrentBlock()
	curBlockHeight := curBlock.NumberU64()
	if curBlockHeight == prevBlockHeight {
		return prevBlockHeight
	}
	utils.GetLogInstance().Info("Updating metrics block height", "blockHeight", curBlockHeight)
	metrics.UpdateBlockHeight(curBlockHeight)
	return curBlockHeight
}

// UpdateConnectionsNumberForMetrics uppdates connections number for metrics service.
func (node *Node) UpdateConnectionsNumberForMetrics(prevNumPeers int) int {
	curNumPeers := node.numPeers
	if curNumPeers == prevNumPeers {
		return prevNumPeers
	}
	utils.GetLogInstance().Info("Updating metrics connections number", "connectionsNumber", curNumPeers)
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
	utils.GetLogInstance().Info("Updating metrics node balance", "nodeBalance", curBalance)
	metrics.UpdateNodeBalance(curBalance)
	return curBalance
}

// CollectMetrics collects metrics: block height, connections number, node balance, block reward, last consensus, accepted blocks.
func (node *Node) CollectMetrics() {
	utils.GetLogInstance().Info("[Metrics Service] Update metrics")
	prevNumPeers := 0
	prevBlockHeight := uint64(0)
	prevBalance := big.NewInt(0)
	for range time.Tick(100 * time.Millisecond) {
		prevBlockHeight = node.UpdateBlockHeightForMetrics(prevBlockHeight)
		prevNumPeers = node.UpdateConnectionsNumberForMetrics(prevNumPeers)
		prevBalance = node.UpdateBalanceForMetrics(prevBalance)
		select {
		case event := <-node.metricsChan:
			switch event.Event {
			case ConsensusReached:
				metrics.UpdateLastConsensus(event.Time)
			case BlockReward:
				metrics.UpdateBlockReward(big.NewInt(0))
			}
		default:
		}
	}
}
