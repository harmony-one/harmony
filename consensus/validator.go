package consensus

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/pkg/errors"
)

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) error {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Error().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnAnnounce] Unparseable leader message")
		return err
	}

	consensus.Locks.Global.Lock()
	defer consensus.Locks.Global.Unlock()

	// NOTE let it handle its own logs
	if !consensus.onAnnounceSanityChecks(recvMsg) {
		return nil
	}

	consensus.FBFTLog.AddMessage(recvMsg)
	consensus.SetBlockHash(recvMsg.BlockHash)

	// we have already added message and block, skip check viewID
	// and send prepare message if is in ViewChanging mode
	if consensus.Current.Mode() == ViewChanging {
		utils.Logger().Debug().
			Msg("[OnAnnounce] Still in ViewChanging Mode, Exiting !!")
		return nil
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.Current.Mode() == Normal {
			utils.Logger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnAnnounce] ViewID check failed")
			return errors.New("viewID check failed")
		}
	}
	return consensus.prepare()
}

func (consensus *Consensus) prepare() error {
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
			return err
		}

		if err := consensus.host.SendMessageToGroups(
			groupID,
			p2p.ConstructMessage(networkMessage.Bytes),
		); err != nil {
			utils.Logger().Warn().Err(err).Msg("[OnAnnounce] Cannot send prepare message")
			return err
		}

		utils.Logger().Info().
			Msg("[OnAnnounce] Sent Prepare Message!!")

	}
	consensus.switchPhase(FBFTPrepare)
	return nil
}

// if onPrepared accepts the prepared message from the leader, then
// it will send a COMMIT message for the leader to receive on the network.
func (consensus *Consensus) onPrepared(msg *msg_pb.Message) error {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("[OnPrepared] Unparseable validator message")
		return err
	}
	utils.Logger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.BlockNum() {
		utils.Logger().Debug().Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return err
	}

	consensus.Locks.Global.Lock()
	defer consensus.Locks.Global.Unlock()

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("ReadSignatureBitmapPayload failed!")
		return err
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask) {
		utils.Logger().Warn().
			Msgf("[OnPrepared] Quorum Not achieved")
		return nil
	}

	if !aggSig.VerifyHash(mask.AggregatePublic, blockHash[:]) {
		utils.Logger().Warn().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[OnPrepared] failed to verify multi signature for prepare phase")
		return errors.New("failed to verify multi signature for prepare phase")
	}

	// check validity of block
	var blockObj types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		utils.Logger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnPrepared] Unparseable block header data")
		return err
	}
	// let this handle it own logs
	if !consensus.onPreparedSanityChecks(&blockObj, recvMsg) {
		return nil
	}

	consensus.FBFTLog.AddBlock(&blockObj)
	consensus.SetBlock(recvMsg.Block)
	consensus.FBFTLog.AddMessage(recvMsg)
	utils.Logger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Hex("blockHash", recvMsg.BlockHash[:]).
		Msg("[OnPrepared] Prepared message and block added")

	if err := consensus.tryCatchup(); err != nil {
		return err
	}

	if m := consensus.Current.Mode(); m != Normal {
		// don't sign the block that is not verified
		utils.Logger().Debug().Msg("[OnPrepared] Not in normal mode, Exiting!!")
		return errors.Errorf("not in normal mode current: %s", m.String())
	}

	if consensus.checkViewID(recvMsg) != nil {
		if consensus.Current.Mode() == Normal {
			utils.Logger().Debug().
				Uint64("MsgViewID", recvMsg.ViewID).
				Uint64("MsgBlockNum", recvMsg.BlockNum).
				Msg("[OnPrepared] ViewID check failed")
			return errors.New("viewID check failed")
		}
	}
	if num := consensus.BlockNum(); recvMsg.BlockNum > num {
		utils.Logger().Debug().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", num).
			Msg("[OnPrepared] Future Block Received, ignoring!!")
		return errors.New("future Block Received, ignoring")
	}

	// TODO: genesis account node delay for 1 second,
	// this is a temp fix for allows FN nodes to earning reward
	if consensus.delayCommit > 0 {
		time.Sleep(consensus.delayCommit)
	}

	// add preparedSig field
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	// local viewID may not be constant with other, so use received msg viewID.
	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		new(big.Int).SetUint64(consensus.Epoch()),
		consensus.BlockHash().Bytes(),
		consensus.BlockNum(),
		recvMsg.ViewID,
	)
	groupID := []nodeconfig.GroupID{
		nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
	}
	for i, key := range consensus.PubKey.PublicKey {
		networkMessage, _ := consensus.construct(
			msg_pb.MessageType_COMMIT,
			commitPayload,
			key, consensus.priKey.PrivateKey[i],
		)

		if consensus.Current.Mode() != Listening {
			if err := consensus.host.SendMessageToGroups(groupID,
				p2p.ConstructMessage(networkMessage.Bytes),
			); err != nil {
				utils.Logger().Warn().Msg("[OnPrepared] Cannot send commit message!!")
				return err
			}
			utils.Logger().Info().
				Hex("blockHash", consensus.BlockHash().Bytes()).
				Msg("[OnPrepared] Sent Commit Message!!")

		}
	}
	consensus.switchPhase(FBFTCommit)
	return nil
}

func (consensus *Consensus) onCommitted(msg *msg_pb.Message) error {
	recvMsg, err := ParseFBFTMessage(msg)
	if err != nil {
		utils.Logger().Warn().Msg("[OnCommitted] unable to parse msg")
		return err
	}

	consensus.Locks.Global.Lock()
	defer consensus.Locks.Global.Unlock()

	// NOTE let it handle its own logs
	if !consensus.isRightBlockNumCheck(recvMsg) {
		return nil
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(recvMsg.Payload, 0)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("[OnCommitted] readSignatureBitmapPayload failed")
		return err
	}

	if !consensus.Decider.IsQuorumAchievedByMask(mask) {
		utils.Logger().Warn().
			Msgf("[OnCommitted] Quorum Not achieved")
		return nil
	}

	// Received msg must be about same epoch, otherwise it's invalid anyways.
	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		new(big.Int).SetUint64(consensus.Epoch()),
		recvMsg.BlockHash.Bytes(),
		recvMsg.BlockNum,
		recvMsg.ViewID,
	)
	if !aggSig.VerifyHash(mask.AggregatePublic, commitPayload) {
		utils.Logger().Error().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[OnCommitted] Failed to verify the multi signature for commit phase")
		return errors.New("Failed to verify the multi signature for commit phase")
	}

	consensus.FBFTLog.AddMessage(recvMsg)

	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	if recvMsg.BlockNum-consensus.BlockNum() > consensusBlockNumBuffer {

		fmt.Println("out of sync actually happened",
			recvMsg.BlockNum,
			consensus.BlockNum(),
		)

		panic("who?")
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

	if err := consensus.tryCatchup(); err != nil {
		return err
	}

	if consensus.Current.Mode() == ViewChanging {
		utils.Logger().Debug().Msg("[OnCommitted] Still in ViewChanging mode, Exiting!!")
		return nil
	}

	utils.Logger().Debug().Msg("[OnCommitted] Start consensus timer")
	consensus.Timeouts.Consensus.Start(consensus.BlockNum())
	return nil
}
