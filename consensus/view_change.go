package consensus

import (
	"math/big"
	"time"

	"github.com/harmony-one/harmony/internal/chain"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// MaxViewIDDiff limits the received view ID to only 249 further from the current view ID
const MaxViewIDDiff = 249

// State contains current mode and current viewID
type State struct {
	mode Mode

	// current view id in normal mode
	// it changes per successful consensus
	blockViewID uint64

	// view changing id is used during view change mode
	// it is the next view id
	viewChangingID uint64

	isBackup bool
}

// Mode return the current node mode
func (pm *State) Mode() Mode {
	return pm.mode
}

// SetMode set the node mode as required
func (pm *State) SetMode(s Mode) {
	if s == Normal && pm.isBackup {
		s = NormalBackup
	}

	pm.mode = s
}

// GetCurBlockViewID return the current view id
func (pm *State) GetCurBlockViewID() uint64 {
	return pm.blockViewID
}

// SetCurBlockViewID sets the current view id
func (pm *State) SetCurBlockViewID(viewID uint64) uint64 {
	pm.blockViewID = viewID
	return pm.blockViewID
}

// GetViewChangingID return the current view changing id
// It is meaningful during view change mode
func (pm *State) GetViewChangingID() uint64 {
	return pm.viewChangingID
}

// SetViewChangingID set the current view changing id
// It is meaningful during view change mode
func (pm *State) SetViewChangingID(id uint64) {
	pm.viewChangingID = id
}

// GetViewChangeDuraion return the duration of the current view change
// It increase in the power of difference betweeen view changing ID and current view ID
func (pm *State) GetViewChangeDuraion() time.Duration {
	diff := int64(pm.viewChangingID - pm.blockViewID)
	return time.Duration(diff * diff * int64(viewChangeDuration))
}

func (pm *State) SetIsBackup(isBackup bool) {
	pm.isBackup = isBackup
}

// fallbackNextViewID return the next view ID and duration when there is an exception
// to calculate the time-based viewId
func (consensus *Consensus) fallbackNextViewID() (uint64, time.Duration) {
	diff := int64(consensus.getViewChangingID() + 1 - consensus.getCurBlockViewID())
	if diff <= 0 {
		diff = int64(1)
	}
	consensus.getLogger().Error().
		Int64("diff", diff).
		Msg("[fallbackNextViewID] use legacy viewID algorithm")
	return consensus.getViewChangingID() + 1, time.Duration(diff * diff * int64(viewChangeDuration))
}

// getNextViewID return the next view ID based on the timestamp
// The next view ID is calculated based on the difference of validator's timestamp
// and the block's timestamp. So that it can be deterministic to return the next view ID
// only based on the blockchain block and the validator's current timestamp.
// The next view ID is the single factor used to determine
// the next leader, so it is mod the number of nodes per shard.
// It returns the next viewID and duration of the view change
// The view change duration is a fixed duration now to avoid stuck into offline nodes during
// the view change.
// viewID is only used as the fallback mechansim to determine the nextViewID
func (consensus *Consensus) getNextViewID() (uint64, time.Duration) {
	// handle corner case at first
	if consensus.Blockchain() == nil {
		return consensus.fallbackNextViewID()
	}
	curHeader := consensus.Blockchain().CurrentHeader()
	if curHeader == nil {
		return consensus.fallbackNextViewID()
	}
	blockTimestamp := curHeader.Time().Int64()
	stuckBlockViewID := curHeader.ViewID().Uint64() + 1
	curTimestamp := time.Now().Unix()

	// timestamp messed up in current validator node
	if curTimestamp <= blockTimestamp {
		consensus.getLogger().Error().
			Int64("curTimestamp", curTimestamp).
			Int64("blockTimestamp", blockTimestamp).
			Msg("[getNextViewID] timestamp of block too high")
		return consensus.fallbackNextViewID()
	}
	// diff only increases, since view change timeout is shorter than
	// view change slot now, we want to make sure diff is always greater than 0
	diff := uint64((curTimestamp-blockTimestamp)/viewChangeSlot + 1)
	nextViewID := diff + stuckBlockViewID

	consensus.getLogger().Info().
		Int64("curTimestamp", curTimestamp).
		Int64("blockTimestamp", blockTimestamp).
		Uint64("nextViewID", nextViewID).
		Uint64("stuckBlockViewID", stuckBlockViewID).
		Msg("[getNextViewID]")

	// duration is always the fixed view change duration for synchronous view change
	return nextViewID, viewChangeDuration
}

// getNextLeaderKey uniquely determine who is the leader for given viewID
// It reads the current leader's pubkey based on the blockchain data and returns
// the next leader based on the gap of the viewID of the view change and the last
// know view id of the block.
func (consensus *Consensus) getNextLeaderKey(viewID uint64) *bls.PublicKeyWrapper {
	gap := 1

	cur := consensus.getCurBlockViewID()
	if viewID > cur {
		gap = int(viewID - cur)
	}
	var lastLeaderPubKey *bls.PublicKeyWrapper
	var err error
	blockchain := consensus.Blockchain()
	epoch := big.NewInt(0)
	if blockchain == nil {
		consensus.getLogger().Error().Msg("[getNextLeaderKey] Blockchain is nil. Use consensus.LeaderPubKey")
		lastLeaderPubKey = consensus.LeaderPubKey
	} else {
		curHeader := blockchain.CurrentHeader()
		if curHeader == nil {
			consensus.getLogger().Error().Msg("[getNextLeaderKey] Failed to get current header from blockchain")
			lastLeaderPubKey = consensus.LeaderPubKey
		} else {
			stuckBlockViewID := curHeader.ViewID().Uint64() + 1
			gap = int(viewID - stuckBlockViewID)
			// this is the truth of the leader based on blockchain blocks
			lastLeaderPubKey, err = chain.GetLeaderPubKeyFromCoinbase(blockchain, curHeader)
			if err != nil || lastLeaderPubKey == nil {
				consensus.getLogger().Error().Err(err).
					Msg("[getNextLeaderKey] Unable to get leaderPubKey from coinbase. Set it to consensus.LeaderPubKey")
				lastLeaderPubKey = consensus.LeaderPubKey
			}
			epoch = curHeader.Epoch()
			// viewchange happened at the first block of new epoch
			// use the LeaderPubKey as the base of the next leader
			// as we shouldn't use lastLeader from coinbase as the base.
			// The LeaderPubKey should be updated to the node of index 0 of the committee
			// so, when validator joined the view change process later in the epoch block
			// it can still sync with other validators.
			if curHeader.IsLastBlockInEpoch() {
				consensus.getLogger().Info().Msg("[getNextLeaderKey] view change in the first block of new epoch")
				lastLeaderPubKey = consensus.Decider.FirstParticipant(shard.Schedule.InstanceForEpoch(epoch))
			}
		}
	}
	consensus.getLogger().Info().
		Str("lastLeaderPubKey", lastLeaderPubKey.Bytes.Hex()).
		Str("leaderPubKey", consensus.LeaderPubKey.Bytes.Hex()).
		Int("gap", gap).
		Uint64("newViewID", viewID).
		Uint64("myCurBlockViewID", consensus.getCurBlockViewID()).
		Msg("[getNextLeaderKey] got leaderPubKey from coinbase")
	// wasFound, next := consensus.Decider.NthNext(lastLeaderPubKey, gap)
	// FIXME: rotate leader on harmony nodes only before fully externalization
	var wasFound bool
	var next *bls.PublicKeyWrapper
	if blockchain != nil && blockchain.Config().IsLeaderRotation(epoch) {
		if blockchain.Config().IsLeaderRotationExternalValidatorsAllowed(epoch, consensus.ShardID) {
			wasFound, next = consensus.Decider.NthNext(
				lastLeaderPubKey,
				gap)
		} else {
			wasFound, next = consensus.Decider.NthNextHmy(
				shard.Schedule.InstanceForEpoch(epoch),
				lastLeaderPubKey,
				gap)
		}
	} else {
		wasFound, next = consensus.Decider.NthNextHmy(
			shard.Schedule.InstanceForEpoch(epoch),
			lastLeaderPubKey,
			gap)
	}
	if !wasFound {
		consensus.getLogger().Warn().
			Str("key", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("[getNextLeaderKey] currentLeaderKey not found")
	}
	consensus.getLogger().Info().
		Str("nextLeader", next.Bytes.Hex()).
		Msg("[getNextLeaderKey] next Leader")
	return next
}

func createTimeout() map[TimeoutType]*utils.Timeout {
	timeouts := make(map[TimeoutType]*utils.Timeout)
	timeouts[timeoutConsensus] = utils.NewTimeout(phaseDuration)
	timeouts[timeoutViewChange] = utils.NewTimeout(viewChangeDuration)
	timeouts[timeoutBootstrap] = utils.NewTimeout(bootstrapDuration)
	return timeouts
}

// startViewChange start the view change process
func (consensus *Consensus) startViewChange() {
	if consensus.disableViewChange || consensus.isBackup {
		return
	}

	consensus.consensusTimeout[timeoutConsensus].Stop()
	consensus.consensusTimeout[timeoutBootstrap].Stop()
	consensus.current.SetMode(ViewChanging)
	nextViewID, duration := consensus.getNextViewID()
	consensus.setViewChangingID(nextViewID)
	// TODO: set the Leader PubKey to the next leader for view change
	// this is dangerous as the leader change is not succeeded yet
	// we use it this way as in many code we validate the messages
	// aganist the consensus.LeaderPubKey variable.
	// Ideally, we shall use another variable to keep track of the
	// leader pubkey in viewchange mode
	consensus.LeaderPubKey = consensus.getNextLeaderKey(nextViewID)

	consensus.getLogger().Warn().
		Uint64("nextViewID", nextViewID).
		Uint64("viewChangingID", consensus.getViewChangingID()).
		Dur("timeoutDuration", duration).
		Str("NextLeader", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("[startViewChange]")
	consensusVCCounterVec.With(prometheus.Labels{"viewchange": "started"}).Inc()

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	defer consensus.consensusTimeout[timeoutViewChange].Start()

	// update the dictionary key if the viewID is first time received
	members := consensus.Decider.Participants()
	consensus.vc.AddViewIDKeyIfNotExist(nextViewID, members)

	// init my own payload
	if err := consensus.vc.InitPayload(
		consensus.FBFTLog,
		nextViewID,
		consensus.getBlockNum(),
		consensus.priKey,
		members); err != nil {
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
	consensus.consensusTimeout[timeoutConsensus].Start()

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

	if consensus.Decider.IsQuorumAchievedByMask(consensus.vc.GetViewIDBitmap(recvMsg.ViewID)) {
		consensus.getLogger().Info().
			Int64("have", consensus.Decider.SignersCount(quorum.ViewChange)).
			Int64("need", consensus.Decider.TwoThirdsSignersCount()).
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
	members := consensus.Decider.Participants()
	consensus.vc.AddViewIDKeyIfNotExist(recvMsg.ViewID, members)

	// do it once only per viewID/Leader
	if err := consensus.vc.InitPayload(consensus.FBFTLog,
		recvMsg.ViewID,
		recvMsg.BlockNum,
		consensus.priKey,
		members); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[onViewChange] Init Payload Error")
		return
	}

	err = consensus.vc.ProcessViewChangeMsg(consensus.FBFTLog, consensus.Decider, recvMsg)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Uint64("viewID", recvMsg.ViewID).
			Uint64("blockNum", recvMsg.BlockNum).
			Str("msgSender", senderKey.Bytes.Hex()).
			Msg("[onViewChange] process View Change message error")
		return
	}

	// received enough view change messages, change state to normal consensus
	if consensus.Decider.IsQuorumAchievedByMask(consensus.vc.GetViewIDBitmap(recvMsg.ViewID)) && consensus.isViewChangingMode() {
		// no previous prepared message, go straight to normal mode
		// and start proposing new block
		if consensus.vc.IsM1PayloadEmpty() {
			if err := consensus.startNewView(recvMsg.ViewID, newLeaderPriKey, true); err != nil {
				consensus.getLogger().Error().Err(err).Msg("[onViewChange] startNewView failed")
				return
			}
			go consensus.ReadySignal(SyncProposal)
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

	preparedBlock, err := consensus.vc.VerifyNewViewMsg(recvMsg)
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
		aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 32, consensus.Decider.Participants())
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

		preparedMsg.SenderPubkeys = []*bls.PublicKeyWrapper{senderKey}
		consensus.FBFTLog.AddVerifiedMessage(&preparedMsg)

		if preparedBlock != nil {
			consensus.FBFTLog.AddBlock(preparedBlock)
		}
	}

	if !consensus.isViewChangingMode() {
		consensus.getLogger().Info().Msg("Not in ViewChanging Mode.")
		return
	}

	consensus.consensusTimeout[timeoutViewChange].Stop()

	// newView message verified success, override my state
	consensus.setViewIDs(recvMsg.ViewID)
	consensus.LeaderPubKey = senderKey
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
		Str("newLeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
		Msg("new leader changed")
	consensus.consensusTimeout[timeoutConsensus].Start()
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
		Str("Phase", consensus.phase.String()).
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.current.SetMode(Normal)
	consensus.vc.Reset()
	consensus.Decider.ResetViewChangeVotes()
}
