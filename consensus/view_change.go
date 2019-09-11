package consensus

import (
	"bytes"
	"encoding/binary"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

// PbftPhase  PBFT phases: pre-prepare, prepare and commit
type PbftPhase int

// Enum for PbftPhase
const (
	Announce PbftPhase = iota
	Prepare
	Commit
)

// Mode determines whether a node is in normal or viewchanging mode
type Mode int

// Enum for node Mode
const (
	Normal Mode = iota
	ViewChanging
	Syncing
	Listening
)

// PbftMode contains mode and viewID of viewchanging
type PbftMode struct {
	mode   Mode
	viewID uint64
	mux    sync.Mutex
}

// Mode return the current node mode
func (pm *PbftMode) Mode() Mode {
	return pm.mode
}

//String print mode string
func (mode Mode) String() string {
	if mode == Normal {
		return "Normal"
	} else if mode == ViewChanging {
		return "ViewChanging"
	} else if mode == Syncing {
		return "Syncing"
	} else if mode == Listening {
		return "Listening"
	}
	return "Unknown"
}

// String print phase string
func (phase PbftPhase) String() string {
	if phase == Announce {
		return "Announce"
	} else if phase == Prepare {
		return "Prepare"
	} else if phase == Commit {
		return "Commit"
	}
	return "Unknown"
}

// SetMode set the node mode as required
func (pm *PbftMode) SetMode(m Mode) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.mode = m
}

// ViewID return the current viewchanging id
func (pm *PbftMode) ViewID() uint64 {
	return pm.viewID
}

// SetViewID sets the viewchanging id accordingly
func (pm *PbftMode) SetViewID(viewID uint64) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.viewID = viewID
}

// GetViewID returns the current viewchange viewID
func (pm *PbftMode) GetViewID() uint64 {
	return pm.viewID
}

// switchPhase will switch PbftPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(desirePhase PbftPhase, override bool) {
	if override {
		consensus.phase = desirePhase
		return
	}

	var nextPhase PbftPhase
	switch consensus.phase {
	case Announce:
		nextPhase = Prepare
	case Prepare:
		nextPhase = Commit
	case Commit:
		nextPhase = Announce
	}
	if nextPhase == desirePhase {
		consensus.phase = nextPhase
	}
}

// GetNextLeaderKey uniquely determine who is the leader for given viewID
func (consensus *Consensus) GetNextLeaderKey() *bls.PublicKey {
	idx := consensus.getIndexOfPubKey(consensus.LeaderPubKey)
	if idx == -1 {
		utils.Logger().Warn().
			Str("key", consensus.LeaderPubKey.SerializeToHexStr()).
			Msg("GetNextLeaderKey: currentLeaderKey not found")
	}
	idx = (idx + 1) % len(consensus.PublicKeys)
	return consensus.PublicKeys[idx]
}

func (consensus *Consensus) getIndexOfPubKey(pubKey *bls.PublicKey) int {
	for k, v := range consensus.PublicKeys {
		if v.IsEqual(pubKey) {
			return k
		}
	}
	return -1
}

// ResetViewChangeState reset the state for viewchange
func (consensus *Consensus) ResetViewChangeState() {
	utils.Logger().Debug().
		Str("Phase", consensus.phase.String()).
		Msg("[ResetViewChangeState] Resetting view change state")
	consensus.mode.SetMode(Normal)
	bhpBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, nil)
	nilBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, nil)
	viewIDBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, nil)
	consensus.bhpBitmap = bhpBitmap
	consensus.nilBitmap = nilBitmap
	consensus.viewIDBitmap = viewIDBitmap
	consensus.m1Payload = []byte{}

	consensus.bhpSigs = map[string]*bls.Sign{}
	consensus.nilSigs = map[string]*bls.Sign{}
	consensus.viewIDSigs = map[string]*bls.Sign{}
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
	consensus.mode.SetMode(ViewChanging)
	consensus.mode.SetViewID(viewID)
	consensus.LeaderPubKey = consensus.GetNextLeaderKey()

	diff := viewID - consensus.viewID
	duration := time.Duration(int64(diff) * int64(viewChangeDuration))
	utils.Logger().Info().
		Uint64("ViewChangingID", viewID).
		Dur("timeoutDuration", duration).
		Str("NextLeader", consensus.LeaderPubKey.SerializeToHexStr()).
		Msg("[startViewChange]")

	msgToSend := consensus.constructViewChangeMessage()
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	consensus.consensusTimeout[timeoutViewChange].Start()
	utils.Logger().Debug().
		Uint64("ViewChangingID", consensus.mode.ViewID()).
		Msg("[startViewChange] start view change timer")
}

func (consensus *Consensus) onViewChange(msg *msg_pb.Message) {
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		utils.Logger().Warn().Msg("[onViewChange] Unable To Parse Viewchange Message")
		return
	}
	newLeaderKey := recvMsg.LeaderPubkey
	if !consensus.PubKey.IsEqual(newLeaderKey) {
		return
	}

	if len(consensus.viewIDSigs) >= consensus.Quorum() {
		utils.Logger().Debug().
			Int("have", len(consensus.viewIDSigs)).
			Int("need", consensus.Quorum()).
			Str("validatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr()).
			Msg("[onViewChange] Received Enough View Change Messages")
		return
	}

	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[onViewChange] VerifySenderKey Failed")
		return
	}

	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	if consensus.blockNum > recvMsg.BlockNum {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] Message BlockNum Is Low")
		return
	}

	if consensus.blockNum < recvMsg.BlockNum {
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onViewChange] New Leader Has Lower Blocknum")
		return
	}

	if consensus.mode.Mode() == ViewChanging && consensus.mode.ViewID() > recvMsg.ViewID {
		utils.Logger().Warn().
			Uint64("MyViewChangingID", consensus.mode.ViewID()).
			Uint64("MsgViewChangingID", recvMsg.ViewID).
			Msg("[onViewChange] ViewChanging ID Is Low")
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.Logger().Debug().Err(err).Msg("[onViewChange] Failed To Verify Sender's Signature")
		return
	}

	consensus.vcLock.Lock()
	defer consensus.vcLock.Unlock()

	// add self m1 or m2 type message signature and bitmap
	_, ok1 := consensus.nilSigs[consensus.PubKey.SerializeToHexStr()]
	_, ok2 := consensus.bhpSigs[consensus.PubKey.SerializeToHexStr()]
	if !(ok1 || ok2) {
		// add own signature for newview message
		preparedMsgs := consensus.PbftLog.GetMessagesByTypeSeq(msg_pb.MessageType_PREPARED, recvMsg.BlockNum)
		preparedMsg := consensus.PbftLog.FindMessageByMaxViewID(preparedMsgs)
		if preparedMsg == nil {
			utils.Logger().Debug().Msg("[onViewChange] add my M2(NIL) type messaage")
			consensus.nilSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(NIL)
			consensus.nilBitmap.SetKey(consensus.PubKey, true)
		} else {
			utils.Logger().Debug().Msg("[onViewChange] add my M1 type messaage")
			msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
			consensus.bhpSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(msgToSign)
			consensus.bhpBitmap.SetKey(consensus.PubKey, true)
		}
	}
	// add self m3 type message signature and bitmap
	_, ok3 := consensus.viewIDSigs[consensus.PubKey.SerializeToHexStr()]
	if !ok3 {
		viewIDBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)
		consensus.viewIDSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(viewIDBytes)
		consensus.viewIDBitmap.SetKey(consensus.PubKey, true)
	}

	// m2 type message
	if len(recvMsg.Payload) == 0 {
		_, ok := consensus.nilSigs[senderKey.SerializeToHexStr()]
		if ok {
			utils.Logger().Debug().
				Str("validatorPubKey", senderKey.SerializeToHexStr()).
				Msg("[onViewChange] Already Received M2 message from validator")
			return
		}

		if !recvMsg.ViewchangeSig.VerifyHash(senderKey, NIL) {
			utils.Logger().Warn().Msg("[onViewChange] Failed To Verify Signature For M2 Type Viewchange Message")
			return
		}

		utils.Logger().Debug().
			Str("validatorPubKey", senderKey.SerializeToHexStr()).
			Msg("[onViewChange] Add M2 (NIL) type message")
		consensus.nilSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.nilBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	} else { // m1 type message
		_, ok := consensus.bhpSigs[senderKey.SerializeToHexStr()]
		if ok {
			utils.Logger().Debug().
				Str("validatorPubKey", senderKey.SerializeToHexStr()).
				Msg("[onViewChange] Already Received M1 Message From the Validator")
			return
		}
		if !recvMsg.ViewchangeSig.VerifyHash(recvMsg.SenderPubkey, recvMsg.Payload) {
			utils.Logger().Warn().Msg("[onViewChange] Failed to Verify Signature for M1 Type Viewchange Message")
			return
		}

		// first time receive m1 type message, need verify validity of prepared message
		if len(consensus.m1Payload) == 0 || !bytes.Equal(consensus.m1Payload, recvMsg.Payload) {
			if len(recvMsg.Payload) <= 32 {
				utils.Logger().Debug().
					Int("len", len(recvMsg.Payload)).
					Msg("[onViewChange] M1 RecvMsg Payload Not Enough Length")
				return
			}
			blockHash := recvMsg.Payload[:32]
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("[onViewChange] M1 RecvMsg Payload Read Error")
				return
			}
			// check has 2f+1 signature in m1 type message
			if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
				utils.Logger().Debug().
					Int("need", consensus.Quorum()).
					Int("have", count).
					Msg("[onViewChange] M1 Payload Not Have Enough Signature")
				return
			}

			// Verify the multi-sig for prepare phase
			if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
				utils.Logger().Warn().
					Hex("blockHash", blockHash).
					Msg("[onViewChange] failed to verify multi signature for m1 prepared payload")
				return
			}

			// if m1Payload is empty, we just add one
			if len(consensus.m1Payload) == 0 {
				consensus.m1Payload = append(recvMsg.Payload[:0:0], recvMsg.Payload...)
				// create prepared message for new leader
				preparedMsg := PbftMessage{MessageType: msg_pb.MessageType_PREPARED, ViewID: recvMsg.ViewID, BlockNum: recvMsg.BlockNum}
				preparedMsg.BlockHash = common.Hash{}
				copy(preparedMsg.BlockHash[:], recvMsg.Payload[:32])
				preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
				copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
				preparedMsg.SenderPubkey = consensus.PubKey
				utils.Logger().Info().Msg("[onViewChange] New Leader Prepared Message Added")
				consensus.PbftLog.AddMessage(&preparedMsg)
			}
		}
		utils.Logger().Debug().
			Str("validatorPubKey", senderKey.SerializeToHexStr()).
			Msg("[onViewChange] Add M1 (prepared) type message")
		consensus.bhpSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.bhpBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	}

	// check and add viewID (m3 type) message signature
	_, ok := consensus.viewIDSigs[senderKey.SerializeToHexStr()]
	if ok {
		utils.Logger().Debug().
			Str("validatorPubKey", senderKey.SerializeToHexStr()).
			Msg("[onViewChange] Already Received M3(ViewID) message from the validator")
		return
	}
	viewIDHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDHash, recvMsg.ViewID)
	if !recvMsg.ViewidSig.VerifyHash(recvMsg.SenderPubkey, viewIDHash) {
		utils.Logger().Warn().
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onViewChange] Failed to Verify M3 Message Signature")
		return
	}
	utils.Logger().Debug().
		Str("validatorPubKey", senderKey.SerializeToHexStr()).
		Msg("[onViewChange] Add M3 (ViewID) type message")
	consensus.viewIDSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewidSig
	consensus.viewIDBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	utils.Logger().Debug().
		Int("numSigs", len(consensus.viewIDSigs)).
		Int("needed", consensus.Quorum()).
		Msg("[onViewChange]")

	// received enough view change messages, change state to normal consensus
	if len(consensus.viewIDSigs) >= consensus.Quorum() {
		consensus.mode.SetMode(Normal)
		consensus.LeaderPubKey = consensus.PubKey
		consensus.ResetState()
		if len(consensus.m1Payload) == 0 {
			go func() {
				consensus.ReadySignal <- struct{}{}
			}()
		} else {
			utils.Logger().Debug().
				Str("From", consensus.phase.String()).
				Str("To", Commit.String()).
				Msg("[OnViewChange] Switching phase")
			consensus.switchPhase(Commit, true)
			copy(consensus.blockHash[:], consensus.m1Payload[:32])
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				utils.Logger().Error().Err(err).Msg("[onViewChange] ReadSignatureBitmapPayload Fail")
				return
			}
			consensus.aggregatedPrepareSig = aggSig
			consensus.prepareBitmap = mask

			// Leader sign and add commit message
			blockNumBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(blockNumBytes, consensus.blockNum)
			commitPayload := append(blockNumBytes, consensus.blockHash[:]...)
			consensus.commitSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(commitPayload)
			if err = consensus.commitBitmap.SetKey(consensus.PubKey, true); err != nil {
				utils.Logger().Debug().Msg("[OnViewChange] New Leader commit bitmap set failed")
				return
			}
		}

		consensus.mode.SetViewID(recvMsg.ViewID)
		msgToSend := consensus.constructNewViewMessage()

		utils.Logger().Warn().
			Int("payloadSize", len(consensus.m1Payload)).
			Hex("M1Payload", consensus.m1Payload).
			Msg("[onViewChange] Sent NewView Message")
		consensus.msgSender.SendWithRetry(consensus.blockNum, msg_pb.MessageType_NEWVIEW, []p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		consensus.viewID = recvMsg.ViewID
		consensus.ResetViewChangeState()
		consensus.consensusTimeout[timeoutViewChange].Stop()
		consensus.consensusTimeout[timeoutConsensus].Start()
		utils.Logger().Debug().
			Uint64("viewChangingID", consensus.mode.ViewID()).
			Msg("[onViewChange] New Leader Start Consensus Timer and Stop View Change Timer")
		utils.Logger().Debug().
			Str("myKey", consensus.PubKey.SerializeToHexStr()).
			Uint64("viewID", consensus.viewID).
			Uint64("block", consensus.blockNum).
			Msg("[onViewChange] I am the New Leader")
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onNewView(msg *msg_pb.Message) {
	utils.Logger().Debug().Msg("[onNewView] Received NewView Message")
	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[onNewView] VerifySenderKey Failed")
		return
	}
	recvMsg, err := consensus.ParseNewViewMessage(msg)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[onNewView] Unable to Parse NewView Message")
		return
	}

	if err = verifyMessageSig(senderKey, msg); err != nil {
		utils.Logger().Error().Err(err).Msg("[onNewView] Failed to Verify New Leader's Signature")
		return
	}
	consensus.vcLock.Lock()
	defer consensus.vcLock.Unlock()

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		utils.Logger().Error().Msg("[onNewView] M3AggSig or M3Bitmap is nil")
		return
	}
	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(viewIDBytes, recvMsg.ViewID)
	// check total number of sigs >= 2f+1
	if count := utils.CountOneBits(m3Mask.Bitmap); count < consensus.Quorum() {
		utils.Logger().Debug().
			Int("need", consensus.Quorum()).
			Int("have", count).
			Msg("[onNewView] Not Have Enough M3 (ViewID) Signature")
		return
	}

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		utils.Logger().Warn().
			Str("m3Sig", m3Sig.SerializeToHexStr()).
			Hex("m3Mask", m3Mask.Bitmap).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[onNewView] Unable to Verify Aggregated Signature of M3 (ViewID) payload")
		return
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		utils.Logger().Debug().Msg("[onNewView] M2AggSig (NIL) is Not Empty")
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			utils.Logger().Warn().Msg("[onNewView] Unable to Verify Aggregated Signature of M2 (NIL) payload")
			return
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	if m2Mask == nil || m2Mask.Bitmap == nil || (m2Mask != nil && m2Mask.Bitmap != nil && utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		if len(recvMsg.Payload) <= 32 {
			utils.Logger().Debug().Msg("[onNewView] M1 (prepared) Type Payload Not Have Enough Length")
			return
		}
		// m1 is not empty, check it's valid
		blockHash := recvMsg.Payload[:32]
		aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
		if err != nil {
			utils.Logger().Error().Err(err).Msg("[onNewView] ReadSignatureBitmapPayload Failed")
			return
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash) {
			utils.Logger().Warn().Msg("[onNewView] Failed to Verify Signature for M1 (prepare) message")
			return
		}
		copy(consensus.blockHash[:], blockHash)
		consensus.aggregatedPrepareSig = aggSig
		consensus.prepareBitmap = mask

		// create prepared message from newview
		preparedMsg := PbftMessage{MessageType: msg_pb.MessageType_PREPARED, ViewID: recvMsg.ViewID, BlockNum: recvMsg.BlockNum}
		preparedMsg.BlockHash = common.Hash{}
		copy(preparedMsg.BlockHash[:], blockHash[:])
		preparedMsg.Payload = make([]byte, len(recvMsg.Payload)-32)
		copy(preparedMsg.Payload[:], recvMsg.Payload[32:])
		preparedMsg.SenderPubkey = senderKey
		consensus.PbftLog.AddMessage(&preparedMsg)
	}

	// newView message verified success, override my state
	consensus.viewID = recvMsg.ViewID
	consensus.mode.SetViewID(recvMsg.ViewID)
	consensus.LeaderPubKey = senderKey
	consensus.ResetViewChangeState()

	// change view and leaderKey to keep in sync with network
	if consensus.blockNum != recvMsg.BlockNum {
		utils.Logger().Debug().
			Str("newLeaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[onNewView] New Leader Changed")
		return
	}

	// NewView message is verified, change state to normal consensus
	if len(recvMsg.Payload) > 32 {
		// Construct and send the commit message
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		msgToSend := consensus.constructCommitMessage(commitPayload)

		utils.Logger().Info().Msg("onNewView === commit")
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
		utils.Logger().Debug().
			Str("From", consensus.phase.String()).
			Str("To", Commit.String()).
			Msg("[OnViewChange] Switching phase")
		consensus.switchPhase(Commit, true)
	} else {
		consensus.ResetState()
		utils.Logger().Info().Msg("onNewView === announce")
	}
	utils.Logger().Debug().
		Str("newLeaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
		Msg("new leader changed")
	utils.Logger().Debug().Msg("validator start consensus timer and stop view change timer")
	consensus.consensusTimeout[timeoutConsensus].Start()
	consensus.consensusTimeout[timeoutViewChange].Stop()
}
