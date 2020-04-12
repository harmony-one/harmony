package consensus

import (
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

func (consensus *Consensus) didReachPrepareQuorum() error {
	logger := utils.Logger()
	logger.Debug().Msg("[OnPrepare] Received Enough Prepare Signatures")
	leaderPriKey, err := consensus.GetConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[OnPrepare] leader not found")
		return err
	}
	// Construct and broadcast prepared message
	networkMessage, err := consensus.construct(
		msg_pb.MessageType_PREPARED, nil, consensus.LeaderPubKey, leaderPriKey,
	)
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_PREPARED.String()).
			Msg("failed constructing message")
		return err
	}
	msgToSend, FBFTMsg, aggSig :=
		networkMessage.Bytes,
		networkMessage.FBFTMsg,
		networkMessage.OptionalAggregateSignature

	consensus.aggregatedPrepareSig = aggSig
	consensus.FBFTLog.AddMessage(FBFTMsg)
	// Leader add commit phase signature
	// TODO(audit): sign signature on hash+blockNum+viewID (add a hard fork)
	blockNumHash := [8]byte{}
	binary.LittleEndian.PutUint64(blockNumHash[:], consensus.blockNum)
	commitPayload := append(blockNumHash[:], consensus.blockHash[:]...)

	// so by this point, everyone has committed to the blockhash of this block
	// in prepare and so this is the actual block.
	for i, key := range consensus.PubKey.PublicKey {
		if _, err := consensus.Decider.SubmitVote(
			quorum.Commit,
			key,
			consensus.priKey.PrivateKey[i].SignHash(commitPayload),
			common.BytesToHash(consensus.blockHash[:]),
			consensus.blockNum,
			consensus.viewID,
		); err != nil {
			return err
		}

		if err := consensus.commitBitmap.SetKey(key, true); err != nil {
			consensus.getLogger().Debug().Msg("[OnPrepare] Leader commit bitmap set failed")
			return err
		}
	}
	if err := consensus.msgSender.SendWithRetry(
		consensus.blockNum,
		msg_pb.MessageType_PREPARED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		p2p.ConstructMessage(msgToSend),
	); err != nil {
		consensus.getLogger().Warn().Msg("[OnPrepare] Cannot send prepared message")
	} else {
		consensus.getLogger().Debug().
			Hex("blockHash", consensus.blockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepare] Sent Prepared Message!!")
	}
	consensus.msgSender.StopRetry(msg_pb.MessageType_ANNOUNCE)
	// Stop retry committed msg of last consensus
	consensus.msgSender.StopRetry(msg_pb.MessageType_COMMITTED)

	consensus.getLogger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTCommit.String()).
		Msg("[OnPrepare] Switching phase")

	return nil
}
