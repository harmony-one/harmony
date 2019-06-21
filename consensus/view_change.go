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
)

// PbftMode contains mode and viewID of viewchanging
type PbftMode struct {
	mode   Mode
	viewID uint32
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
		return "Sycning"
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
func (pm *PbftMode) ViewID() uint32 {
	return pm.viewID
}

// SetViewID sets the viewchanging id accordingly
func (pm *PbftMode) SetViewID(viewID uint32) {
	pm.mux.Lock()
	defer pm.mux.Unlock()
	pm.viewID = viewID
}

// GetViewID returns the current viewchange viewID
func (pm *PbftMode) GetViewID() uint32 {
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
		consensus.getLogger().Warn("GetNextLeaderKey: currentLeaderKey not found", "key", consensus.LeaderPubKey.SerializeToHexStr())
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
	consensus.getLogger().Debug("[ResetViewChangeState] Resetting view change state", "Phase", consensus.phase)
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
func (consensus *Consensus) startViewChange(viewID uint32) {
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
	consensus.getLogger().Info("[startViewChange]", "ViewChangingID", viewID, "timeoutDuration", duration, "NextLeader", consensus.LeaderPubKey.SerializeToHexStr())

	msgToSend := consensus.constructViewChangeMessage()
	consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

	consensus.consensusTimeout[timeoutViewChange].SetDuration(duration)
	consensus.consensusTimeout[timeoutViewChange].Start()
	consensus.getLogger().Debug("[startViewChange] start view change timer", "ViewChangingID", consensus.mode.ViewID())
}

func (consensus *Consensus) onViewChange(msg *msg_pb.Message) {
	recvMsg, err := ParseViewChangeMessage(msg)
	if err != nil {
		consensus.getLogger().Warn("[onViewChange] Unable To Parse Viewchange Message")
		return
	}
	newLeaderKey := recvMsg.LeaderPubkey
	if !consensus.PubKey.IsEqual(newLeaderKey) {
		return
	}

	if len(consensus.viewIDSigs) >= consensus.Quorum() {
		consensus.getLogger().Debug("[onViewChange] Received Enough View Change Messages", "have", len(consensus.viewIDSigs), "need", consensus.Quorum(), "validatorPubKey", recvMsg.SenderPubkey.SerializeToHexStr())
		return
	}

	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		consensus.getLogger().Debug("[onViewChange] VerifySenderKey Failed", "error", err)
		return
	}

	// TODO: if difference is only one, new leader can still propose the same committed block to avoid another view change
	if consensus.blockNum > recvMsg.BlockNum {
		consensus.getLogger().Debug("[onViewChange] Message BlockNum Is Low", "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	if consensus.blockNum < recvMsg.BlockNum {
		consensus.getLogger().Warn("[onViewChange] New Leader Has Lower Blocknum", "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	if consensus.mode.Mode() == ViewChanging && consensus.mode.ViewID() > recvMsg.ViewID {
		consensus.getLogger().Warn("[onViewChange] ViewChanging ID Is Low", "MyViewChangingID", consensus.mode.ViewID(), "MsgViewChangingID", recvMsg.ViewID)
		return
	}
	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Debug("[onViewChange] Failed To Verify Sender's Signature", "error", err)
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
			consensus.getLogger().Debug("[onViewChange] add my M2(NIL) type messaage")
			consensus.nilSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(NIL)
			consensus.nilBitmap.SetKey(consensus.PubKey, true)
		} else {
			consensus.getLogger().Debug("[onViewChange] add my M1 type messaage")
			msgToSign := append(preparedMsg.BlockHash[:], preparedMsg.Payload...)
			consensus.bhpSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(msgToSign)
			consensus.bhpBitmap.SetKey(consensus.PubKey, true)
		}
	}
	// add self m3 type message signature and bitmap
	_, ok3 := consensus.viewIDSigs[consensus.PubKey.SerializeToHexStr()]
	if !ok3 {
		viewIDBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(viewIDBytes, recvMsg.ViewID)
		consensus.viewIDSigs[consensus.PubKey.SerializeToHexStr()] = consensus.priKey.SignHash(viewIDBytes)
		consensus.viewIDBitmap.SetKey(consensus.PubKey, true)
	}

	// m2 type message
	if len(recvMsg.Payload) == 0 {
		_, ok := consensus.nilSigs[senderKey.SerializeToHexStr()]
		if ok {
			consensus.getLogger().Debug("[onViewChange] Already Received M2 message from validator", "validatorPubKey", senderKey.SerializeToHexStr())
			return
		}

		if !recvMsg.ViewchangeSig.VerifyHash(senderKey, NIL) {
			consensus.getLogger().Warn("[onViewChange] Failed To Verify Signature For M2 Type Viewchange Message")
			return
		}

		consensus.getLogger().Debug("[onViewChange] Add M2 (NIL) type message", "validatorPubKey", senderKey.SerializeToHexStr())
		consensus.nilSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.nilBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	} else { // m1 type message
		_, ok := consensus.bhpSigs[senderKey.SerializeToHexStr()]
		if ok {
			consensus.getLogger().Debug("[onViewChange] Already Received M1 Message From the Validator", "validatorPubKey", senderKey.SerializeToHexStr())
			return
		}
		if !recvMsg.ViewchangeSig.VerifyHash(recvMsg.SenderPubkey, recvMsg.Payload) {
			consensus.getLogger().Warn("[onViewChange] Failed to Verify Signature for M1 Type Viewchange Message")
			return
		}

		// first time receive m1 type message, need verify validity of prepared message
		if len(consensus.m1Payload) == 0 || !bytes.Equal(consensus.m1Payload, recvMsg.Payload) {
			if len(recvMsg.Payload) <= 32 {
				consensus.getLogger().Debug("[onViewChange] M1 RecvMsg Payload Not Enough Length", "len", len(recvMsg.Payload))
				return
			}
			blockHash := recvMsg.Payload[:32]
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				consensus.getLogger().Error("[onViewChange] M1 RecvMsg Payload Read Error", "error", err)
				return
			}
			// check has 2f+1 signature in m1 type message
			if count := utils.CountOneBits(mask.Bitmap); count < consensus.Quorum() {
				consensus.getLogger().Debug("[onViewChange] M1 Payload Not Have Enough Signature", "need", consensus.Quorum(), "have", count)
				return
			}

			// Verify the multi-sig for prepare phase
			if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
				consensus.getLogger().Warn("[onViewChange] failed to verify multi signature for m1 prepared payload", "blockHash", blockHash)
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
				consensus.getLogger().Info("[onViewChange] New Leader Prepared Message Added")
				consensus.PbftLog.AddMessage(&preparedMsg)
			}
		}
		consensus.getLogger().Debug("[onViewChange] Add M1 (prepared) type message", "validatorPubKey", senderKey.SerializeToHexStr())
		consensus.bhpSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewchangeSig
		consensus.bhpBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	}

	// check and add viewID (m3 type) message signature
	_, ok := consensus.viewIDSigs[senderKey.SerializeToHexStr()]
	if ok {
		consensus.getLogger().Debug("[onViewChange] Already Received M3(ViewID) message from the validator", "senderKey.SerializeToHexStr()", senderKey.SerializeToHexStr())
		return
	}
	viewIDHash := make([]byte, 4)
	binary.LittleEndian.PutUint32(viewIDHash, recvMsg.ViewID)
	if !recvMsg.ViewidSig.VerifyHash(recvMsg.SenderPubkey, viewIDHash) {
		consensus.getLogger().Warn("[onViewChange] Failed to Verify M3 Message Signature", "MsgViewID", recvMsg.ViewID)
		return
	}
	consensus.getLogger().Debug("[onViewChange] Add M3 (ViewID) type message", "validatorPubKey", senderKey.SerializeToHexStr())
	consensus.viewIDSigs[senderKey.SerializeToHexStr()] = recvMsg.ViewidSig
	consensus.viewIDBitmap.SetKey(recvMsg.SenderPubkey, true) // Set the bitmap indicating that this validator signed.
	consensus.getLogger().Debug("[onViewChange]", "numSigs", len(consensus.viewIDSigs), "needed", consensus.Quorum())

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
			consensus.getLogger().Debug("[OnViewChange] Switching phase", "From", consensus.phase, "To", Commit)
			consensus.switchPhase(Commit, true)
			copy(consensus.blockHash[:], consensus.m1Payload[:32])
			aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
			if err != nil {
				consensus.getLogger().Error("[onViewChange] ReadSignatureBitmapPayload Fail", "error", err)
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
				consensus.getLogger().Debug("[OnViewChange] New Leader commit bitmap set failed")
				return
			}
		}

		consensus.mode.SetViewID(recvMsg.ViewID)
		msgToSend := consensus.constructNewViewMessage()
		consensus.getLogger().Warn("[onViewChange] Sent NewView Message", "len(M1Payload)", len(consensus.m1Payload), "M1Payload", consensus.m1Payload)
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))

		consensus.viewID = recvMsg.ViewID
		consensus.ResetViewChangeState()
		consensus.consensusTimeout[timeoutViewChange].Stop()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Debug("[onViewChange] New Leader Start Consensus Timer and Stop View Change Timer", "viewChangingID", consensus.mode.ViewID())
		consensus.getLogger().Debug("[onViewChange] I am the New Leader", "myKey", consensus.PubKey.SerializeToHexStr(), "viewID", consensus.viewID, "block", consensus.blockNum)
	}
}

// TODO: move to consensus_leader.go later
func (consensus *Consensus) onNewView(msg *msg_pb.Message) {
	consensus.getLogger().Debug("[onNewView] Received NewView Message")
	senderKey, err := consensus.verifyViewChangeSenderKey(msg)
	if err != nil {
		consensus.getLogger().Warn("[onNewView] VerifySenderKey Failed", "error", err)
		return
	}
	recvMsg, err := consensus.ParseNewViewMessage(msg)
	if err != nil {
		consensus.getLogger().Warn("[onNewView] Unable to Parse NewView Message", "error", err)
		return
	}

	if err = verifyMessageSig(senderKey, msg); err != nil {
		consensus.getLogger().Error("[onNewView] Failed to Verify New Leader's Signature", "error", err)
		return
	}
	consensus.vcLock.Lock()
	defer consensus.vcLock.Unlock()

	if recvMsg.M3AggSig == nil || recvMsg.M3Bitmap == nil {
		consensus.getLogger().Error("[onNewView] M3AggSig or M3Bitmap is nil")
		return
	}
	m3Sig := recvMsg.M3AggSig
	m3Mask := recvMsg.M3Bitmap

	viewIDBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(viewIDBytes, recvMsg.ViewID)
	// check total number of sigs >= 2f+1
	if count := utils.CountOneBits(m3Mask.Bitmap); count < consensus.Quorum() {
		consensus.getLogger().Debug("[onNewView] Not Have Enough M3 (ViewID) Signature", "need", consensus.Quorum(), "have", count)
		return
	}

	if !m3Sig.VerifyHash(m3Mask.AggregatePublic, viewIDBytes) {
		consensus.getLogger().Warn("[onNewView] Unable to Verify Aggregated Signature of M3 (ViewID) payload", "m3Sig", m3Sig.SerializeToHexStr(), "m3Mask", m3Mask.Bitmap, "MsgViewID", recvMsg.ViewID)
		return
	}

	m2Mask := recvMsg.M2Bitmap
	if recvMsg.M2AggSig != nil {
		consensus.getLogger().Debug("[onNewView] M2AggSig (NIL) is Not Empty")
		m2Sig := recvMsg.M2AggSig
		if !m2Sig.VerifyHash(m2Mask.AggregatePublic, NIL) {
			consensus.getLogger().Warn("[onNewView] Unable to Verify Aggregated Signature of M2 (NIL) payload")
			return
		}
	}

	// check when M3 sigs > M2 sigs, then M1 (recvMsg.Payload) should not be empty
	if m2Mask == nil || m2Mask.Bitmap == nil || (m2Mask != nil && m2Mask.Bitmap != nil && utils.CountOneBits(m3Mask.Bitmap) > utils.CountOneBits(m2Mask.Bitmap)) {
		if len(recvMsg.Payload) <= 32 {
			consensus.getLogger().Debug("[onNewView] M1 (prepared) Type Payload Not Have Enough Length")
			return
		}
		// m1 is not empty, check it's valid
		blockHash := recvMsg.Payload[:32]
		aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 32)
		if err != nil {
			consensus.getLogger().Error("[onNewView] ReadSignatureBitmapPayload Failed", "error", err)
			return
		}
		if !aggSig.VerifyHash(mask.AggregatePublic, blockHash) {
			consensus.getLogger().Warn("[onNewView] Failed to Verify Signature for M1 (prepare) message")
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
		consensus.getLogger().Debug("[onNewView] New Leader Changed", "newLeaderKey", consensus.LeaderPubKey.SerializeToHexStr(), "MsgBlockNum", recvMsg.BlockNum)
		return
	}

	// NewView message is verified, change state to normal consensus
	if len(recvMsg.Payload) > 32 {
		// Construct and send the commit message
		blockNumHash := make([]byte, 8)
		binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
		commitPayload := append(blockNumHash, consensus.blockHash[:]...)
		msgToSend := consensus.constructCommitMessage(commitPayload)

		consensus.getLogger().Info("onNewView === commit")
		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
		consensus.getLogger().Debug("[OnViewChange] Switching phase", "From", consensus.phase, "To", Commit)
		consensus.switchPhase(Commit, true)
	} else {
		consensus.ResetState()
		consensus.getLogger().Info("onNewView === announce")
	}
	consensus.getLogger().Debug("new leader changed", "newLeaderKey", consensus.LeaderPubKey.SerializeToHexStr())
	consensus.getLogger().Debug("validator start consensus timer and stop view change timer")
	consensus.consensusTimeout[timeoutConsensus].Start()
	consensus.consensusTimeout[timeoutViewChange].Stop()
}
