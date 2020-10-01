package consensus

import (
	"sync"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// MaxViewIDDiff limits the received view ID to only 249 further from the current view ID
const MaxViewIDDiff = 249

// State contains current mode and current viewID
type State struct {
	mode    Mode
	modeMux sync.RWMutex

	// current view id in normal mode
	// it changes per successful consensus
	blockViewID uint64
	cViewMux    sync.RWMutex

	// view changing id is used during view change mode
	// it is the next view id
	viewChangingID uint64

	viewMux sync.RWMutex
}

// Mode return the current node mode
func (pm *State) Mode() Mode {
	pm.modeMux.RLock()
	defer pm.modeMux.RUnlock()
	return pm.mode
}

// SetMode set the node mode as required
func (pm *State) SetMode(s Mode) {
	pm.modeMux.Lock()
	defer pm.modeMux.Unlock()
	pm.mode = s
}

// GetCurBlockViewID return the current view id
func (pm *State) GetCurBlockViewID() uint64 {
	pm.cViewMux.RLock()
	defer pm.cViewMux.RUnlock()
	return pm.blockViewID
}

// SetCurBlockViewID sets the current view id
func (pm *State) SetCurBlockViewID(viewID uint64) {
	pm.cViewMux.Lock()
	defer pm.cViewMux.Unlock()
	pm.blockViewID = viewID
}

// GetViewChangingID return the current view changing id
// It is meaningful during view change mode
func (pm *State) GetViewChangingID() uint64 {
	pm.viewMux.RLock()
	defer pm.viewMux.RUnlock()
	return pm.viewChangingID
}

// SetViewChangingID set the current view changing id
// It is meaningful during view change mode
func (pm *State) SetViewChangingID(id uint64) {
	pm.viewMux.Lock()
	defer pm.viewMux.Unlock()
	pm.viewChangingID = id
}

// GetViewChangeDuraion return the duration of the current view change
// It increase in the power of difference betweeen view changing ID and current view ID
func (pm *State) GetViewChangeDuraion() time.Duration {
	pm.viewMux.RLock()
	pm.cViewMux.RLock()
	defer pm.viewMux.RUnlock()
	defer pm.cViewMux.RUnlock()
	diff := int64(pm.viewChangingID - pm.blockViewID)
	return time.Duration(diff * diff * int64(viewChangeDuration))
}

// GetNextLeaderKey uniquely determine who is the leader for given viewID
func (consensus *Consensus) GetNextLeaderKey(viewID uint64) *bls.PublicKeyWrapper {
	gap := 1
	if viewID > consensus.GetCurBlockViewID() {
		gap = int(viewID - consensus.GetCurBlockViewID())
	}
	consensus.getLogger().Info().
		Str("leaderPubKey", consensus.LeaderPubKey.Bytes.Hex()).
		Uint64("newViewID", viewID).
		Uint64("myCurBlockViewID", consensus.GetCurBlockViewID()).
		Msg("[GetNextLeaderKey] got leaderPubKey from coinbase")
	wasFound, next := consensus.Decider.NextAfter(consensus.LeaderPubKey, gap)
	if !wasFound {
		consensus.getLogger().Warn().
			Str("key", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("GetNextLeaderKey: currentLeaderKey not found")
	}
	return next
}

func createTimeout() map[TimeoutType]*utils.Timeout {
	timeouts := make(map[TimeoutType]*utils.Timeout)
	timeouts[timeoutConsensus] = utils.NewTimeout(phaseDuration)
	timeouts[timeoutViewChange] = utils.NewTimeout(viewChangeDuration)
	timeouts[timeoutBootstrap] = utils.NewTimeout(bootstrapDuration)
	return timeouts
}

// startViewChange send a new view change
// the viewID is the current viewID
func (consensus *Consensus) startViewChange(viewID uint64) {
	if consensus.disableViewChange {
		return
	}
	consensus.consensusTimeout[timeoutConsensus].Stop()
	consensus.consensusTimeout[timeoutBootstrap].Stop()
	consensus.current.SetMode(ViewChanging)
	consensus.SetViewChangingID(viewID)
	consensus.LeaderPubKey = consensus.GetNextLeaderKey(viewID)

	duration := consensus.current.GetViewChangeDuraion()
	consensus.getLogger().Warn().
		Uint64("viewID", viewID).
		Uint64("viewChangingID", consensus.GetViewChangingID()).
		Dur("timeoutDuration", duration).
		Str("NextLeader", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("[startViewChange]")

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	defer consensus.consensusTimeout[timeoutViewChange].Start()

	// for view change, send separate view change per public key
	// do not do multi-sign of view change message
	for _, key := range consensus.priKey {
		if !consensus.IsValidatorInCommittee(key.Pub.Bytes) {
			continue
		}
		msgToSend := consensus.constructViewChangeMessage(&key)
		consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
			p2p.ConstructMessage(msgToSend),
		)
	}
}

// onViewChange is called when the view change message is received.
func (consensus *Consensus) onViewChange(msg *msg_pb.Message) {
	consensus.getLogger().Debug().Msg("[onViewChange] Received ViewChange Message")
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onViewChange] Unable To Parse Viewchange Message")
		return
	}
	// if not leader, noop
	newLeaderKey := recvMsg.LeaderPubkey
	newLeaderPriKey, err := consensus.GetLeaderPrivateKey(newLeaderKey.Object)
	if err != nil {
		consensus.getLogger().Debug().
			Err(err).
			Str("Sender", recvMsg.SenderPubkey.Bytes.Hex()).
			Str("NextLeader", recvMsg.LeaderPubkey.Bytes.Hex()).
			Str("myBLSPubKey", consensus.priKey.GetPublicKeys().SerializeToHexStr()).
			Msg("[onViewChange] I am not the Leader")
		return
	}

	if consensus.Decider.IsQuorumAchieved(quorum.ViewChange) {
		consensus.getLogger().Info().
			Int64("have", consensus.Decider.SignersCount(quorum.ViewChange)).
			Int64("need", consensus.Decider.TwoThirdsSignersCount()).
			Str("validatorPubKey", recvMsg.SenderPubkey.Bytes.Hex()).
			Str("newLeaderKey", newLeaderKey.Bytes.Hex()).
			Msg("[onViewChange] Received Enough View Change Messages")
		return
	}

	if !consensus.onViewChangeSanityCheck(recvMsg) {
		return
	}

	// update the dictionary key if the viewID is first time received
	members := consensus.Decider.Participants()
	consensus.VC.AddViewIDKeyIfNotExist(recvMsg.ViewID, members)

	// do it once only per viewID/Leader
	if err := consensus.VC.InitPayload(consensus.FBFTLog,
		recvMsg.ViewID,
		recvMsg.BlockNum,
		newLeaderKey.Bytes.Hex(),
		consensus.priKey); err != nil {
		consensus.getLogger().Error().Err(err).Msg("Init Payload Error")
		return
	}

	msgType, preparedBlock, err := consensus.VC.VerifyViewChangeMsg(recvMsg, members)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Uint64("viewID", recvMsg.ViewID).
			Uint64("blockNum", recvMsg.BlockNum).
			Str("msgSender", recvMsg.SenderPubkey.Bytes.Hex()).
			Msg("Verify View Change Message Error")
		return
	}

	if msgType == M1 {
		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_PREPARED, recvMsg.BlockNum,
		)
		preparedMsg := consensus.FBFTLog.FindMessageByViewID(preparedMsgs, recvMsg.ViewID)
		if preparedMsg == nil {
			// create prepared message for new leader
			preparedMsg := FBFTMessage{
				MessageType: msg_pb.MessageType_PREPARED,
				ViewID:      recvMsg.ViewID,
				BlockNum:    recvMsg.BlockNum,
			}
			preparedMsg.BlockHash = common.Hash{}
			copy(preparedMsg.BlockHash[:], recvMsg.Payload[:32])
			preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
			copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
			preparedMsg.SenderPubkey = newLeaderKey
			consensus.getLogger().Info().Msg("[onViewChange] New Leader Prepared Message Added")
			consensus.FBFTLog.AddMessage(&preparedMsg)

			consensus.FBFTLog.AddBlock(preparedBlock)
		}
	}

	// received enough view change messages, change state to normal consensus
	if consensus.Decider.IsQuorumAchievedByMask(consensus.VC.GetViewIDBitmap(recvMsg.ViewID)) {
		consensus.getLogger().Info().Msg("[onViewChange] View Change Message Quorum Reached")
		consensus.current.SetMode(Normal)
		consensus.LeaderPubKey = newLeaderKey
		consensus.ResetState()
		if consensus.VC.IsM1PayloadEmpty() {
			// TODO(Chao): explain why ReadySignal is sent only in this case but not the other case.
			// Make sure the newly proposed block have the correct view ID
			consensus.SetCurBlockViewID(recvMsg.ViewID)
			go func() {
				consensus.ReadySignal <- struct{}{}
			}()
		} else {
			consensus.switchPhase("onViewChange", FBFTCommit, true)
			payload := consensus.VC.GetM1Payload()
			copy(consensus.blockHash[:], payload[:32])
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(payload, 32)

			if err != nil {
				consensus.getLogger().Error().Err(err).
					Msg("[onViewChange] ReadSignatureBitmapPayload Fail")
				return
			}

			consensus.aggregatedPrepareSig = aggSig
			consensus.prepareBitmap = mask
			// Leader sign and add commit message
			block := consensus.FBFTLog.GetBlockByHash(consensus.blockHash)
			if block == nil {
				consensus.getLogger().Warn().Msg("[onViewChange] failed to get prepared block for self commit")
				return
			}
			commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
				block.Epoch(), block.Hash(), block.NumberU64(), block.Header().ViewID().Uint64())
			for i, key := range consensus.priKey {
				if err := consensus.commitBitmap.SetKey(key.Pub.Bytes, true); err != nil {
					consensus.getLogger().Warn().Err(err).
						Msgf("[OnViewChange] New Leader commit bitmap set failed for key at index %d", i)
					continue
				}

				if _, err := consensus.Decider.SubmitVote(
					quorum.Commit,
					key.Pub.Bytes,
					key.Pri.SignHash(commitPayload),
					common.BytesToHash(consensus.blockHash[:]),
					block.NumberU64(),
					block.Header().ViewID().Uint64(),
				); err != nil {
					consensus.getLogger().Warn().Err(err).Msg("submit vote on viewchange commit failed")
					return
				}

			}
		}

		consensus.SetViewChangingID(recvMsg.ViewID)
		msgToSend := consensus.constructNewViewMessage(
			recvMsg.ViewID, newLeaderPriKey,
		)

		if err := consensus.msgSender.SendWithRetry(
			consensus.blockNum,
			msg_pb.MessageType_NEWVIEW,
			[]nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
			p2p.ConstructMessage(msgToSend),
		); err != nil {
			consensus.getLogger().Err(err).
				Msg("could not send out the NEWVIEW message")
		}
		consensus.getLogger().Info().
			Str("myKey", newLeaderKey.Bytes.Hex()).
			Hex("M1Payload", consensus.VC.GetM1Payload()).
			Msg("[onViewChange] Sent NewView Messge")

		consensus.SetCurBlockViewID(recvMsg.ViewID)
		consensus.ResetViewChangeState()
		consensus.consensusTimeout[timeoutViewChange].Stop()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Info().
			Str("myKey", newLeaderKey.Bytes.Hex()).
			Msg("[onViewChange] I am the New Leader")
	}
}

// onViewChange is called when validators received newView message from the new leader
// the validator needs to check the m3bitmap to see if the quorum is reached
// If the new view message contains payload (block), and at least one m1 message was
// collected by the new leader (m3count > m2count), the validator will create a new
// prepared message from the payload and commit it to the block
// Or the validator will enter announce phase to wait for the new block proposed
// from the new leader
func (consensus *Consensus) onNewView(msg *msg_pb.Message) {
	consensus.getLogger().Info().Msg("[onNewView] Received NewView Message")
	members := consensus.Decider.Participants()
	recvMsg, err := ParseNewViewMessage(msg, members)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onNewView] Unable to Parse NewView Message")
		return
	}

	// change view and leaderKey to keep in sync with network
	if consensus.blockNum != recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("myBlockNum", consensus.blockNum).
			Msg("[onNewView] Invalid block number")
		return
	}

	if !consensus.onNewViewSanityCheck(recvMsg) {
		return
	}
	preparedBlock, err := consensus.VC.VerifyNewViewMsg(recvMsg)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onNewView] Verify New View Msg Failed")
		return
	}

	m3Mask := recvMsg.M3Bitmap
	if !consensus.Decider.IsQuorumAchievedByMask(m3Mask) {
		consensus.getLogger().Warn().
			Msgf("[onNewView] Quorum Not achieved")
		return
	}

	m2Mask := recvMsg.M2Bitmap
	if m2Mask == nil || m2Mask.Bitmap == nil ||
		(m2Mask != nil && m2Mask.Bitmap != nil &&
			utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		// m1 is not empty, check it's valid
		blockHash := recvMsg.Payload[:32]
		aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
		if err != nil {
			consensus.getLogger().Error().Err(err).
				Msg("[onNewView] ReadSignatureBitmapPayload Failed")
			return
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash) {
			consensus.getLogger().Warn().
				Msg("[onNewView] Failed to Verify Signature for M1 (prepare) message")
			return
		}
		copy(consensus.blockHash[:], blockHash)
		consensus.aggregatedPrepareSig = aggSig
		consensus.prepareBitmap = mask
		// create prepared message from newview
		preparedMsg := FBFTMessage{
			MessageType: msg_pb.MessageType_PREPARED,
			ViewID:      recvMsg.ViewID,
			BlockNum:    recvMsg.BlockNum,
		}
		preparedMsg.BlockHash = common.Hash{}
		copy(preparedMsg.BlockHash[:], blockHash[:])
		preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
		copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
		preparedMsg.SenderPubkey = recvMsg.SenderPubkey
		consensus.FBFTLog.AddMessage(&preparedMsg)

		if preparedBlock != nil {
			consensus.FBFTLog.AddBlock(preparedBlock)
		}
	}

	// newView message verified success, override my state
	consensus.SetViewIDs(recvMsg.ViewID)
	consensus.LeaderPubKey = recvMsg.SenderPubkey
	consensus.ResetViewChangeState()

	// NewView message is verified, change state to normal consensus
	if preparedBlock != nil {
		// Construct and send the commit message
		commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
			preparedBlock.Epoch(), preparedBlock.Hash(), preparedBlock.NumberU64(), preparedBlock.Header().ViewID().Uint64())
		groupID := []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))}
		for _, key := range consensus.priKey {
			if !consensus.IsValidatorInCommittee(key.Pub.Bytes) {
				continue
			}
			network, err := consensus.construct(
				msg_pb.MessageType_COMMIT,
				commitPayload,
				&key,
			)
			if err != nil {
				consensus.getLogger().Err(err).Msg("could not create commit message")
				continue
			}
			msgToSend := network.Bytes
			consensus.getLogger().Info().Msg("onNewView === commit")
			consensus.host.SendMessageToGroups(
				groupID,
				p2p.ConstructMessage(msgToSend),
			)
		}
		consensus.switchPhase("onNewView", FBFTCommit, true)
	} else {
		consensus.ResetState()
		consensus.getLogger().Info().Msg("onNewView === announce")
	}
	consensus.getLogger().Info().
		Str("newLeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("new leader changed")
	consensus.consensusTimeout[timeoutConsensus].Start()
	consensus.consensusTimeout[timeoutViewChange].Stop()
}

// ResetViewChangeState resets the view change structure
func (consensus *Consensus) ResetViewChangeState() {
	consensus.getLogger().Info().
		Str("Phase", consensus.phase.String()).
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.current.SetMode(Normal)
	consensus.VC.Reset()
	consensus.Decider.ResetViewChangeVotes()
}
