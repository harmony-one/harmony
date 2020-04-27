package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnAnnounce] Unparseable leader message")
		return
	}

	// NOTE let it handle its own logs
	if !consensus.onAnnounceSanityChecks(recvMsg) {
		return
	}

	utils.Logger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnAnnounce] Announce message Added")
	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.blockHash = recvMsg.BlockHash
	// we have already added message and block, skip check viewID
	// and send prepare message if is in ViewChanging mode
	if consensus.current.Mode() == ViewChanging {
		utils.Logger().Debug().
			Msg("[OnAnnounce] Still in ViewChanging Mode, Exiting !!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			utils.Logger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnAnnounce] ViewID check failed")
		}
		return
	}
	consensus.prepare()
}

func (consensus *Consensus) prepare() {
	groupID := []nodeconfig.GroupID{
		nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
	}
	for i, key := range consensus.PubKey.PublicKey {
		networkMessage, err := consensus.construct(
			msg_pb.MessageType_PREPARE, nil, key, consensus.priKey.PrivateKey[i],
		)
		if err != nil {
			utils.Logger().Err(err).
				Str("message-type", msg_pb.MessageType_PREPARE.String()).
				Msg("could not construct message")
			return
		}

		// TODO: this will not return immediatey, may block
		if consensus.current.Mode() != Listening {
			if err := consensus.host.SendMessageToGroups(
				groupID,
				p2p.ConstructMessage(networkMessage.Bytes),
			); err != nil {
				utils.Logger().Warn().Err(err).Msg("[OnAnnounce] Cannot send prepare message")
			} else {
				utils.Logger().Info().
					Str("blockHash", hex.EncodeToString(consensus.blockHash[:])).
					Msg("[OnAnnounce] Sent Prepare Message!!")
			}
		}
	}
	consensus.switchPhase(FBFTPrepare)
}

// if onPrepared accepts the prepared message from the leader, then
// it will send a COMMIT message for the leader to receive on the network.
func (consensus *Consensus) onPrepared(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[OnPrepared] Unparseable validator message")
		return
	}
	utils.Logger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.blockNum {
		utils.Logger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("ReadSignatureBitmapPayload failed!")
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask) {
		utils.Logger().Warn().
			Msgf("[OnPrepared] Quorum Not achieved")
		return
	}

	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		myBlockHash := common.Hash{}
		myBlockHash.SetBytes(consensus.blockHash[:])
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[OnPrepared] failed to verify multi signature for prepare phase")
		return
	}

	// check validity of block
	var blockObj types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		utils.Logger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepared] Unparseable block header data")
		return
	}
	// let this handle it own logs
	if !consensus.onPreparedSanityChecks(&blockObj, recvMsg) {
		return
	}

	consensus.FBFTLog.AddBlock(&blockObj)
	// add block field
	blockPayload := make([]byte, len(recvMsg.Block))
	copy(blockPayload[:], recvMsg.Block[:])
	consensus.block = blockPayload
	recvMsg.Block = []byte{} // save memory space
	consensus.FBFTLog.AddMessage(recvMsg)
	utils.Logger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Hex("blockHash", recvMsg.BlockHash[:]).
		Msg("[OnPrepared] Prepared message and block added")

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		utils.Logger().Debug().Msg("[OnPrepared] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.current.Mode() == Normal {
			utils.Logger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnPrepared] ViewID check failed")
		}
		return
	}
	if recvMsg.BlockNum > consensus.blockNum {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepared] Future Block Received, ignoring!!")
		return
	}

	// TODO: genesis account node delay for 1 second,
	// this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	// add preparedSig field
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	// Optimistically add blockhash field of prepare message
	emptyHash := [32]byte{}
	if bytes.Equal(consensus.blockHash[:], emptyHash[:]) {
		copy(consensus.blockHash[:], blockHash[:])
	}

	// local viewID may not be constant with other, so use received msg viewID.
	commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
		new(big.Int).SetUint64(consensus.epoch), consensus.blockHash, consensus.blockNum, recvMsg.ViewID)
	groupID := []nodeconfig.GroupID{
		nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
	}
	for i, key := range consensus.PubKey.PublicKey {
		networkMessage, _ := consensus.construct(
			msg_pb.MessageType_COMMIT,
			commitPayload,
			key, consensus.priKey.PrivateKey[i],
		)

		if consensus.current.Mode() != Listening {
			if err := consensus.host.SendMessageToGroups(groupID,
				p2p.ConstructMessage(networkMessage.Bytes),
			); err != nil {
				utils.Logger().Warn().Msg("[OnPrepared] Cannot send commit message!!")
			} else {
				utils.Logger().Info().
					Uint64("blockNum", consensus.blockNum).
					Hex("blockHash", consensus.blockHash[:]).
					Msg("[OnPrepared] Sent Commit Message!!")
			}
		}
	}
	consensus.switchPhase(FBFTCommit)
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Warn().Msg("[OnCommitted] unable to parse msg")
		return
	}
	// NOTE let it handle its own logs
	if !consensus.isRightBlockNumCheck(recvMsg) {
		return
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[OnCommitted] readSignatureBitmapPayload failed")
		return
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask) {
		utils.Logger().Warn().
			Msgf("[OnCommitted] Quorum Not achieved")
		return
	}

	// Received msg must be about same epoch, otherwise it's invalid anyways.
	commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
		new(big.Int).SetUint64(consensus.epoch), recvMsg.BlockHash, recvMsg.BlockNum, recvMsg.ViewID)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		utils.Logger().Error().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnCommitted] Failed to verify the multi signature for commit phase")
		return
	}

	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	if recvMsg.BlockNum-consensus.blockNum > consensusBlockNumBuffer {
		fmt.Println("out of sync actually happened")
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnCommitted] OUT OF SYNC")
		// go func() {
		// 	select {
		// 	case consensus.BlockNumLowChan <- struct{}{}:
		// 		consensus.current.SetMode(Syncing)
		// 	case <-time.After(1 * time.Second):
		// 	}
		// }()
		// return
	}

	consensus.tryCatchup()
	if consensus.current.Mode() == ViewChanging {
		utils.Logger().Debug().Msg("[OnCommitted] Still in ViewChanging mode, Exiting!!")
		return
	}

	utils.Logger().Debug().Msg("[OnCommitted] Start consensus timer")
	consensus.timeouts.consensus.Start()
}
