package node

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
)

// Constants of lower bound limit of a new block.
const (
	ConsensusTimeOut = 20
	PeriodicBlock    = 200 * time.Millisecond
)

// WaitForConsensusReadyv2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
// TODO: clean pending transactions for validators; or validators not prepare pending transactions
func (node *Node) WaitForConsensusReadyv2(readySignal chan struct{}, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		// TODO: remove it later
		firstTime := true

		utils.GetLogInstance().Debug("Waiting for Consensus ready")
		time.Sleep(30 * time.Second) // Wait for other nodes to be ready (test-only)

		timeoutCount := 0
		var newBlock *types.Block

		// Set up the very first deadline.
		deadline := time.Now().Add(node.BlockPeriod)
		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.GetLogInstance().Debug("Consensus new block proposal: STOPPED!")
				return
			case <-time.After(ConsensusTimeOut * time.Second):
				if node.Consensus.PubKey.IsEqual(node.Consensus.LeaderPubKey) {
					utils.GetLogInstance().Debug("Leader consensus timeout, retry!", "count", timeoutCount)
					//node.Consensus.ResetState()
					timeoutCount++
					if newBlock != nil {
						// Send the new block to Consensus so it can be confirmed.
						node.BlockChannel <- newBlock
					}
				}
			case <-readySignal:
				for {
					time.Sleep(PeriodicBlock)
					if time.Now().Before(deadline) {
						continue
					}

					//TODO: remove it after shard0 fix
					if node.Consensus.NeedsBlockRecovery(node.Blockchain().CurrentBlock().NumberU64()) && firstTime {
						firstTime = false
						newBlock := node.Blockchain().CurrentBlock()
						node.BlockChannel <- newBlock
						break
					}

					coinbase := node.Consensus.SelfAddress
					// Normal tx block consensus
					selectedTxs := types.Transactions{} // Empty transaction list
					if node.NodeConfig.GetNetworkType() != nodeconfig.Mainnet {
						selectedTxs = node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock, coinbase)
						if err := node.Worker.UpdateCurrent(coinbase); err != nil {
							utils.GetLogger().Error("Failed updating worker's state", "Error", err)
						}
					}
					utils.GetLogInstance().Info("PROPOSING NEW BLOCK ------------------------------------------------", "blockNum", node.Blockchain().CurrentBlock().NumberU64()+1, "selectedTxs", len(selectedTxs))
					if err := node.Worker.CommitTransactions(selectedTxs, coinbase); err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot commit transactions").
								WithCause(err))
					}
					sig, mask, err := node.Consensus.LastCommitSig()
					if err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("Cannot got commit signatures from last block").
								WithCause(err))
						continue
					}
					viewID := node.Consensus.GetViewID()
					// add aggregated commit signatures from last block, except for the first two blocks

					if node.NodeConfig.GetNetworkType() == nodeconfig.Mainnet {
						if err = node.Worker.UpdateCurrent(coinbase); err != nil {
							utils.GetLogger().Debug("Failed updating worker's state", "Error", err)
							continue
						}
					}

					newBlock, err = node.Worker.Commit(sig, mask, viewID, coinbase)

					if err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot commit new block").
								WithCause(err))
						continue
					} else if err := node.proposeShardStateWithoutBeaconSync(newBlock); err != nil {
						ctxerror.Log15(utils.GetLogger().Error,
							ctxerror.New("cannot add shard state").
								WithCause(err))
					} else {
						utils.GetLogInstance().Debug("Successfully proposed new block", "blockNum", newBlock.NumberU64(), "numTxs", newBlock.Transactions().Len())

						// Set deadline will be BlockPeriod from now at this place. Announce stage happens right after this.
						deadline = time.Now().Add(node.BlockPeriod)
						// Send the new block to Consensus so it can be confirmed.
						node.BlockChannel <- newBlock
						break
					}
				}
			}
		}
	}()
}

func (node *Node) proposeShardStateWithoutBeaconSync(block *types.Block) error {
	if !core.IsEpochLastBlock(block) {
		return nil
	}
	nextEpoch := new(big.Int).Add(block.Header().Epoch, common.Big1)
	shardState := core.GetShardState(nextEpoch)
	return block.AddShardState(shardState)
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
	return block.AddShardState(shardState)
}

func (node *Node) proposeLocalShardState(block *types.Block) {
	logger := block.Logger(utils.Logger())
	// TODO ek – read this from beaconchain once BC sync is fixed
	if node.nextShardState.master == nil {
		logger.Debug().Msg("yet to receive master proposal from beaconchain")
		return
	}

	nlogger := logger.With().
		Uint64("nextEpoch", node.nextShardState.master.Epoch).
		Time("proposeTime", node.nextShardState.proposeTime).
		Logger()
	logger = &nlogger
	if time.Now().Before(node.nextShardState.proposeTime) {
		logger.Debug().Msg("still waiting for shard state to propagate")
		return
	}
	masterShardState := node.nextShardState.master.ShardState
	var localShardState types.ShardState
	committee := masterShardState.FindCommitteeByID(block.ShardID())
	if committee != nil {
		logger.Info().Msg("found local shard info; proposing it")
		localShardState = append(localShardState, *committee)
	} else {
		logger.Info().Msg("beacon committee disowned us; proposing nothing")
		// Leave local proposal empty to signal the end of shard (disbanding).
	}
	err := block.AddShardState(localShardState)
	if err != nil {
		logger.Error().Err(err).Msg("Failed proposin local shard state")
	}
}
