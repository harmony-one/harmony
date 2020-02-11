package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p/host"
)

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		errMsg := "[OnAnnounce] Unparseable leader message"
		consensus.getLogger().Error().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	// verify validity of block header object
	// TODO: think about just sending the block hash instead of the header.
	encodedHeader := recvMsg.Payload
	header := new(block.Header)
	if err := rlp.DecodeBytes(encodedHeader, header); err != nil {
		errMsg := "[OnAnnounce] Unparseable block header data"
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	if recvMsg.BlockNum < consensus.blockNum ||
		recvMsg.BlockNum != header.Number().Uint64() {
		errMsg := "[OnAnnounce] BlockNum does not match"
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("hdrBlockNum", header.Number().Uint64()).
			Uint64("consensuBlockNum", consensus.blockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	if err := chain.Engine.VerifyHeader(consensus.ChainReader, header, true); err != nil {
		errMsg := "[OnAnnounce] Block content is not verified successfully"
		consensus.getLogger().Warn().
			Err(err).
			Str("inChain", consensus.ChainReader.CurrentHeader().Number().String()).
			Str("MsgBlockNum", header.Number().String()).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	//VRF/VDF is only generated in the beach chain
	if consensus.NeedsRandomNumberGeneration(header.Epoch()) {
		//validate the VRF with proof if a non zero VRF is found in header
		if len(header.Vrf()) > 0 {
			if !consensus.ValidateVrfAndProof(header) {
				addToConsensusLog(msg, "[OnAnnounce] Failed VRF validation/proof")
				return
			}
		}

		//validate the VDF with proof if a non zero VDF is found in header
		if len(header.Vdf()) > 0 {
			if !consensus.ValidateVdfAndProof(header) {
				addToConsensusLog(msg, "[OnAnnounce] Failed VDF validation/proof")
				return
			}
		}
	}

	logMsgs := consensus.FBFTLog.GetMessagesByTypeSeqView(
		msg_pb.MessageType_ANNOUNCE, recvMsg.BlockNum, recvMsg.ViewID,
	)
	if len(logMsgs) > 0 {
		if logMsgs[0].BlockHash != recvMsg.BlockHash &&
			logMsgs[0].SenderPubkey.IsEqual(recvMsg.SenderPubkey) {
			errMsg := "[OnAnnounce] Leader is malicious"
			consensus.getLogger().Debug().
				Str("logMsgSenderKey", logMsgs[0].SenderPubkey.SerializeToHexStr()).
				Str("logMsgBlockHash", logMsgs[0].BlockHash.Hex()).
				Str("recvMsg.SenderPubkey", recvMsg.SenderPubkey.SerializeToHexStr()).
				Uint64("recvMsg.BlockNum", recvMsg.BlockNum).
				Uint64("recvMsg.ViewID", recvMsg.ViewID).
				Str("recvMsgBlockHash", recvMsg.BlockHash.Hex()).
				Str("LeaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
				Msg(errMsg)
			addToConsensusLog(msg, errMsg)
			if consensus.current.Mode() == ViewChanging {
				viewID := consensus.current.ViewID()
				consensus.startViewChange(viewID + 1)
			} else {
				consensus.startViewChange(consensus.viewID + 1)
			}
		}
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.SerializeToHexStr()).
			Msg("[OnAnnounce] Announce message received again")
	}

	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnAnnounce] Announce message Added")
	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.blockHash = recvMsg.BlockHash
	// we have already added message and block, skip check viewID
	// and send prepare message if is in ViewChanging mode
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().
			Msg("[OnAnnounce] Still in ViewChanging Mode, Exiting !!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			errMsg := "[OnAnnounce] ViewID check failed"
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		}
		return
	}
	//Sucessfully processed Announce message
	addToConsensusLog(msg, "")
	consensus.prepare()
}

func (consensus *Consensus) prepare() {
	network, err := consensus.construct(msg_pb.MessageType_PREPARE, nil)
	if err != nil {
		errMsg := "[OnAnnounce] Could not construct message"
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_PREPARE.String()).
			Msg(errMsg)
		addToConsensusLog(nil, errMsg)
		return
	}
	msgToSend := network.Bytes

	sendMsg := &msg_pb.Message{}
	protobuf.Unmarshal(msgToSend, sendMsg)

	// TODO: this will not return immediatey, may block
	if err := consensus.msgSender.SendWithoutRetry(
		[]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		host.ConstructP2pMessage(byte(17), msgToSend),
	); err != nil {
		errMsg := "[OnAnnounce] Cannot send prepare message"
		consensus.getLogger().Warn().Err(err).Msg(errMsg)
		addToConsensusLog(sendMsg, errMsg)
	} else {
		consensus.getLogger().Info().
			Str("blockHash", hex.EncodeToString(consensus.blockHash[:])).
			Msg("[OnAnnounce] Sent Prepare Message!!")
		addToConsensusLog(sendMsg, "")
	}
	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTPrepare.String()).
		Msg("[Announce] Switching Phase")
	consensus.switchPhase(FBFTPrepare, true)
}

// if onPrepared accepts the prepared message from the leader, then
// it will send a COMMIT message for the leader to receive on the network.
func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		errMsg := "[OnPrepared] Unparseable validator message"
		consensus.getLogger().Debug().Err(err).Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.blockNum {
		errMsg := "[OnPrepared] Old block received, ignoring!!"
		consensus.getLogger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		errMsg := "[OnPrepared] ReadSignatureBitmapPayload failed!!"
		consensus.getLogger().Error().Err(err).Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask, true) {
		consensus.getLogger().Warn().
			Msgf("[OnPrepared] Quorum Not achieved")
		return
	}

	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		errMsg := "[OnPrepared] failed to verify multi signature for prepare phase"
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	// check validity of block
	var blockObj types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		errMsg := "[OnPrepared] Unparseable block header data"
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}
	// let this handle it own logs
	if !consensus.onPreparedSanityChecks(&blockObj, msg) {
		return
	}
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.FBFTLog.AddBlock(&blockObj)
	// add block field
	blockPayload := make([]byte, len(recvMsg.Block))
	copy(blockPayload[:], recvMsg.Block[:])
	consensus.block = blockPayload
	recvMsg.Block = []byte{} // save memory space
	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Hex("blockHash", recvMsg.BlockHash[:]).
		Msg("[OnPrepared] Prepared message and block added")

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().Msg("[OnPrepared] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			errMsg := "[OnPrepared] ViewID check failed"
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg(errMsg)
			addToConsensusLog(msg, errMsg)
		}
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		errMsg := "[OnPrepared] Future Block Received, ignoring!!"
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	// add preparedSig field
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	// Optimistically add blockhash field of prepare message
	emptyHash := [32]byte{}
	if bytes.Compare(consensus.blockHash[:], emptyHash[:]) == 0 {
		copy(consensus.blockHash[:], blockHash[:])
	}
	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, consensus.blockNum)
	network, _ := consensus.construct(
		// TODO: should only sign on block hash
		msg_pb.MessageType_COMMIT,
		append(blockNumBytes, consensus.blockHash[:]...),
	)

	//Successfully processed Prepared message
	addToConsensusLog(msg, "")
	msgToSend := network.Bytes

	sendMsg := &msg_pb.Message{}
	protobuf.Unmarshal(msgToSend, sendMsg)
	// TODO: genesis account node delay for 1 second,
	// this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	if err := consensus.msgSender.SendWithoutRetry(
		[]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
		host.ConstructP2pMessage(byte(17), msgToSend),
	); err != nil {
		errMsg := "[OnPrepared] Cannot send commit message!!"
		consensus.getLogger().Warn().Msg(errMsg)
		addToConsensusLog(msg, errMsg)
	} else {
		consensus.getLogger().Info().
			Uint64("blockNum", consensus.blockNum).
			Hex("blockHash", consensus.blockHash[:]).
			Msg("[OnPrepared] Sent Commit Message!!")
		addToConsensusLog(msg, "")
	}

	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTCommit.String()).
		Msg("[OnPrepared] Switching phase")
	consensus.switchPhase(FBFTCommit, true)
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		errMsg := "[OnCommitted] unable to parse msg"
		consensus.getLogger().Warn().Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	if recvMsg.BlockNum < consensus.blockNum {
		errMsg := "[OnCommitted] Received Old Blocks!!"
		consensus.getLogger().Info().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		errMsg := "[OnCommitted] readSignatureBitmapPayload failed"
		consensus.getLogger().Error().Err(err).Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask, true) {
		consensus.getLogger().Warn().
			Msgf("[OnCommitted] Quorum Not achieved")
		return
	}

	blockNumBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumBytes, recvMsg.BlockNum)
	commitPayload := append(blockNumBytes, recvMsg.BlockHash[:]...)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		errMsg := "[OnCommitted] Failed to verify the multi signature for commit phase"
		consensus.getLogger().Error().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg(errMsg)
		addToConsensusLog(msg, errMsg)
		return
	}

	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.ChainReader.WriteLastCommits(recvMsg.Payload)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnCommitted] Committed message added")

	// Successfully processed Committed message
	addToConsensusLog(msg, "")

	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	if recvMsg.BlockNum-consensus.blockNum > consensusBlockNumBuffer {
		consensus.getLogger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).Msg("[OnCommitted] out of sync")
		go func() {
			select {
			case consensus.blockNumLowChan <- struct{}{}:
				consensus.current.SetMode(Syncing)
				for _, v := range consensus.consensusTimeout {
					v.Stop()
				}
			case <-time.After(1 * time.Second):
			}
		}()
		return
	}

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		consensus.getLogger().Debug().Msg("[OnCommitted] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug().Msg("[OnCommitted] Start consensus timer; stop bootstrap timer only once")
	} else {
		consensus.getLogger().Debug().Msg("[OnCommitted] Start consensus timer")
	}
	consensus.consensusTimeout[timeoutConsensus].Start()
}
