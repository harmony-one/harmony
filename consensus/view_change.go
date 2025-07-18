package consensus

import (
	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// MaxViewIDDiff limits the received view ID to only 249 further from the current view ID
const MaxViewIDDiff = 249

func createTimeout() map[TimeoutType]*utils.Timeout {
	timeouts := make(map[TimeoutType]*utils.Timeout)
	timeouts[timeoutConsensus] = utils.NewTimeout(phaseDuration)
	timeouts[timeoutViewChange] = utils.NewTimeout(viewChangeDuration)
	timeouts[timeoutBootstrap] = utils.NewTimeout(bootstrapDuration)
	return timeouts
}

// startViewChange start the view change process
func (consensus *Consensus) startViewChange() {
	if consensus.isBackup {
		return
	}

	consensus.consensusTimeout[timeoutConsensus].Stop()
	consensus.consensusTimeout[timeoutBootstrap].Stop()
	consensus.current.SetMode(ViewChanging)
	curHeader := consensus.Blockchain().CurrentHeader()
	nextViewID, duration := consensus.current.getNextViewID(curHeader)
	consensus.setViewChangingID(nextViewID)
	epoch := curHeader.Epoch()
	ss, err := consensus.Blockchain().ReadShardState(epoch)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("Failed to read shard state")
		return
	}
	committee, err := ss.FindCommitteeByID(consensus.ShardID)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("Failed to find committee")
		return
	}
	// TODO: set the Leader PubKey to the next leader for view change
	// this is dangerous as the leader change is not succeeded yet
	// we use it this way as in many code we validate the messages
	// aganist the consensus.LeaderPubKey variable.
	// Ideally, we shall use another variable to keep track of the
	// leader pubkey in viewchange mode
	consensus.setLeaderPubKey(
		consensus.current.getNextLeaderKey(consensus.Blockchain(), consensus.decider, nextViewID, committee))

	consensus.getLogger().Warn().
		Uint64("nextViewID", nextViewID).
		Uint64("viewChangingID", consensus.getViewChangingID()).
		Dur("timeoutDuration", duration).
		Str("NextLeader", consensus.getLeaderPubKey().Bytes.Hex()).
		Msg("[startViewChange]")
	consensusVCCounterVec.With(prometheus.Labels{"viewchange": "started"}).Inc()

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	defer consensus.consensusTimeout[timeoutViewChange].Start("start view change")

	// update the dictionary key if the viewID is first time received
	members := consensus.decider.Participants()
	consensus.vc.AddViewIDKeyIfNotExist(nextViewID, members)

	// init my own payload
	if err := consensus.vc.InitPayload(
		consensus.fBFTLog,
		nextViewID,
		consensus.getBlockNum(),
		consensus.priKey,
		members,
		consensus.verifyBlock,
	); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[startViewChange] Init Payload Error")
	}

	// for view change, send separate view change per public key
	// do not do multi-sign of view change message
	for _, key := range consensus.priKey {
		if !consensus.isValidatorInCommittee(key.Pub.Bytes) {
			continue
		}
		msgToSend := consensus.constructViewChangeMessage(&key)
		if err := consensus.msgSender.SendWithRetry(
			consensus.getBlockNum(),
			msg_pb.MessageType_VIEWCHANGE,
			[]nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
			p2p.ConstructMessage(msgToSend),
		); err != nil {
			consensus.getLogger().Err(err).
				Msg("[startViewChange] could not send out the ViewChange message")
		}
	}
}

// startNewView stops the current view change
func (consensus *Consensus) startNewView(viewID uint64, newLeaderPriKey *bls.PrivateKeyWrapper, reset bool) error {
	if !consensus.isViewChangingMode() {
		return errors.New("not in view changing mode anymore")
	}

	msgToSend := consensus.constructNewViewMessage(
		viewID, newLeaderPriKey,
	)
	if msgToSend == nil {
		return errors.New("failed to construct NewView message")
	}

	if err := consensus.msgSender.SendWithRetry(
		consensus.getBlockNum(),
		msg_pb.MessageType_NEWVIEW,
		[]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
		p2p.ConstructMessage(msgToSend),
	); err != nil {
		return errors.New("failed to send out the NewView message")
	}
	consensus.getLogger().Info().
		Str("myKey", newLeaderPriKey.Pub.Bytes.Hex()).
		Hex("M1Payload", consensus.vc.GetM1Payload()).
		Msg("[startNewView] Sent NewView Messge")

	consensus.msgSender.StopRetry(msg_pb.MessageType_VIEWCHANGE)

	consensus.current.SetMode(Normal)
	consensus.consensusTimeout[timeoutViewChange].Stop()
	consensus.setViewIDs(viewID)
	consensus.resetViewChangeState()
	consensus.consensusTimeout[timeoutConsensus].Start("starting new view")

	consensus.getLogger().Info().
		Uint64("viewID", viewID).
		Str("myKey", newLeaderPriKey.Pub.Bytes.Hex()).
		Msg("[startNewView] viewChange stopped. I am the New Leader")

	// TODO: consider make ResetState unified and only called in one place like finalizeCommit()
	if reset {
		consensus.resetState()
	}
	consensus.setLeaderPubKey(newLeaderPriKey.Pub)

	return nil
}

// onViewChange is called when the view change message is received.
func (consensus *Consensus) onViewChange(recvMsg *FBFTMessage) {
	consensus.getLogger().Debug().
		Uint64("viewID", recvMsg.ViewID).
		Uint64("blockNum", recvMsg.BlockNum).
		Interface("SenderPubkeys", recvMsg.SenderPubkeys).
		Msg("[onViewChange] Received ViewChange Message")

	// if not leader, noop
	newLeaderKey := recvMsg.LeaderPubkey
	newLeaderPriKey, err := consensus.getLeaderPrivateKey(newLeaderKey.Object)
	if err != nil {
		consensus.getLogger().Debug().
			Err(err).
			Interface("SenderPubkeys", recvMsg.SenderPubkeys).
			Str("NextLeader", recvMsg.LeaderPubkey.Bytes.Hex()).
			Str("myBLSPubKey", consensus.priKey.GetPublicKeys().SerializeToHexStr()).
			Msg("[onViewChange] I am not the Leader")
		return
	}

	if consensus.decider.IsQuorumAchievedByMask(consensus.vc.GetViewIDBitmap(recvMsg.ViewID)) {
		consensus.getLogger().Info().
			Int64("have", consensus.decider.SignersCount(quorum.ViewChange)).
			Int64("need", consensus.decider.TwoThirdsSignersCount()).
			Interface("SenderPubkeys", recvMsg.SenderPubkeys).
			Str("newLeaderKey", newLeaderKey.Bytes.Hex()).
			Msg("[onViewChange] Received Enough View Change Messages")
		return
	}

	if !consensus.onViewChangeSanityCheck(recvMsg) {
		return
	}

	// already checked the length of SenderPubkeys in onViewChangeSanityCheck
	senderKey := recvMsg.SenderPubkeys[0]

	// update the dictionary key if the viewID is first time received
	members := consensus.decider.Participants()
	consensus.vc.AddViewIDKeyIfNotExist(recvMsg.ViewID, members)

	// do it once only per viewID/Leader
	if err := consensus.vc.InitPayload(
		consensus.fBFTLog,
		recvMsg.ViewID,
		recvMsg.BlockNum,
		consensus.priKey,
		members,
		consensus.verifyBlock,
	); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[onViewChange] Init Payload Error")
		return
	}

	err = consensus.vc.ProcessViewChangeMsg(consensus.fBFTLog, consensus.decider, recvMsg, consensus.verifyBlock)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Uint64("viewID", recvMsg.ViewID).
			Uint64("blockNum", recvMsg.BlockNum).
			Str("msgSender", senderKey.Bytes.Hex()).
			Msg("[onViewChange] process View Change message error")
		return
	}
	consensus.sendLastSignPower()

	// received enough view change messages, change state to normal consensus
	if consensus.decider.IsQuorumAchievedByMask(consensus.vc.GetViewIDBitmap(recvMsg.ViewID)) && consensus.isViewChangingMode() {
		// no previous prepared message, go straight to normal mode
		// and start proposing new block
		if consensus.vc.IsM1PayloadEmpty() {
			if err := consensus.startNewView(recvMsg.ViewID, newLeaderPriKey, true); err != nil {
				consensus.getLogger().Error().Err(err).Msg("[onViewChange] startNewView failed")
				return
			}
			go consensus.ReadySignal(NewProposal(SyncProposal, consensus.Blockchain().CurrentHeader().NumberU64()+1), "onViewChange", "quorum is achieved by mask and is view change mode and M1 payload is empty")
			return
		}

		payload := consensus.vc.GetM1Payload()
		if err := consensus.selfCommit(payload); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[onViewChange] self commit failed")
			return
		}
		if err := consensus.startNewView(recvMsg.ViewID, newLeaderPriKey, false); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[onViewChange] startNewView failed")
			return
		}
	}
}

// onNewView is called when validators received newView message from the new leader
// the validator needs to check the m3bitmap to see if the quorum is reached
// If the new view message contains payload (block), and at least one m1 message was
// collected by the new leader (m3count > m2count), the validator will create a new
// prepared message from the payload and commit it to the block
// Or the validator will enter announce phase to wait for the new block proposed
// from the new leader
func (consensus *Consensus) onNewView(recvMsg *FBFTMessage) {
	consensus.getLogger().Info().
		Uint64("viewID", recvMsg.ViewID).
		Uint64("blockNum", recvMsg.BlockNum).
		Interface("SenderPubkeys", recvMsg.SenderPubkeys).
		Msg("[onNewView] Received NewView Message")

	// change view and leaderKey to keep in sync with network
	if consensus.getBlockNum() != recvMsg.BlockNum {
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("myBlockNum", consensus.getBlockNum()).
			Msg("[onNewView] Invalid block number")
		return
	}

	if !recvMsg.HasSingleSender() {
		consensus.getLogger().Error().Msg("[onNewView] multiple signers in view change message.")
		return
	}
	senderKey := recvMsg.SenderPubkeys[0]

	if !consensus.onNewViewSanityCheck(recvMsg) {
		return
	}

	preparedBlock, err := consensus.vc.VerifyNewViewMsg(recvMsg, consensus.verifyBlock)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onNewView] Verify New View Msg Failed")
		return
	}

	m3Mask := recvMsg.M3Bitmap
	if !consensus.decider.IsQuorumAchievedByMask(m3Mask) {
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
		aggSig, mask, err := readSignatureBitmapPayload(recvMsg.Payload, 32, consensus.decider.Participants())
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
		copy(consensus.current.blockHash[:], blockHash)
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

		preparedMsg.SenderPubkeys = []*bls.PublicKeyWrapper{senderKey}
		consensus.fBFTLog.AddVerifiedMessage(&preparedMsg)

		if preparedBlock != nil {
			consensus.fBFTLog.AddBlock(preparedBlock)
		}
	}

	if !consensus.isViewChangingMode() {
		consensus.getLogger().Info().Msg("Not in ViewChanging Mode.")
		return
	}

	consensus.consensusTimeout[timeoutViewChange].Stop()

	// newView message verified success, override my state
	consensus.setViewIDs(recvMsg.ViewID)
	consensus.setLeaderPubKey(senderKey)
	consensus.resetViewChangeState()

	consensus.msgSender.StopRetry(msg_pb.MessageType_VIEWCHANGE)

	// NewView message is verified, change state to normal consensus
	if preparedBlock != nil {
		consensus.sendCommitMessages(preparedBlock)
		consensus.switchPhase("onNewView", FBFTCommit)
	} else {
		consensus.resetState()
		consensus.getLogger().Info().Msg("onNewView === announce")
	}
	consensus.getLogger().Info().
		Str("newLeaderKey", consensus.getLeaderPubKey().Bytes.Hex()).
		Msg("new leader changed")
	consensus.consensusTimeout[timeoutConsensus].Start("starting on new view")
	consensusVCCounterVec.With(prometheus.Labels{"viewchange": "finished"}).Inc()
}

// ResetViewChangeState resets the view change structure
func (consensus *Consensus) ResetViewChangeState() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.resetViewChangeState()
}

// ResetViewChangeState resets the view change structure
func (consensus *Consensus) resetViewChangeState() {
	consensus.getLogger().Info().
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.current.SetMode(Normal)
	consensus.vc.Reset()
	consensus.decider.ResetViewChangeVotes()
}
