package node

import (
	"time"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants of lower bound limit of a new block.
const (
	DefaultThreshold   = 1
	FirstTimeThreshold = 2
	ConsensusTimeOut   = 10
	PeriodicBlock      = 3 * time.Second
)

// WaitForConsensusReady listen for the readiness signal from consensus and generate new block for consensus.
func (node *Node) WaitForConsensusReady(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.GetLogInstance().Debug("Waiting for Consensus ready")
		time.Sleep(30 * time.Second) // Wait for other nodes to be ready (test-only)

		firstTime := true
		var newBlock *types.Block
		timeoutCount := 0
		for {
			// keep waiting for Consensus ready
			select {
			case <-readySignal:
				time.Sleep(1000 * time.Millisecond) // Delay a bit so validator is catched up (test-only).
			case <-time.After(ConsensusTimeOut * time.Second):
				node.Consensus.ResetState()
				timeoutCount++
				utils.GetLogInstance().Debug("Consensus timeout, retry!", "count", timeoutCount)
				// FIXME: retry is not working, there is no retry logic here. It will only wait for new transaction.
			case <-stopChan:
				utils.GetLogInstance().Debug("Consensus propose new block: STOPPED!")
				return
			}

			for {
				// threshold and firstTime are for the test-only built-in smart contract tx.
				// TODO: remove in production
				threshold := DefaultThreshold
				if firstTime {
					threshold = FirstTimeThreshold
					firstTime = false
				}
				if len(node.pendingTransactions) >= threshold {
					utils.GetLogInstance().Debug("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.blockchain.CurrentBlock().NumberU64()+1, "threshold", threshold, "pendingTransactions", len(node.pendingTransactions))
					// Normal tx block consensus
					selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
					if len(selectedTxs) != 0 {
						node.Worker.CommitTransactions(selectedTxs)
						block, err := node.Worker.Commit()
						if err != nil {
							utils.GetLogInstance().Debug("Failed committing new block", "Error", err)
						} else {
							if node.Consensus.ShardID == 0 {
								// add new shard state if it's epoch block
								// TODO: bug fix - the stored shard state between here and PostConsensusProcessing are different.
								//node.addNewShardState(block)
							}
							newBlock = block
							utils.GetLogInstance().Debug("Successfully proposed new block", "blockNum", block.NumberU64(), "numTxs", block.Transactions().Len())
							break
						}
					}
				}
				// If not enough transactions to run Consensus,
				// periodically check whether we have enough transactions to package into block.
				time.Sleep(PeriodicBlock)
			}
			// Send the new block to Consensus so it can be confirmed.
			if newBlock != nil {
				utils.GetLogInstance().Debug("Consensus sending new block to block channel")
				node.BlockChannel <- newBlock
				utils.GetLogInstance().Debug("Consensus sent new block to block channel")
			}
		}
	}()
}

func (node *Node) addNewShardState(block *types.Block) {
	logger := utils.GetLogInstance().New("blockHash", block.Hash())
	shardState := node.blockchain.GetNewShardState(block, &node.CurrentStakes)
	if shardState != nil {
		shardHash := shardState.Hash()
		logger.Debug("[Shard State Hash] adding new shard state", "shardHash", shardHash)
		block.AddShardState(shardState)
	}
}

// WaitForConsensusReadyv2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
func (node *Node) WaitForConsensusReadyv2(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.GetLogInstance().Debug("Waiting for Consensus ready")
		time.Sleep(30 * time.Second) // Wait for other nodes to be ready (test-only)

		firstTime := true
		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.GetLogInstance().Debug("Consensus propose new block: STOPPED!")
				return

			case <-readySignal:
				firstTry := true
				for {
					if !firstTry {
						time.Sleep(PeriodicBlock)
					}
					firstTry = false
					// threshold and firstTime are for the test-only built-in smart contract tx.
					// TODO: remove in production
					threshold := DefaultThreshold
					if firstTime {
						threshold = FirstTimeThreshold
						firstTime = false
					}
					if len(node.pendingTransactions) < threshold {
						continue
					}
					// Normal tx block consensus
					selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
					if len(selectedTxs) == 0 {
						continue
					}
					utils.GetLogInstance().Debug("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.blockchain.CurrentBlock().NumberU64()+1, "threshold", threshold, "selectedTxs", len(selectedTxs))
					node.Worker.CommitTransactions(selectedTxs)
					block, err := node.Worker.Commit()
					if err != nil {
						utils.GetLogInstance().Debug("Failed committing new block", "Error", err)
						continue
					}
					if node.Consensus.ShardID == 0 {
						// add new shard state if it's epoch block
						// TODO: bug fix - the stored shard state between here and PostConsensusProcessing are different.
						//node.addNewShardState(block)
					}
					newBlock := block
					utils.GetLogInstance().Debug("Successfully proposed new block", "blockNum", block.NumberU64(), "numTxs", block.Transactions().Len())

					// Send the new block to Consensus so it can be confirmed.
					node.BlockChannel <- newBlock
					break
				}
			}
		}
	}()
}
