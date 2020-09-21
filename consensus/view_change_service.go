package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"sync"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// MaxViewIDDiff limits the received view ID to only 100 further from the current view ID
const MaxViewIDDiff = 100

// TODO: view change message retry to accelerate the view change process

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

// switchPhase will switch FBFTPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(subject string, desired FBFTPhase, override bool) {
	consensus.getLogger().Info().
		Str("from:", consensus.phase.String()).
		Str("to:", desired.String()).
		Bool("override:", override).
		Msg(subject)

	if override {
		consensus.phase = desired
		return
	}

	var nextPhase FBFTPhase
	switch consensus.phase {
	case FBFTAnnounce:
		nextPhase = FBFTPrepare
	case FBFTPrepare:
		nextPhase = FBFTCommit
	case FBFTCommit:
		nextPhase = FBFTAnnounce
	}
	if nextPhase == desired {
		consensus.phase = nextPhase
	}
}

// GetNextLeaderKey uniquely determine who is the leader for given viewID
func (consensus *Consensus) GetNextLeaderKey() *bls.PublicKeyWrapper {
	wasFound, next := consensus.Decider.NextAfter(consensus.LeaderPubKey)
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

// startViewChange send a  new view change
func (consensus *Consensus) startViewChange(viewID uint64) {
	if consensus.disableViewChange {
		return
	}
	consensus.consensusTimeout[timeoutConsensus].Stop()
	consensus.consensusTimeout[timeoutBootstrap].Stop()
	consensus.current.SetMode(ViewChanging)
	consensus.SetViewChangingID(viewID)
	consensus.LeaderPubKey = consensus.GetNextLeaderKey()

	duration := consensus.current.GetViewChangeDuraion()
	consensus.getLogger().Warn().
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

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	consensus.consensusTimeout[timeoutViewChange].Start()
}

func (consensus *Consensus) onViewChange(msg *msg_pb.Message) {
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		consensus.getLogger().Warn().Msg("[onViewChange] Unable To Parse Viewchange Message")
		return
	}
	// if not leader, noop
	// TODO: count number of view chagne messages and set lower viewID
	// based on PBFT 4.5.2
	newLeaderKey := recvMsg.LeaderPubkey
	newLeaderPriKey, err := consensus.GetLeaderPrivateKey(newLeaderKey.Object)
	if err != nil {
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
		consensus.getLogger().Error().Err(err).Msg("Parse View Change Message Error")
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
					consensus.getLogger().Warn().
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
					consensus.getLogger().Warn().Msg("submit vote on viewchange commit failed")
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

		consensus.SetCurBlockViewID(recvMsg.ViewID)
		consensus.ResetViewChangeState()
		consensus.consensusTimeout[timeoutViewChange].Stop()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Info().Str("myKey", newLeaderKey.Bytes.Hex()).Msg("[onViewChange] I am the New Leader")
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onNewView(msg *msg_pb.Message) {
	consensus.getLogger().Info().Msg("[onNewView] Received NewView Message")
	members := consensus.Decider.Participants()
	recvMsg, err := ParseNewViewMessage(msg, members)
	if err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[onNewView] Unable to Parse NewView Message")
		return
	}

	if !consensus.onNewViewSanityCheck(recvMsg) {
		return
	}

	senderKey := recvMsg.SenderPubkey

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		consensus.getLogger().Error().Msg("[onNewView] M3AggSig or M3Bitmap is nil")
		return
	}
	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)

	if !consensus.Decider.IsQuorumAchievedByMask(m3Mask) {
		consensus.getLogger().Warn().
			Msgf("[onNewView] Quorum Not achieved")
		return
	}

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		consensus.getLogger().Warn().
			Str("m3Sig", m3Sig.SerializeToHexStr()).
			Hex("m3Mask", m3Mask.Bitmap).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onNewView] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
		return
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		consensus.getLogger().Info().Msg("[onNewView] M2AggSig (NIL) is Not Empty")
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			consensus.getLogger().Warn().
				Msg("[onNewView] Unable to Verify Aggregated Signature of M2 (NIL) payload")
			return
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	preparedBlock := &types.Block{}
	hasBlock := false
	if len(recvMsg.Payload) != 0 && len(recvMsg.Block) != 0 {
		if err := rlp.DecodeBytes(recvMsg.Block, preparedBlock); err != nil {
			consensus.getLogger().Warn().
				Err(err).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[onNewView] Unparseable prepared block data")
			return
		}

		blockHash := recvMsg.Payload[:32]
		preparedBlockHash := preparedBlock.Hash()
		if !bytes.Equal(preparedBlockHash[:], blockHash) {
			consensus.getLogger().Warn().
				Err(err).
				Str("blockHash", preparedBlock.Hash().Hex()).
				Str("payloadBlockHash", hex.EncodeToString(blockHash)).
				Msg("[onNewView] Prepared block hash doesn't match msg block hash.")
			return
		}
		hasBlock = true
		if consensus.BlockVerifier(preparedBlock); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[onNewView] Prepared block verification failed")
			return
		}
	}

	if m2Mask == nil || m2Mask.Bitmap == nil ||
		(m2Mask != nil && m2Mask.Bitmap != nil &&
			utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		if len(recvMsg.Payload) <= 32 {
			consensus.getLogger().Info().
				Msg("[onNewView] M1 (prepared) Type Payload Not Have Enough Length")
			return
		}
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
		preparedMsg.SenderPubkey = senderKey
		consensus.FBFTLog.AddMessage(&preparedMsg)

		if hasBlock {
			consensus.FBFTLog.AddBlock(preparedBlock)
		}
	}

	// newView message verified success, override my state
	consensus.SetViewIDs(recvMsg.ViewID)
	consensus.LeaderPubKey = senderKey
	consensus.ResetViewChangeState()

	// change view and leaderKey to keep in sync with network
	if consensus.blockNum != recvMsg.BlockNum {
		consensus.getLogger().Info().
			Str("newLeaderKey", consensus.LeaderPubKey.Bytes.Hex()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onNewView] New Leader Changed")
		return
	}

	// NewView message is verified, change state to normal consensus
	if hasBlock {
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
				return
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
	consensus.getLogger().Info().
		Msg("validator start consensus timer and stop view change timer")
	consensus.consensusTimeout[timeoutConsensus].Start()
	consensus.consensusTimeout[timeoutViewChange].Stop()
}

func (consensus *Consensus) ResetViewChangeState() {
	consensus.getLogger().Debug().
		Str("Phase", consensus.phase.String()).
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.current.SetMode(Normal)
	consensus.VC.Reset()
	consensus.Decider.ResetViewChangeVotes()
}
