package consensus

import (
	"github.com/ethereum/go-ethereum/rlp"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

func (consensus *Consensus) didReachPrepareQuorum() error {
	logger := utils.Logger()
	logger.Info().Msg("[OnPrepare] Received Enough Prepare Signatures")
	leaderPriKey, err := consensus.getConsensusLeaderPrivateKey()
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("[OnPrepare] leader not found")
		return err
	}
	// Construct and broadcast prepared message
	networkMessage, err := consensus.construct(
		msg_pb.MessageType_PREPARED, nil, []*bls.PrivateKeyWrapper{leaderPriKey},
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
	consensus.fBFTLog.AddVerifiedMessage(FBFTMsg)
	// Leader add commit phase signature
	var blockObj types.Block
	if err := rlp.DecodeBytes(consensus.block, &blockObj); err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Uint64("BlockNum", consensus.BlockNum()).
			Msg("[didReachPrepareQuorum] Unparseable block data")
		return err
	}
	commitPayload := signature.ConstructCommitPayload(consensus.Blockchain().Config(),
		blockObj.Epoch(), blockObj.Hash(), blockObj.NumberU64(), blockObj.Header().ViewID().Uint64())

	// so by this point, everyone has committed to the blockhash of this block
	// in prepare and so this is the actual block.
	for i, key := range consensus.priKey {
		if err := consensus.commitBitmap.SetKey(key.Pub.Bytes, true); err != nil {
			consensus.getLogger().Warn().Msgf("[OnPrepare] Leader commit bitmap set failed for key at index %d", i)
			continue
		}

		if _, err := consensus.decider.AddNewVote(
			quorum.Commit,
			[]*bls.PublicKeyWrapper{key.Pub},
			key.Pri.SignHash(commitPayload),
			blockObj.Hash(),
			blockObj.NumberU64(),
			blockObj.Header().ViewID().Uint64(),
		); err != nil {
			return err
		}
	}
	if err := consensus.msgSender.SendWithRetry(
		consensus.BlockNum(),
		msg_pb.MessageType_PREPARED, []nodeconfig.GroupID{
			nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
		},
		p2p.ConstructMessage(msgToSend),
	); err != nil {
		consensus.getLogger().Warn().Msg("[OnPrepare] Cannot send prepared message")
	} else {
		consensus.getLogger().Info().
			Hex("blockHash", consensus.blockHash[:]).
			Uint64("blockNum", consensus.BlockNum()).
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
