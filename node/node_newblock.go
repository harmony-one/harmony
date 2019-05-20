package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/harmony-one/harmony/core"
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
					utils.GetLogInstance().Debug("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.Blockchain().CurrentBlock().NumberU64()+1, "threshold", threshold, "pendingTransactions", len(node.pendingTransactions))
					// Normal tx block consensus
					selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
					if len(selectedTxs) != 0 {
						node.Worker.CommitTransactions(selectedTxs)
						block, err := node.Worker.Commit()
						if err != nil {
							utils.GetLogInstance().Debug("Failed committing new block", "Error", err)
						} else {
							node.proposeShardState(block)
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
					utils.GetLogInstance().Debug("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.Blockchain().CurrentBlock().NumberU64()+1, "threshold", threshold, "selectedTxs", len(selectedTxs))
					node.Worker.CommitTransactions(selectedTxs)
					block, err := node.Worker.Commit()
					if err != nil {
						utils.GetLogInstance().Debug("Failed committing new block", "Error", err)
						continue
					}
					node.proposeShardState(block)
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

func (node *Node) proposeShardState(block *types.Block) error {
	switch node.Consensus.ShardID {
	case 0:
		return node.proposeBeaconShardState(block)
	default:
		node.proposeLocalShardState(block)
		return nil
	}
}

func (node *Node) proposeBeaconShardState(block *types.Block) error {
	// TODO ek - replace this with variable epoch logic.
	if !core.IsEpochLastBlock(block) {
		// We haven't reached the end of this epoch; don't propose yet.
		return nil
	}
	nextEpoch := new(big.Int).Add(block.Header().Epoch, common.Big1)
	shardState, err := core.CalculateNewShardState(
		node.Blockchain(), nextEpoch, &node.CurrentStakes)
	if err != nil {
		return err
	}
	block.AddShardState(shardState)
	return nil
}

func (node *Node) proposeLocalShardState(block *types.Block) {
	logger := block.Logger(utils.GetLogInstance())
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }
	// TODO ek – read this from beaconchain once BC sync is fixed
	if node.nextShardState.master == nil {
		getLogger().Debug("yet to receive master proposal from beaconchain")
		return
	}
	logger = logger.New(
		"nextEpoch", node.nextShardState.master.Epoch,
		"proposeTime", node.nextShardState.proposeTime)
	if time.Now().Before(node.nextShardState.proposeTime) {
		getLogger().Debug("still waiting for shard state to propagate")
		return
	}
	masterShardState := node.nextShardState.master.ShardState
	var localShardState types.ShardState
	committee := masterShardState.FindCommitteeByID(block.ShardID())
	if committee != nil {
		getLogger().Info("found local shard info; proposing it")
		localShardState = append(localShardState, *committee)
	} else {
		getLogger().Info("beacon committee disowned us; proposing nothing")
		// Leave local proposal empty to signal the end of shard (disbanding).
	}
	block.AddShardState(localShardState)
}
