package consensus

import (
	"encoding/binary"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/host"
)

func (consensus *Consensus) didReachPrepareQuorum() error {
	logger := utils.Logger()
	logger.Debug().Msg("[OnPrepare] Received Enough Prepare Signatures")
	// Construct and broadcast prepared message
	network, err := consensus.construct(msg_pb.MessageType_PREPARED, nil)
	if err != nil {
		consensus.getLogger().Err(err).
			Str("message-type", msg_pb.MessageType_PREPARED.String()).
			Msg("failed constructing message")
		return err
	}
	msgToSend, FBFTMsg, aggSig :=
		network.Bytes,
		network.FBFTMsg,
		network.OptionalAggregateSignature

	consensus.aggregatedPrepareSig = aggSig
	consensus.FBFTLog.AddMessage(FBFTMsg)
	// Leader add commit phase signature
	blockNumHash := make([]byte, 8)
	binary.LittleEndian.PutUint64(blockNumHash, consensus.blockNum)
	commitPayload := append(blockNumHash, consensus.blockHash[:]...)

	// so by this point, everyone has committed to the blockhash of this block
	// in prepare and so this is the actual block.

	consensus.Decider.SubmitVote(
		quorum.Commit,
		consensus.PubKey,
		consensus.priKey.SignHash(commitPayload),
		consensus.block[:],
	)

	if err := consensus.commitBitmap.SetKey(consensus.PubKey, true); err != nil {
		consensus.getLogger().Debug().Msg("[OnPrepare] Leader commit bitmap set failed")
		return err
	}

	if err := consensus.msgSender.SendWithRetry(
		consensus.blockNum,
		msg_pb.MessageType_PREPARED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		host.ConstructP2pMessage(byte(17), msgToSend),
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
