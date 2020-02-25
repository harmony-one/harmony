package consensus

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p/host"
)

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		consensus.getLogger().Error().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnAnnounce] Unparseable leader message")
		return
	}

	// NOTE let it handle its own logs
	if !consensus.onAnnounceSanityChecks(recvMsg) {
		return
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
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnAnnounce] ViewID check failed")
		}
		return
	}
	consensus.prepare()
}

func (consensus *Consensus) prepare() {
	networkMessage, err := consensus.construct(msg_pb.MessageType_PREPARE, nil, nil)
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_PREPARE.String()).
			Msg("could not construct message")
		return
	}

	// TODO: this will not return immediatey, may block
	if err := consensus.msgSender.SendWithoutRetry(
		[]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		host.ConstructP2pMessage(byte(17), networkMessage.Bytes),
	); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnAnnounce] Cannot send prepare message")
	} else {
		consensus.getLogger().Info().
			Str("blockHash", hex.EncodeToString(consensus.blockHash[:])).
			Msg("[OnAnnounce] Sent Prepare Message!!")
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
		consensus.getLogger().Debug().Err(err).Msg("[OnPrepared] Unparseable validator message")
		return
	}
	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.blockNum {
		consensus.getLogger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("ReadSignatureBitmapPayload failed!")
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
		consensus.getLogger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[OnPrepared] failed to verify multi signature for prepare phase")
		return
	}

	// check validity of block
	var blockObj types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepared] Unparseable block header data")
		return
	}
	// let this handle it own logs
	if !consensus.onPreparedSanityChecks(&blockObj, recvMsg) {
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
			consensus.getLogger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnPrepared] ViewID check failed")
		}
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		consensus.getLogger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepared] Future Block Received, ignoring!!")
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
	networkMessage, _ := consensus.construct(
		msg_pb.MessageType_COMMIT,
		append(blockNumBytes, consensus.blockHash[:]...),
		consensus.blockHash[:],
	)
	// TODO: genesis account node delay for 1 second,
	// this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	if err := consensus.msgSender.SendWithoutRetry(
		[]nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
		host.ConstructP2pMessage(byte(17), networkMessage.Bytes),
	); err != nil {
		consensus.getLogger().Warn().Msg("[OnPrepared] Cannot send commit message!!")
	} else {
		consensus.getLogger().Info().
			Uint64("blockNum", consensus.blockNum).
			Hex("blockHash", consensus.blockHash[:]).
			Msg("[OnPrepared] Sent Commit Message!!")
	}

	if consensus.DoDoubleSign {
		cpy := append(consensus.blockHash[0:0], consensus.blockHash[:]...)
		cpy[0] = byte(29)
		networkMsg, _ := consensus.construct(
			msg_pb.MessageType_COMMIT, append(blockNumBytes, cpy...), cpy,
		)
		l := consensus.getLogger().Info().
			Str("consensus", consensus.String()).
			Str("double-signed-msg", networkMsg.FBFTMsg.String())

		if err := consensus.msgSender.SendWithoutRetry(
			[]nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))},
			host.ConstructP2pMessage(byte(17), networkMsg.Bytes),
		); err != nil {
			l.Err(err).Msg("trouble sending out the double-sign message")
		} else {
			l.Msg("successfully sent a double sign, setting .DoubleSign=false")
			consensus.DoDoubleSign = false
		}
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
		consensus.getLogger().Warn().Msg("[OnCommitted] unable to parse msg")
		return
	}
	// NOTE let it handle its own logs
	if !consensus.isRightBlockNumCheck(recvMsg) {
		return
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnCommitted] readSignatureBitmapPayload failed")
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
		consensus.getLogger().Error().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnCommitted] Failed to verify the multi signature for commit phase")
		return
	}

	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.ChainReader.WriteLastCommits(recvMsg.Payload)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnCommitted] Committed message added")

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
