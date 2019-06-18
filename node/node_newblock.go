package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants of lower bound limit of a new block.
const (
	DefaultThreshold   = 1
	FirstTimeThreshold = 2
	ConsensusTimeOut   = 30
	PeriodicBlock      = 1 * time.Second
	BlockPeriod        = 10 * time.Second
)

// WaitForConsensusReadyv2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
// TODO: clean pending transactions for validators; or validators not prepare pending transactions
func (node *Node) WaitForConsensusReadyv2(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.GetLogInstance().Debug("Waiting for Consensus ready")
		time.Sleep(30 * time.Second) // Wait for other nodes to be ready (test-only)

		firstTime := true
		timeoutCount := 0
		var newBlock *types.Block
		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.GetLogInstance().Debug("Consensus new block proposal: STOPPED!")
				return
			case <-time.After(ConsensusTimeOut * time.Second):
				if node.Consensus.PubKey.IsEqual(node.Consensus.LeaderPubKey) {
					utils.GetLogInstance().Debug("Consensus timeout, retry!", "count", timeoutCount)
					node.Consensus.ResetState()
					timeoutCount++
					if newBlock != nil {
						// Send the new block to Consensus so it can be confirmed.
						node.BlockChannel <- newBlock
					}
				}
			case <-readySignal:
				firstTry := true
				deadline := time.Now().Add(BlockPeriod)
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
					if len(node.pendingTransactions) < threshold && time.Now().Before(deadline) {
						continue
					}
					deadline = time.Now().Add(BlockPeriod)
					// Normal tx block consensus
					selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
					utils.GetLogInstance().Info("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.Blockchain().CurrentBlock().NumberU64()+1, "threshold", threshold, "selectedTxs", len(selectedTxs))
					if err := node.Worker.CommitTransactions(selectedTxs); err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot commit transactions").
								WithCause(err))
					}
					newBlock, err := node.Worker.Commit()
					if err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot commit new block").
								WithCause(err))
						continue
					} else if err := node.proposeShardState(newBlock); err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot add shard state").
								WithCause(err))
					} else {
						utils.GetLogInstance().Debug("Successfully proposed new block", "blockNum", newBlock.NumberU64(), "numTxs", newBlock.Transactions().Len())

						// Send the new block to Consensus so it can be confirmed.
						node.BlockChannel <- newBlock
						break
					}
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
