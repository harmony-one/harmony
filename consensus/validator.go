package consensus

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"

	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/chain"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/p2p"
)

func (consensus *Consensus) onAnnounce(msg *msg_pb.Message) {
	recvMsg, err := consensus.parseFBFTMessage(msg)
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
	consensus.StartFinalityCount()

	consensus.getLogger().Info().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Msg("[OnAnnounce] Announce message Added")
	consensus.fBFTLog.AddVerifiedMessage(recvMsg)
	consensus.blockHash = recvMsg.BlockHash
	// we have already added message and block, skip check viewID
	// and send prepare message if is in ViewChanging mode
	if consensus.isViewChangingMode() {
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
	consensus.switchPhase("Announce", FBFTPrepare)

	if len(recvMsg.Block) > 0 {
		go func() {
			// Best effort check, no need to error out.
			_, err := consensus.ValidateNewBlock(recvMsg)
			if err == nil {
				consensus.GetLogger().Info().
					Msg("[Announce] Block verified")
			}
		}()
	}
}

func (consensus *Consensus) ValidateNewBlock(recvMsg *FBFTMessage) (*types.Block, error) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	return consensus.validateNewBlock(recvMsg)
}
func (consensus *Consensus) validateNewBlock(recvMsg *FBFTMessage) (*types.Block, error) {
	if consensus.fBFTLog.IsBlockVerified(recvMsg.BlockHash) {
		var blockObj *types.Block

		blockObj = consensus.fBFTLog.GetBlockByHash(recvMsg.BlockHash)
		if blockObj == nil {
			var blockObj2 types.Block
			if err := rlp.DecodeBytes(recvMsg.Block, &blockObj2); err != nil {
				consensus.getLogger().Warn().
					Err(err).
					Uint64("MsgBlockNum", recvMsg.BlockNum).
					Msg("[validateNewBlock] Unparseable block header data")
				return nil, errors.New("Failed parsing new block")
			}
			blockObj = &blockObj2
		}
		consensus.getLogger().Info().
			Msg("[validateNewBlock] Block Already verified")
		return blockObj, nil
	}
	// check validity of block if any
	var blockObj types.Block
	if err := rlp.DecodeBytes(recvMsg.Block, &blockObj); err != nil {
		consensus.getLogger().Warn().
			Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("[validateNewBlock] Unparseable block header data")
		return nil, errors.New("Failed parsing new block")
	}

	consensus.fBFTLog.AddBlock(&blockObj)

	// let this handle it own logs
	if !consensus.newBlockSanityChecks(&blockObj, recvMsg) {
		return nil, errors.New("new block failed sanity checks")
	}

	// add block field
	blockPayload := make([]byte, len(recvMsg.Block))
	copy(blockPayload[:], recvMsg.Block[:])
	consensus.block = blockPayload
	recvMsg.Block = []byte{} // save memory space
	consensus.fBFTLog.AddVerifiedMessage(recvMsg)
	consensus.getLogger().Debug().
		Uint64("MsgViewID", recvMsg.ViewID).
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Hex("blockHash", recvMsg.BlockHash[:]).
		Msg("[validateNewBlock] Prepared message and block added")

	if consensus.BlockVerifier == nil {
		consensus.getLogger().Debug().Msg("[validateNewBlock] consensus received message before init. Ignoring")
		return nil, errors.New("nil block verifier")
	}

	if err := consensus.verifyBlock(&blockObj); err != nil {
		consensus.getLogger().Error().Err(err).Msg("[validateNewBlock] Block verification failed")
		return nil, errors.Errorf("Block verification failed: %s", err.Error())
	}
	return &blockObj, nil
}

func (consensus *Consensus) prepare() {
	if consensus.IsBackup() {
		return
	}

	priKeys := consensus.getPriKeysInCommittee()

	p2pMsgs := consensus.constructP2pMessages(msg_pb.MessageType_PREPARE, nil, priKeys)

	if err := consensus.broadcastConsensusP2pMessages(p2pMsgs); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[OnAnnounce] Cannot send prepare message")
	} else {
		consensus.getLogger().Info().
			Str("blockHash", hex.EncodeToString(consensus.blockHash[:])).
			Msg("[OnAnnounce] Sent Prepare Message!!")
	}
}

// sendCommitMessages send out commit messages to leader
func (consensus *Consensus) sendCommitMessages(blockObj *types.Block) {
	if consensus.IsBackup() || blockObj == nil {
		return
	}

	priKeys := consensus.getPriKeysInCommittee()

	// Sign commit signature on the received block and construct the p2p messages
	commitPayload := signature.ConstructCommitPayload(consensus.Blockchain().Config(),
		blockObj.Epoch(), blockObj.Hash(), blockObj.NumberU64(), blockObj.Header().ViewID().Uint64())

	p2pMsgs := consensus.constructP2pMessages(msg_pb.MessageType_COMMIT, commitPayload, priKeys)

	if err := consensus.broadcastConsensusP2pMessages(p2pMsgs); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[sendCommitMessages] Cannot send commit message!!")
	} else {
		consensus.getLogger().Info().
			Uint64("blockNum", consensus.BlockNum()).
			Hex("blockHash", consensus.blockHash[:]).
			Msg("[sendCommitMessages] Sent Commit Message!!")
	}
}

// if onPrepared accepts the prepared message from the leader, then
// it will send a COMMIT message for the leader to receive on the network.
func (consensus *Consensus) onPrepared(recvMsg *FBFTMessage) {
	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnPrepared] Received prepared message")

	if recvMsg.BlockNum < consensus.BlockNum() {
		consensus.getLogger().Info().Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return
	}
	if recvMsg.BlockNum > consensus.BlockNum() {
		consensus.getLogger().Warn().
			Uint64("myBlockNum", consensus.BlockNum()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("myBlockHash", consensus.blockHash[:]).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Msgf("[OnPrepared] low consensus block number. Spin sync")
		consensus.spinUpStateSync()
	}

	// check validity of prepared signature
	blockHash := recvMsg.BlockHash
	aggSig, mask, err := consensus.readSignatureBitmapPayload(recvMsg.Payload, 0, consensus.Decider.Participants())
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("ReadSignatureBitmapPayload failed!")
		return
	}
	if !consensus.Decider.IsQuorumAchievedByMask(mask) {
		consensus.getLogger().Warn().Msgf("[OnPrepared] Quorum Not achieved.")
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

	var blockObj *types.Block
	if blockObj, err = consensus.validateNewBlock(recvMsg); err != nil {
		consensus.getLogger().Err(err).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("MsgViewID", recvMsg.ViewID).
			Msg("[OnPrepared] failed to verify new block")
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
	if recvMsg.BlockNum > consensus.BlockNum() {
		consensus.getLogger().Info().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Uint64("blockNum", consensus.BlockNum()).
			Msg("[OnPrepared] Future Block Received, ignoring!!")
		return
	}

	// add preparedSig field
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask

	// Optimistically add blockhash field of prepare message
	copy(consensus.blockHash[:], blockHash[:])

	// tryCatchup is also run in onCommitted(), so need to lock with commitMutex.
	if consensus.current.Mode() == Normal {
		consensus.sendCommitMessages(blockObj)
		consensus.switchPhase("onPrepared", FBFTCommit)
	} else {
		// don't sign the block that is not verified
		consensus.getLogger().Info().Msg("[OnPrepared] Not in normal mode, Exiting!!")
	}

	go func() {
		// Try process future committed messages and process them in case of receiving committed before prepared
		if blockObj == nil {
			return
		}
		curBlockNum := consensus.BlockNum()
		consensus.mutex.Lock()
		defer consensus.mutex.Unlock()
		for _, committedMsg := range consensus.fBFTLog.GetNotVerifiedCommittedMessages(blockObj.NumberU64(), blockObj.Header().ViewID().Uint64(), blockObj.Hash()) {
			if committedMsg != nil {
				consensus.onCommitted(committedMsg)
			}
			if curBlockNum < consensus.getBlockNum() {
				consensus.getLogger().Info().Msg("[OnPrepared] Successfully caught up with committed message")
				break
			}
		}
	}()
}

func (consensus *Consensus) onCommitted(recvMsg *FBFTMessage) {
	consensus.getLogger().Info().
		Uint64("MsgBlockNum", recvMsg.BlockNum).
		Uint64("MsgViewID", recvMsg.ViewID).
		Msg("[OnCommitted] Received committed message")

	// Ok to receive committed from last block since it could have more signatures
	if recvMsg.BlockNum < consensus.BlockNum()-1 {
		consensus.getLogger().Info().
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Msg("Wrong BlockNum Received, ignoring!")
		return
	}

	if recvMsg.BlockNum > consensus.BlockNum() {
		consensus.getLogger().Info().
			Uint64("myBlockNum", consensus.BlockNum()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("myBlockHash", consensus.blockHash[:]).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Msg("[OnCommitted] low consensus block number. Spin up state sync")
		consensus.spinUpStateSync()
	}

	// Optimistically add committedMessage in case of receiving committed before prepared
	consensus.fBFTLog.AddNotVerifiedMessage(recvMsg)

	// Must have the corresponding block to verify committed message.
	blockObj := consensus.fBFTLog.GetBlockByHash(recvMsg.BlockHash)
	if blockObj == nil {
		consensus.getLogger().Info().
			Uint64("blockNum", recvMsg.BlockNum).
			Uint64("viewID", recvMsg.ViewID).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommitted] Failed finding a matching block for committed message")
		return
	}
	sigBytes, bitmap, err := chain.ParseCommitSigAndBitmap(recvMsg.Payload)
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Uint64("blockNum", recvMsg.BlockNum).
			Uint64("viewID", recvMsg.ViewID).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommitted] Failed to parse commit sigBytes and bitmap")
		return
	}
	if err := consensus.Blockchain().Engine().VerifyHeaderSignature(consensus.Blockchain(), blockObj.Header(),
		sigBytes, bitmap); err != nil {
		consensus.getLogger().Error().
			Uint64("blockNum", recvMsg.BlockNum).
			Uint64("viewID", recvMsg.ViewID).
			Str("blockHash", recvMsg.BlockHash.Hex()).
			Msg("[OnCommitted] Failed to verify the multi signature for commit phase")
		return
	}

	aggSig, mask, err := chain.DecodeSigBitmap(sigBytes, bitmap, consensus.Decider.Participants())
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[OnCommitted] readSignatureBitmapPayload failed")
		return
	}
	consensus.fBFTLog.AddVerifiedMessage(recvMsg)
	consensus.aggregatedCommitSig = aggSig
	consensus.commitBitmap = mask

	// If we already have a committed signature received before, check whether the new one
	// has more signatures and if yes, override the old data.
	// Otherwise, simply write the commit signature in db.
	commitSigBitmap, err := consensus.Blockchain().ReadCommitSig(blockObj.NumberU64())
	// Need to check whether this block actually was committed, because it could be another block
	// with the same number that's committed and overriding its commit sigBytes is wrong.
	blk := consensus.Blockchain().GetBlockByHash(blockObj.Hash())
	if err == nil && len(commitSigBitmap) == len(recvMsg.Payload) && blk != nil {
		new := mask.CountEnabled()
		mask.SetMask(commitSigBitmap[bls.BLSSignatureSizeInBytes:])
		cur := mask.CountEnabled()
		if new > cur {
			consensus.getLogger().Info().Hex("old", commitSigBitmap).Hex("new", recvMsg.Payload).Msg("[OnCommitted] Overriding commit signatures!!")
			consensus.Blockchain().WriteCommitSig(blockObj.NumberU64(), recvMsg.Payload)
		}
	}

	initBn := consensus.BlockNum()
	consensus.tryCatchup()

	if recvMsg.BlockNum > consensus.BlockNum() {
		consensus.getLogger().Info().
			Uint64("myBlockNum", consensus.BlockNum()).
			Uint64("MsgBlockNum", recvMsg.BlockNum).
			Hex("myBlockHash", consensus.blockHash[:]).
			Hex("MsgBlockHash", recvMsg.BlockHash[:]).
			Msg("[OnCommitted] OUT OF SYNC")
		return
	}

	if consensus.isViewChangingMode() {
		consensus.getLogger().Info().Msg("[OnCommitted] Still in ViewChanging mode, Exiting!!")
		return
	}

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Debug().Msg("[OnCommitted] stop bootstrap timer only once")
	}

	if initBn < consensus.BlockNum() {
		consensus.getLogger().Info().Msg("[OnCommitted] Start consensus timer (new block added)")
		consensus.consensusTimeout[timeoutConsensus].Start()
	}
}

// Collect private keys that are part of the current committee.
// TODO: cache valid private keys and only update when keys change.
func (consensus *Consensus) getPriKeysInCommittee() []*bls.PrivateKeyWrapper {
	priKeys := []*bls.PrivateKeyWrapper{}
	for i, key := range consensus.priKey {
		if !consensus.isValidatorInCommittee(key.Pub.Bytes) {
			continue
		}
		priKeys = append(priKeys, &consensus.priKey[i])
	}
	return priKeys
}

func (consensus *Consensus) constructP2pMessages(msgType msg_pb.MessageType, payloadForSign []byte, priKeys []*bls.PrivateKeyWrapper) []*NetworkMessage {
	p2pMsgs := []*NetworkMessage{}
	if consensus.AggregateSig {
		networkMessage, err := consensus.construct(msgType, payloadForSign, priKeys)
		if err != nil {
			logger := consensus.getLogger().Err(err).
				Str("message-type", msgType.String())
			for _, key := range priKeys {
				logger.Str("key", key.Pri.SerializeToHexStr())
			}
			logger.Msg("could not construct message")
		} else {
			p2pMsgs = append(p2pMsgs, networkMessage)
		}

	} else {
		for _, key := range priKeys {
			networkMessage, err := consensus.construct(msgType, payloadForSign, []*bls.PrivateKeyWrapper{key})
			if err != nil {
				consensus.getLogger().Err(err).
					Str("message-type", msgType.String()).
					Str("key", key.Pri.SerializeToHexStr()).
					Msg("could not construct message")
				continue
			}

			p2pMsgs = append(p2pMsgs, networkMessage)
		}
	}
	return p2pMsgs
}

func (consensus *Consensus) broadcastConsensusP2pMessages(p2pMsgs []*NetworkMessage) error {
	groupID := []nodeconfig.GroupID{nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID))}

	for _, p2pMsg := range p2pMsgs {
		// TODO: this will not return immediately, may block
		if consensus.current.Mode() != Listening {
			if err := consensus.msgSender.SendWithoutRetry(
				groupID,
				p2p.ConstructMessage(p2pMsg.Bytes),
			); err != nil {
				return err
			}
		}
	}
	return nil
}
