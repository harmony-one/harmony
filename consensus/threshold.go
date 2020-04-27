package consensus

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
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
		utils.Logger().Err(err).
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
	commitPayload := signature.ConstructCommitPayload(
		consensus.ChainReader,
		new(big.Int).SetUint64(consensus.epoch),
		consensus.blockHash, consensus.blockNum, consensus.viewID,
	)

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
			utils.Logger().Debug().Msg("[OnPrepare] Leader commit bitmap set failed")
			return err
		}
	}
	if err := consensus.host.SendMessageToGroups([]nodeconfig.GroupID{
		nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
	}, p2p.ConstructMessage(msgToSend),
	); err != nil {
		utils.Logger().Warn().Msg("[OnPrepare] Cannot send prepared message")
	} else {
		utils.Logger().Debug().
			Hex("blockHash", consensus.blockHash[:]).
			Uint64("blockNum", consensus.blockNum).
			Msg("[OnPrepare] Sent Prepared Message!!")
	}

	utils.Logger().Debug().
		Str("From", consensus.phase.String()).
		Str("To", FBFTCommit.String()).
		Msg("[OnPrepare] Switching phase")

	return nil
}
