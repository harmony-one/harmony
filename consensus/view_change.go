package consensus

import (
	"bytes"
	"encoding/binary"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/consensus/timeouts"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

// MaxViewIDDiff limits the received view ID to only 100 further from the current view ID
const MaxViewIDDiff = 100

// State contains current mode and current viewID
type State struct {
	mode   atomic.Value
	viewID atomic.Value
}

// NewState ..
func NewState() State {
	var m, v atomic.Value
	m.Store(Normal)
	v.Store(uint64(0))

	return State{
		mode:   m,
		viewID: v,
	}
}

// Mode return the current node mode
func (pm *State) Mode() Mode {
	return pm.mode.Load().(Mode)
}

// SetMode set the node mode as required
func (pm *State) SetMode(s Mode) {
	pm.mode.Store(s)
}

// ViewID return the current viewchanging id
func (pm *State) ViewID() uint64 {
	return pm.viewID.Load().(uint64)
}

// SetViewID sets the viewchanging id accordingly
func (pm *State) SetViewID(viewID uint64) {
	pm.viewID.Store(viewID)
}

// switchPhase will switch FBFTPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(desired FBFTPhase) {
	utils.Logger().Debug().
		Str("From", consensus.phase.Load().(FBFTPhase).String()).
		Str("To", desired.String()).
		Msg("switching phase")
	consensus.phase.Store(desired)
}

// GetNextLeaderKey uniquely determine who is the leader for given viewID
func (consensus *Consensus) GetNextLeaderKey() *bls.PublicKey {
	wasFound, next := consensus.Decider.NextAfter(consensus.LeaderPubKey)
	if !wasFound {
		utils.Logger().Warn().
			Msg("GetNextLeaderKey: currentLeaderKey not found")
	}
	return next
}

// ResetViewChangeState reset the state for viewchange
func (consensus *Consensus) ResetViewChangeState() {
	utils.Logger().Debug().Msg("[ResetViewChangeState] Resetting view change state")
	consensus.Current.SetMode(Normal)
	consensus.m1Payload = []byte{}
	consensus.bhpSigs = map[uint64]map[string]*bls.Sign{}
	consensus.nilSigs = map[uint64]map[string]*bls.Sign{}
	consensus.viewIDSigs = map[uint64]map[string]*bls.Sign{}
	consensus.bhpBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.nilBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.viewIDBitmap = map[uint64]*bls_cosi.Mask{}
	consensus.Decider.ResetViewChangeVotes()
}

// StartViewChange ..
func (consensus *Consensus) StartViewChange(viewID uint64) {

	if consensus.disableViewChange {
		return
	}
	consensus.Current.SetMode(ViewChanging)
	consensus.Current.SetViewID(viewID)
	consensus.LeaderPubKey = consensus.GetNextLeaderKey()
	diff := int64(viewID - consensus.ViewID())
	duration := time.Duration(diff * diff * int64(timeouts.ViewChangeDuration))
	utils.Logger().Info().
		Uint64("ViewChangingID", viewID).
		Dur("timeoutDuration", duration).
		Msg("[startViewChange]")

	for i, key := range consensus.PubKey.PublicKey {
		msgToSend := consensus.constructViewChangeMessage(key, consensus.priKey.PrivateKey[i])
		consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
			p2p.ConstructMessage(msgToSend),
		)
	}

	consensus.Timeouts.ViewChange.SetDuration(duration)
	consensus.Timeouts.ViewChange.Start(consensus.ViewID())

	utils.Logger().Debug().
		Uint64("ViewChangingID", consensus.Current.ViewID()).
		Msg("[startViewChange] start view change timer")
}

func (consensus *Consensus) onViewChange(msg *msg_pb.Message) error {
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		utils.Logger().Warn().Msg("[onViewChange] Unable To Parse Viewchange Message")
		return err
	}
	// if not leader, noop
	newLeaderKey := recvMsg.LeaderPubkey
	newLeaderPriKey, err := consensus.GetLeaderPrivateKey(newLeaderKey)
	if err != nil {
		return err
	}

	if consensus.Decider.IsQuorumAchieved(quorum.ViewChange) {
		utils.Logger().Debug().
			Int64("have", consensus.Decider.SignersCount(quorum.ViewChange)).
			Int64("need", consensus.Decider.TwoThirdsSignersCount()).
			Msg("[onViewChange] Received Enough View Change Messages")
		return nil
	}

	if !consensus.onViewChangeSanityCheck(recvMsg) {
		return nil
	}

	senderKey := recvMsg.SenderPubkey

	consensus.Locks.VC.Lock()
	defer consensus.Locks.VC.Unlock()

	// update the dictionary key if the viewID is first time received
	consensus.addViewIDKeyIfNotExist(recvMsg.ViewID)

	// TODO: remove NIL type message
	// add self m1 or m2 type message signature and bitmap
	_, ok1 := consensus.nilSigs[recvMsg.ViewID][newLeaderKey.SerializeToHexStr()]
	_, ok2 := consensus.bhpSigs[recvMsg.ViewID][newLeaderKey.SerializeToHexStr()]
	if !(ok1 || ok2) {
		// add own signature for newview message
		preparedMsgs := consensus.FBFTLog.GetMessagesByTypeSeq(
			msg_pb.MessageType_PREPARED, recvMsg.BlockNum,
		)
		preparedMsg := consensus.FBFTLog.FindMessageByMaxViewID(preparedMsgs)
		if preparedMsg == nil {
			utils.Logger().Debug().Msg("[onViewChange] add my M2(NIL) type messaage")
			for i, key := range consensus.PubKey.PublicKey {
				priKey := consensus.priKey.PrivateKey[i]
				consensus.nilSigs[recvMsg.ViewID][key.SerializeToHexStr()] = priKey.SignHash(NIL)
				consensus.nilBitmap[recvMsg.ViewID].SetKey(key, true)
			}
		} else {
			utils.Logger().Debug().Msg("[onViewChange] add my M1 type messaage")
			msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
			for i, key := range consensus.PubKey.PublicKey {
				priKey := consensus.priKey.PrivateKey[i]
				consensus.bhpSigs[recvMsg.ViewID][key.SerializeToHexStr()] = priKey.SignHash(msgToSign)
				consensus.bhpBitmap[recvMsg.ViewID].SetKey(key, true)
			}
		}
	}
	// add self m3 type message signature and bitmap
	_, ok3 := consensus.viewIDSigs[recvMsg.ViewID][newLeaderKey.SerializeToHexStr()]
	if !ok3 {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)
		for i, key := range consensus.PubKey.PublicKey {
			priKey := consensus.priKey.PrivateKey[i]
			consensus.viewIDSigs[recvMsg.ViewID][key.SerializeToHexStr()] = priKey.SignHash(
				viewIDBytes,
			)
			consensus.viewIDBitmap[recvMsg.ViewID].SetKey(key, true)
		}
	}

	// m2 type message
	if len(recvMsg.Payload) == 0 {
		_, ok := consensus.nilSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()]
		if ok {
			utils.Logger().Debug().
				Msg("[onViewChange] Already Received M2 message from validator")
			return nil
		}

		if !recvMsg.ViewchangeSig.VerifyHash(senderKey, NIL) {
			utils.Logger().Warn().Msg("[onViewChange] Failed To Verify Signature For M2 Type Viewchange Message")
			return nil
		}

		utils.Logger().Debug().
			Msg("[onViewChange] Add M2 (NIL) type message")
		consensus.nilSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.nilBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	} else { // m1 type message
		_, ok := consensus.bhpSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()]
		if ok {
			utils.Logger().Debug().
				Msg("[onViewChange] Already Received M1 Message From the Validator")
			return nil
		}
		if !recvMsg.ViewchangeSig.VerifyHash(recvMsg.SenderPubkey, recvMsg.Payload) {
			utils.Logger().Warn().
				Msg("[onViewChange] Failed to Verify Signature for M1 Type Viewchange Message")
			return nil
		}

		// first time receive m1 type message, need verify validity of prepared message
		if len(consensus.m1Payload) == 0 || !bytes.Equal(consensus.m1Payload, recvMsg.Payload) {
			if len(recvMsg.Payload) <= 32 {
				utils.Logger().Debug().
					Int("len", len(recvMsg.Payload)).
					Msg("[onViewChange] M1 RecvMsg Payload Not Enough Length")
				return nil
			}
			blockHash := recvMsg.Payload[:32]
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("[onViewChange] M1 RecvMsg Payload Read Error")
				return err
			}

			if !consensus.Decider.IsQuorumAchievedByMask(mask) {
				utils.Logger().Warn().
					Msgf("[onViewChange] Quorum Not achieved")
				return nil
			}

			// Verify the multi-sig for prepare phase
			if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
				utils.Logger().Warn().
					Hex("blockHash", blockHash).
					Msg("[onViewChange] failed to verify multi signature for m1 prepared payload")
				return errors.New("failed to verify multi signature for m1 prepared payload")
			}

			// if m1Payload is empty, we just add one
			if len(consensus.m1Payload) == 0 {
				consensus.m1Payload = append(recvMsg.Payload[:0:0], recvMsg.Payload...)
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
				utils.Logger().Info().Msg("[onViewChange] New Leader Prepared Message Added")
				consensus.FBFTLog.AddMessage(&preparedMsg)
			}
		}
		utils.Logger().Debug().
			Msg("[onViewChange] Add M1 (prepared) type message")
		consensus.bhpSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.bhpBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	}

	// check and add viewID (m3 type) message signature
	if _, ok := consensus.viewIDSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()]; ok {
		utils.Logger().Debug().
			Msg("[onViewChange] Already Received M3(ViewID) message from the validator")
		return nil
	}
	viewIDHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDHash, recvMsg.ViewID)
	if !recvMsg.ViewidSig.VerifyHash(recvMsg.SenderPubkey, viewIDHash) {
		utils.Logger().Warn().
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onViewChange] Failed to Verify M3 Message Signature")
		return errors.New("Failed to Verify M3 Message Signature")
	}
	utils.Logger().Debug().
		Msg("[onViewChange] Add M3 (ViewID) type message")

	consensus.viewIDSigs[recvMsg.ViewID][senderKey.SerializeToHexStr()] = recvMsg.ViewidSig
	// Set the bitmap indicating that this validator signed.
	consensus.viewIDBitmap[recvMsg.ViewID].SetKey(recvMsg.SenderPubkey, true)
	utils.Logger().Debug().
		Int("have", len(consensus.viewIDSigs[recvMsg.ViewID])).
		Int64("needed", consensus.Decider.TwoThirdsSignersCount()).
		Msg("[onViewChange]")

	// received enough view change messages, change state to normal consensus
	if consensus.Decider.IsQuorumAchievedByMask(consensus.viewIDBitmap[recvMsg.ViewID]) {
		consensus.Current.SetMode(Normal)
		consensus.LeaderPubKey = newLeaderKey
		consensus.ResetState()
		if len(consensus.m1Payload) == 0 {
			// TODO(Chao): explain why ReadySignal is sent only in this case but not the other case.
			go func() {
				consensus.ProposalNewBlock <- struct{}{}
			}()
		} else {
			consensus.switchPhase(FBFTCommit)

			consensus.SetBlockHash(common.BytesToHash(consensus.m1Payload[:32]))

			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)

			if err != nil {
				utils.Logger().Error().Err(err).
					Msg("[onViewChange] ReadSignatureBitmapPayload Fail")
				return err
			}

			consensus.aggregatedPrepareSig = aggSig
			consensus.prepareBitmap = mask
			// Leader sign and add commit message

			commitPayload := signature.ConstructCommitPayload(
				consensus.ChainReader,
				new(big.Int).SetUint64(consensus.Epoch()),
				consensus.BlockHash().Bytes(),
				consensus.BlockNum(),
				recvMsg.ViewID,
			)

			for i, key := range consensus.PubKey.PublicKey {
				priKey := consensus.priKey.PrivateKey[i]
				if _, err := consensus.Decider.SubmitVote(
					quorum.Commit,
					key,
					priKey.SignHash(commitPayload),
					consensus.BlockHash(),
					consensus.BlockNum(),
					recvMsg.ViewID,
				); err != nil {
					utils.Logger().Debug().Msg("submit vote on viewchange commit failed")
					return err
				}

				if err := consensus.commitBitmap.SetKey(key, true); err != nil {
					utils.Logger().Debug().
						Msg("[OnViewChange] New Leader commit bitmap set failed")
					return err
				}
			}
		}

		consensus.Current.SetViewID(recvMsg.ViewID)
		msgToSend := consensus.constructNewViewMessage(
			recvMsg.ViewID, newLeaderKey, newLeaderPriKey,
		)

		utils.Logger().Warn().
			Int("payloadSize", len(consensus.m1Payload)).
			Hex("M1Payload", consensus.m1Payload).
			Msg("[onViewChange] Sent NewView Message")
		if err := consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
			p2p.ConstructMessage(msgToSend),
		); err != nil {
			utils.Logger().Err(err).
				Msg("could not send out the NEWVIEW message")
			return err
		}

		consensus.SetViewID(recvMsg.ViewID)
		consensus.ResetViewChangeState()
		consensus.Timeouts.Consensus.Start(consensus.BlockNum())
		utils.Logger().Debug().
			Uint64("viewID", consensus.ViewID()).
			Uint64("block", consensus.BlockNum()).
			Msg("[onViewChange] I am the New Leader")
	}
	return nil
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onNewView(msg *msg_pb.Message) error {
	utils.Logger().Debug().Msg("[onNewView] Received NewView Message")
	recvMsg, err := consensus.ParseNewViewMessage(msg)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[onNewView] Unable to Parse NewView Message")
		return err
	}

	if !consensus.onNewViewSanityCheck(recvMsg) {
		return nil
	}

	senderKey := recvMsg.SenderPubkey
	consensus.Locks.VC.Lock()
	defer consensus.Locks.VC.Unlock()

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		utils.Logger().Error().Msg("[onNewView] M3AggSig or M3Bitmap is nil")
		return nil
	}
	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)

	if !consensus.Decider.IsQuorumAchievedByMask(m3Mask) {
		utils.Logger().Warn().
			Msgf("[onNewView] Quorum Not achieved")
		return nil
	}

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		utils.Logger().Warn().
			Hex("m3Mask", m3Mask.Bitmap).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onNewView] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
		return errors.New("Unable to Verify Aggregated Signature of M3 (ViewID) payload")
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		utils.Logger().Debug().Msg("[onNewView] M2AggSig (NIL) is Not Empty")
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			utils.Logger().Warn().
				Msg("[onNewView] Unable to Verify Aggregated Signature of M2 (NIL) payload")
			return errors.New("Unable to Verify Aggregated Signature of M2 (NIL) payload")
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	if m2Mask == nil || m2Mask.Bitmap == nil ||
		(m2Mask != nil && m2Mask.Bitmap != nil &&
			utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		if len(recvMsg.Payload) <= 32 {
			utils.Logger().Debug().
				Msg("[onNewView] M1 (prepared) Type Payload Not Have Enough Length")
			return nil
		}
		// m1 is not empty, check it's valid
		blockHash := recvMsg.Payload[:32]
		aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
		if err != nil {
			utils.Logger().Error().Err(err).
				Msg("[onNewView] ReadSignatureBitmapPayload Failed")
			return err
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash) {
			utils.Logger().Warn().
				Msg("[onNewView] Failed to Verify Signature for M1 (prepare) message")
			return nil
		}

		consensus.SetBlockHash(common.BytesToHash(blockHash))
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
	}

	// newView message verified success, override my state
	consensus.SetViewID(recvMsg.ViewID)
	consensus.Current.SetViewID(recvMsg.ViewID)
	consensus.LeaderPubKey = senderKey
	consensus.ResetViewChangeState()

	// change view and leaderKey to keep in sync with network
	if consensus.BlockNum() != recvMsg.BlockNum {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onNewView] New Leader Changed")
		return nil
	}

	// NewView message is verified, change state to normal consensus
	// TODO: check magic number 32
	if len(recvMsg.Payload) > 32 {
		// Construct and send the commit message
		commitPayload := signature.ConstructCommitPayload(
			consensus.ChainReader,
			new(big.Int).SetUint64(consensus.Epoch()),
			consensus.BlockHash().Bytes(),
			consensus.BlockNum(),
			consensus.ViewID(),
		)

		groupID := []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		}
		for i, key := range consensus.PubKey.PublicKey {
			network, err := consensus.construct(
				msg_pb.MessageType_COMMIT,
				commitPayload,
				key, consensus.priKey.PrivateKey[i],
			)
			if err != nil {
				utils.Logger().Err(err).Msg("could not create commit message")
				return err
			}
			msgToSend := network.Bytes
			utils.Logger().Info().Msg("onNewView === commit")
			if err := consensus.host.SendMessageToGroups(
				groupID,
				p2p.ConstructMessage(msgToSend),
			); err != nil {
				return err
			}
		}
		utils.Logger().Debug().
			Str("To", FBFTCommit.String()).
			Msg("[OnViewChange] Switching phase")
		consensus.switchPhase(FBFTCommit)
	} else {
		consensus.ResetState()
		utils.Logger().Info().Msg("onNewView === announce")
	}
	utils.Logger().Debug().
		Msg("validator start consensus timer and stop view change timer")
	consensus.Timeouts.Consensus.Start(consensus.BlockNum())
	return nil
}
