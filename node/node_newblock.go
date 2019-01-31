package node

import (
	"time"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// WaitForConsensusReady listen for the readiness signal from consensus and generate new block for consensus.
func (node *Node) WaitForConsensusReady(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.GetLogInstance().Debug("Waiting for Consensus ready", "node", node)
		time.Sleep(15 * time.Second) // Wait for other nodes to be ready (test-only)

		firstTime := true
		var newBlock *types.Block
		timeoutCount := 0
		for {
			// keep waiting for Consensus ready
			select {
			case <-readySignal:
				time.Sleep(100 * time.Millisecond) // Delay a bit so validator is catched up (test-only).
			case <-time.After(200 * time.Second):
				node.Consensus.ResetState()
				timeoutCount++
				utils.GetLogInstance().Debug("Consensus timeout, retry!", "count", timeoutCount, "node", node)
			case <-stopChan:
				return
			}

			for {
				// threshold and firstTime are for the test-only built-in smart contract tx. TODO: remove in production
				threshold := 1
				if firstTime {
					threshold = 2
					firstTime = false
				}
				utils.GetLogInstance().Debug("STARTING BLOCK", "threshold", threshold, "pendingTransactions", len(node.pendingTransactions))
				if len(node.pendingTransactions) >= threshold {
					// Normal tx block consensus
					selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
					if len(selectedTxs) != 0 {
						node.Worker.CommitTransactions(selectedTxs)
						block, err := node.Worker.Commit()
						if err != nil {
							utils.GetLogInstance().Debug("Failed commiting new block", "Error", err)
						} else {
							newBlock = block
							break
						}
					}
				}
				// If not enough transactions to run Consensus,
				// periodically check whether we have enough transactions to package into block.
				time.Sleep(1 * time.Second)
			}
			// Send the new block to Consensus so it can be confirmed.
			if newBlock != nil {
				node.BlockChannel <- newBlock
			}
		}
	}()
}
