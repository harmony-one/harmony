package consensus

import (
	"bytes"
	"context"
	"encoding/hex"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	bls2 "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	vrf_bls "github.com/harmony-one/harmony/crypto/vrf/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/vdf/src/vdf_go"
	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var (
	errSenderPubKeyNotLeader  = errors.New("sender pubkey doesn't match leader")
	errVerifyMessageSignature = errors.New("verify message signature failed")
)

// timeout constant
const (
	// CommitSigSenderTimeout is the timeout for sending the commit sig to finish block proposal
	CommitSigSenderTimeout = 10 * time.Second
	// CommitSigReceiverTimeout is the timeout for the receiving side of the commit sig
	// if timeout, the receiver should instead ready directly from db for the commit sig
	CommitSigReceiverTimeout = 8 * time.Second
)

// IsViewChangingMode return true if curernt mode is viewchanging
func (consensus *Consensus) IsViewChangingMode() bool {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.isViewChangingMode()
}

func (consensus *Consensus) isViewChangingMode() bool {
	return consensus.current.Mode() == ViewChanging
}

// HandleMessageUpdate will update the consensus state according to received message
func (consensus *Consensus) HandleMessageUpdate(ctx context.Context, peer libp2p_peer.ID, msg *msg_pb.Message, senderKey *bls.SerializedPublicKey) error {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	// when node is in ViewChanging mode, it still accepts normal messages into FBFTLog
	// in order to avoid possible trap forever but drop PREPARE and COMMIT
	// which are message types specifically for a node acting as leader
	// so we just ignore those messages
	if consensus.isViewChangingMode() &&
		(msg.Type == msg_pb.MessageType_PREPARE ||
			msg.Type == msg_pb.MessageType_COMMIT) {
		return nil
	}

	// Do easier check before signature check
	if msg.Type == msg_pb.MessageType_ANNOUNCE || msg.Type == msg_pb.MessageType_PREPARED || msg.Type == msg_pb.MessageType_COMMITTED {
		// Only validator needs to check whether the message is from the correct leader
		if !bytes.Equal(senderKey[:], consensus.LeaderPubKey.Bytes[:]) &&
			consensus.current.Mode() == Normal && !consensus.IgnoreViewIDCheck.IsSet() {
			return errSenderPubKeyNotLeader
		}
	}

	if msg.Type != msg_pb.MessageType_PREPARE && msg.Type != msg_pb.MessageType_COMMIT {
		// Leader doesn't need to check validator's message signature since the consensus signature will be checked
		if !consensus.senderKeySanityChecks(msg, senderKey) {
			return errVerifyMessageSignature
		}
	}

	// Parse FBFT message
	var fbftMsg *FBFTMessage
	var err error
	switch t := msg.Type; true {
	case t == msg_pb.MessageType_VIEWCHANGE:
		fbftMsg, err = ParseViewChangeMessage(msg)
	case t == msg_pb.MessageType_NEWVIEW:
		members := consensus.decider.Participants()
		fbftMsg, err = ParseNewViewMessage(msg, members)
	default:
		fbftMsg, err = consensus.parseFBFTMessage(msg)
	}
	if err != nil || fbftMsg == nil {
		return errors.Wrapf(err, "unable to parse consensus msg with type: %s", msg.Type)
	}

	canHandleViewChange := true
	intendedForValidator, intendedForLeader :=
		!consensus.isLeader(),
		consensus.isLeader()

	// if in backup normal mode, force ignore view change event and leader event.
	if consensus.current.Mode() == NormalBackup {
		canHandleViewChange = false
		intendedForLeader = false
	}

	// Route message to handler
	switch t := msg.Type; true {
	// Handle validator intended messages first
	case t == msg_pb.MessageType_ANNOUNCE && intendedForValidator:
		consensus.onAnnounce(msg)
	case t == msg_pb.MessageType_PREPARED && intendedForValidator:
		consensus.onPrepared(fbftMsg)
	case t == msg_pb.MessageType_COMMITTED && intendedForValidator:
		consensus.onCommitted(fbftMsg)

	// Handle leader intended messages now
	case t == msg_pb.MessageType_PREPARE && intendedForLeader:
		consensus.onPrepare(fbftMsg)
	case t == msg_pb.MessageType_COMMIT && intendedForLeader:
		consensus.onCommit(fbftMsg)

	// Handle view change messages
	case t == msg_pb.MessageType_VIEWCHANGE && canHandleViewChange:
		consensus.onViewChange(fbftMsg)
	case t == msg_pb.MessageType_NEWVIEW && canHandleViewChange:
		consensus.onNewView(fbftMsg)
	}

	return nil
}

func (consensus *Consensus) finalCommit() {
	numCommits := consensus.decider.SignersCount(quorum.Commit)

	consensus.getLogger().Info().
		Int64("NumCommits", numCommits).
		Msg("[finalCommit] Finalizing Consensus")
	beforeCatchupNum := consensus.getBlockNum()

	leaderPriKey, err := consensus.getConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[finalCommit] leader not found")
		return
	}
	// Construct committed message
	network, err := consensus.construct(msg_pb.MessageType_COMMITTED, nil, []*bls.PrivateKeyWrapper{leaderPriKey})
	if err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[finalCommit] Unable to construct Committed message")
		return
	}
	var (
		msgToSend          = network.Bytes
		FBFTMsg            = network.FBFTMsg
		commitSigAndBitmap = FBFTMsg.Payload
	)
	consensus.fBFTLog.AddVerifiedMessage(FBFTMsg)
	// find correct block content
	curBlockHash := consensus.blockHash
	block := consensus.fBFTLog.GetBlockByHash(curBlockHash)
	if block == nil {
		consensus.getLogger().Warn().
			Str("blockHash", hex.EncodeToString(curBlockHash[:])).
			Msg("[finalCommit] Cannot find block by hash")
		return
	}

	if err := consensus.verifyLastCommitSig(commitSigAndBitmap, block); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[finalCommit] failed verifying last commit sig")
		return
	}
	consensus.getLogger().Info().Hex("new", commitSigAndBitmap).Msg("[finalCommit] Overriding commit signatures!!")

	if err := consensus.Blockchain().WriteCommitSig(block.NumberU64(), commitSigAndBitmap); err != nil {
		consensus.getLogger().Warn().Err(err).Msg("[finalCommit] failed writting commit sig")
	}

	// Send committed message before block insertion.
	// if leader successfully finalizes the block, send committed message to validators
	// Note: leader already sent 67% commit in preCommit. The 100% commit won't be sent immediately
	// to save network traffic. It will only be sent in retry if consensus doesn't move forward.
	// Or if the leader is changed for next block, the 100% committed sig will be sent to the next leader immediately.
	if !consensus.isLeader() || block.IsLastBlockInEpoch() {
		// send immediately
		if err := consensus.msgSender.SendWithRetry(
			block.NumberU64(),
			msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
			},
			p2p.ConstructMessage(msgToSend)); err != nil {
			consensus.getLogger().Warn().Err(err).Msg("[finalCommit] Cannot send committed message")
		} else {
			consensus.getLogger().Info().
				Hex("blockHash", curBlockHash[:]).
				Uint64("blockNum", consensus.BlockNum()).
				Msg("[finalCommit] Sent Committed Message")
		}
		consensus.getLogger().Info().Msg("[finalCommit] Start consensus timer")
		consensus.consensusTimeout[timeoutConsensus].Start()
	} else {
		// delayed send
		consensus.msgSender.DelayedSendWithRetry(
			block.NumberU64(),
			msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
			},
			p2p.ConstructMessage(msgToSend))
		consensus.getLogger().Info().
			Hex("blockHash", curBlockHash[:]).
			Uint64("blockNum", consensus.BlockNum()).
			Hex("lastCommitSig", commitSigAndBitmap).
			Msg("[finalCommit] Queued Committed Message")
	}

	block.SetCurrentCommitSig(commitSigAndBitmap)
	err = consensus.commitBlock(block, FBFTMsg)

	if err != nil || consensus.BlockNum()-beforeCatchupNum != 1 {
		consensus.getLogger().Err(err).
			Uint64("beforeCatchupBlockNum", beforeCatchupNum).
			Msg("[finalCommit] Leader failed to commit the confirmed block")
	}

	// Dump new block into level db
	// In current code, we add signatures in block in tryCatchup, the block dump to explorer does not contains signatures
	// but since explorer doesn't need signatures, it should be fine
	// in future, we will move signatures to next block
	//explorer.GetStorageInstance(consensus.leader.IP, consensus.leader.Port, true).Dump(block, beforeCatchupNum)

	if consensus.consensusTimeout[timeoutBootstrap].IsActive() {
		consensus.consensusTimeout[timeoutBootstrap].Stop()
		consensus.getLogger().Info().Msg("[finalCommit] stop bootstrap timer only once")
	}

	consensus.getLogger().Info().
		Uint64("blockNum", block.NumberU64()).
		Uint64("epochNum", block.Epoch().Uint64()).
		Uint64("ViewId", block.Header().ViewID().Uint64()).
		Str("blockHash", block.Hash().String()).
		Int("numTxns", len(block.Transactions())).
		Int("numStakingTxns", len(block.StakingTransactions())).
		Msg("HOORAY!!!!!!! CONSENSUS REACHED!!!!!!!")

	consensus.UpdateLeaderMetrics(float64(numCommits), float64(block.NumberU64()))

	// If still the leader, send commit sig/bitmap to finish the new block proposal,
	// else, the block proposal will timeout by itself.
	if consensus.isLeader() {
		if block.IsLastBlockInEpoch() {
			// No pipelining
			go func() {
				consensus.getLogger().Info().Msg("[finalCommit] sending block proposal signal")
				consensus.ReadySignal(SyncProposal)
			}()
		} else {
			// pipelining
			go func() {
				select {
				case consensus.GetCommitSigChannel() <- commitSigAndBitmap:
				case <-time.After(CommitSigSenderTimeout):
					utils.Logger().Error().Err(err).Msg("[finalCommit] channel not received after 6s for commitSigAndBitmap")
				}
			}()
		}
	}
}

// BlockCommitSigs returns the byte array of aggregated
// commit signature and bitmap signed on the block
func (consensus *Consensus) BlockCommitSigs(blockNum uint64) ([]byte, error) {
	if consensus.BlockNum() <= 1 {
		return nil, nil
	}
	lastCommits, err := consensus.Blockchain().ReadCommitSig(blockNum)
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	if err != nil ||
		len(lastCommits) < bls.BLSSignatureSizeInBytes {
		msgs := consensus.FBFTLog().GetMessagesByTypeSeq(
			msg_pb.MessageType_COMMITTED, blockNum,
		)
		if len(msgs) != 1 {
			consensus.getLogger().Error().
				Int("numCommittedMsg", len(msgs)).
				Msg("GetLastCommitSig failed with wrong number of committed message")
			return nil, errors.Errorf(
				"GetLastCommitSig failed with wrong number of committed message %d", len(msgs),
			)
		}
		lastCommits = msgs[0].Payload
	}

	return lastCommits, nil
}

// Start waits for the next new block and run consensus
func (consensus *Consensus) Start(
	stopChan chan struct{},
) {
	consensus.GetLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Consensus started")
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				consensus.Tick()
			}
		}
	}()

	consensus.mutex.Lock()
	consensus.consensusTimeout[timeoutBootstrap].Start()
	consensus.getLogger().Info().Msg("[ConsensusMainLoop] Start bootstrap timeout (only once)")
	// Set up next block due time.
	consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)
	consensus.mutex.Unlock()
}

func (consensus *Consensus) StartChannel() {
	consensus.mutex.Lock()
	consensus.isInitialLeader = consensus.isLeader()
	if consensus.isInitialLeader {
		consensus.start = true
		consensus.getLogger().Info().Time("time", time.Now()).Msg("[ConsensusMainLoop] Send ReadySignal")
		consensus.mutex.Unlock()
		consensus.ReadySignal(SyncProposal)
		return
	}
	consensus.mutex.Unlock()
}

func (consensus *Consensus) syncReadyChan() {
	consensus.getLogger().Info().Msg("[ConsensusMainLoop] syncReadyChan")
	if consensus.getBlockNum() < consensus.Blockchain().CurrentHeader().Number().Uint64()+1 {
		consensus.setBlockNum(consensus.Blockchain().CurrentHeader().Number().Uint64() + 1)
		consensus.setViewIDs(consensus.Blockchain().CurrentHeader().ViewID().Uint64() + 1)
		mode := consensus.updateConsensusInformation()
		consensus.current.SetMode(mode)
		consensus.getLogger().Info().Msg("[syncReadyChan] Start consensus timer")
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Info().Str("Mode", mode.String()).Msg("Node is IN SYNC")
		consensusSyncCounterVec.With(prometheus.Labels{"consensus": "in_sync"}).Inc()
	} else if consensus.mode() == Syncing {
		// Corner case where sync is triggered before `onCommitted` and there is a race
		// for block insertion between consensus and downloader.
		mode := consensus.updateConsensusInformation()
		consensus.setMode(mode)
		consensus.getLogger().Info().Msg("[syncReadyChan] Start consensus timer")
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensusSyncCounterVec.With(prometheus.Labels{"consensus": "in_sync"}).Inc()
	}
}

func (consensus *Consensus) syncNotReadyChan(reason string) {
	mode := consensus.current.Mode()
	consensus.setBlockNum(consensus.Blockchain().CurrentHeader().Number().Uint64() + 1)
	consensus.current.SetMode(Syncing)
	consensus.getLogger().Info().Msgf("[ConsensusMainLoop] syncNotReadyChan, prev %s, reason %s", mode.String(), reason)
	consensus.getLogger().Info().Msgf("[ConsensusMainLoop] Node is OUT OF SYNC, reason: %s", reason)
	consensusSyncCounterVec.With(prometheus.Labels{"consensus": "out_of_sync"}).Inc()
}

func (consensus *Consensus) Tick() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.tick()
}

func (consensus *Consensus) tick() {
	if !consensus.start && consensus.isInitialLeader {
		return
	}
	for k, v := range consensus.consensusTimeout {
		// stop timer in listening mode
		if consensus.current.Mode() == Listening {
			v.Stop()
			continue
		}

		if consensus.current.Mode() == Syncing {
			// never stop bootstrap timer here in syncing mode as it only starts once
			// if it is stopped, bootstrap will be stopped and nodes
			// can't start view change or join consensus
			// the bootstrap timer will be stopped once consensus is reached or view change
			// is succeeded
			if k != timeoutBootstrap {
				consensus.getLogger().Debug().
					Str("k", k.String()).
					Str("Mode", consensus.current.Mode().String()).
					Msg("[ConsensusMainLoop] consensusTimeout stopped!!!")
				v.Stop()
				continue
			}
		}
		if !v.Expired(time.Now()) {
			continue
		}
		if k != timeoutViewChange {
			consensus.getLogger().Warn().Msg("[ConsensusMainLoop] Ops Consensus Timeout!!!")
			consensus.startViewChange()
			break
		} else {
			consensus.getLogger().Warn().Msg("[ConsensusMainLoop] Ops View Change Timeout!!!")
			consensus.startViewChange()
			break
		}
	}
}

func (consensus *Consensus) BlockChannel(newBlock *types.Block) {
	consensus.GetLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Msg("[ConsensusMainLoop] Received Proposed New Block!")

	if newBlock.NumberU64() < consensus.BlockNum() {
		consensus.getLogger().Warn().Uint64("newBlockNum", newBlock.NumberU64()).
			Msg("[ConsensusMainLoop] received old block, abort")
		return
	}
	// Sleep to wait for the full block time
	consensus.GetLogger().Info().Msg("[ConsensusMainLoop] Waiting for Block Time")
	time.AfterFunc(time.Until(consensus.NextBlockDue), func() {
		consensus.StartFinalityCount()
		consensus.mutex.Lock()
		defer consensus.mutex.Unlock()
		// Update time due for next block
		consensus.NextBlockDue = time.Now().Add(consensus.BlockPeriod)

		startTime = time.Now()
		consensus.msgSender.Reset(newBlock.NumberU64())

		consensus.getLogger().Info().
			Int("numTxs", len(newBlock.Transactions())).
			Int("numStakingTxs", len(newBlock.StakingTransactions())).
			Time("startTime", startTime).
			Int64("publicKeys", consensus.decider.ParticipantsCount()).
			Msg("[ConsensusMainLoop] STARTING CONSENSUS")
		consensus.announce(newBlock)
	})
}

// LastMileBlockIter is the iterator to iterate over the last mile blocks in consensus cache.
// All blocks returned are guaranteed to pass the verification.
type LastMileBlockIter struct {
	blockCandidates []*types.Block
	fbftLog         *FBFTLog
	verify          func(*types.Block) error
	curIndex        int
	logger          *zerolog.Logger
}

// GetLastMileBlockIter get the iterator of the last mile blocks starting from number bnStart
func (consensus *Consensus) GetLastMileBlockIter(bnStart uint64, cb func(iter *LastMileBlockIter) error) error {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	return consensus.getLastMileBlockIter(bnStart, cb)
}

// GetLastMileBlockIter get the iterator of the last mile blocks starting from number bnStart
func (consensus *Consensus) getLastMileBlockIter(bnStart uint64, cb func(iter *LastMileBlockIter) error) error {
	blocks, _, err := consensus.getLastMileBlocksAndMsg(bnStart)
	if err != nil {
		return err
	}
	return cb(&LastMileBlockIter{
		blockCandidates: blocks,
		fbftLog:         consensus.fBFTLog,
		verify:          consensus.BlockVerifier,
		curIndex:        0,
		logger:          consensus.getLogger(),
	})
}

// Next iterate to the next last mile block
func (iter *LastMileBlockIter) Next() *types.Block {
	if iter.curIndex >= len(iter.blockCandidates) {
		return nil
	}
	block := iter.blockCandidates[iter.curIndex]
	iter.curIndex++

	if !iter.fbftLog.IsBlockVerified(block.Hash()) {
		if err := iter.verify(block); err != nil {
			iter.logger.Debug().Err(err).Msg("block verification failed in consensus last mile block")
			return nil
		}
		iter.fbftLog.MarkBlockVerified(block)
	}
	return block
}

func (consensus *Consensus) getLastMileBlocksAndMsg(bnStart uint64) ([]*types.Block, []*FBFTMessage, error) {
	var (
		blocks []*types.Block
		msgs   []*FBFTMessage
	)
	for blockNum := bnStart; ; blockNum++ {
		blk, msg, err := consensus.fBFTLog.GetCommittedBlockAndMsgsFromNumber(blockNum, consensus.getLogger())
		if err != nil {
			if err == errFBFTLogNotFound {
				break
			}
			return nil, nil, err
		}
		blocks = append(blocks, blk)
		msgs = append(msgs, msg)
	}
	return blocks, msgs, nil
}

// preCommitAndPropose commit the current block with 67% commit signatures and start
// proposing new block which will wait on the full commit signatures to finish
func (consensus *Consensus) preCommitAndPropose(blk *types.Block) error {
	if blk == nil {
		return errors.New("block to pre-commit is nil")
	}

	leaderPriKey, err := consensus.getConsensusLeaderPrivateKey()
	if err != nil {
		consensus.getLogger().Error().Err(err).Msg("[preCommitAndPropose] leader not found")
		return err
	}

	// Construct committed message
	network, err := consensus.construct(msg_pb.MessageType_COMMITTED, nil, []*bls.PrivateKeyWrapper{leaderPriKey})
	if err != nil {
		consensus.getLogger().Warn().Err(err).
			Msg("[preCommitAndPropose] Unable to construct Committed message")
		return err
	}

	msgToSend, FBFTMsg :=
		network.Bytes,
		network.FBFTMsg
	bareMinimumCommit := FBFTMsg.Payload
	consensus.fBFTLog.AddVerifiedMessage(FBFTMsg)

	if err := consensus.verifyLastCommitSig(bareMinimumCommit, blk); err != nil {
		return errors.Wrap(err, "[preCommitAndPropose] failed verifying last commit sig")
	}

	go func() {
		blk.SetCurrentCommitSig(bareMinimumCommit)

		// Send committed message to validators since 2/3 commit is already collected
		if err := consensus.msgSender.SendWithRetry(
			blk.NumberU64(),
			msg_pb.MessageType_COMMITTED, []nodeconfig.GroupID{
				nodeconfig.NewGroupIDByShardID(nodeconfig.ShardID(consensus.ShardID)),
			},
			p2p.ConstructMessage(msgToSend)); err != nil {
			consensus.GetLogger().Warn().Err(err).Msg("[preCommitAndPropose] Cannot send committed message")
		} else {
			consensus.GetLogger().Info().
				Str("blockHash", blk.Hash().Hex()).
				Uint64("blockNum", consensus.BlockNum()).
				Hex("lastCommitSig", bareMinimumCommit).
				Msg("[preCommitAndPropose] Sent Committed Message")
		}

		if _, err := consensus.Blockchain().InsertChain([]*types.Block{blk}, !consensus.FBFTLog().IsBlockVerified(blk.Hash())); err != nil {
			switch {
			case errors.Is(err, core.ErrKnownBlock):
				consensus.GetLogger().Info().Msg("[preCommitAndPropose] Block already known")
			default:
				consensus.GetLogger().Error().Err(err).Msg("[preCommitAndPropose] Failed to add block to chain")
				return
			}
		}
		consensus.mutex.Lock()
		consensus.getLogger().Info().Msg("[preCommitAndPropose] Start consensus timer")
		consensus.consensusTimeout[timeoutConsensus].Start()

		// Send signal to Node to propose the new block for consensus
		consensus.getLogger().Info().Msg("[preCommitAndPropose] sending block proposal signal")
		consensus.mutex.Unlock()
		consensus.ReadySignal(AsyncProposal)
	}()

	return nil
}

func (consensus *Consensus) verifyLastCommitSig(lastCommitSig []byte, blk *types.Block) error {
	if len(lastCommitSig) < bls.BLSSignatureSizeInBytes {
		return errors.New("lastCommitSig not have enough length")
	}

	aggSigBytes := lastCommitSig[0:bls.BLSSignatureSizeInBytes]

	aggSig := bls2.Sign{}
	err := aggSig.Deserialize(aggSigBytes)

	if err != nil {
		return errors.New("unable to deserialize multi-signature from payload")
	}
	aggPubKey := consensus.commitBitmap.AggregatePublic

	commitPayload := signature.ConstructCommitPayload(consensus.Blockchain().Config(),
		blk.Epoch(), blk.Hash(), blk.NumberU64(), blk.Header().ViewID().Uint64())

	if !aggSig.VerifyHash(aggPubKey, commitPayload) {
		return errors.New("Failed to verify the multi signature for last commit sig")
	}
	return nil
}

// tryCatchup add the last mile block in PBFT log memory cache to blockchain.
func (consensus *Consensus) tryCatchup() error {
	initBN := consensus.getBlockNum()
	defer consensus.postCatchup(initBN)

	blks, msgs, err := consensus.getLastMileBlocksAndMsg(initBN)
	if err != nil {
		return errors.Wrapf(err, "[TryCatchup] Failed to get last mile blocks: %v", err)
	}
	for i := range blks {
		blk, msg := blks[i], msgs[i]
		if blk == nil {
			return nil
		}
		blk.SetCurrentCommitSig(msg.Payload)

		if err := consensus.verifyBlock(blk); err != nil {
			consensus.getLogger().Err(err).Msg("[TryCatchup] failed block verifier")
			return err
		}
		consensus.getLogger().Info().Msg("[TryCatchup] Adding block to chain")
		if err := consensus.commitBlock(blk, msgs[i]); err != nil {
			consensus.getLogger().Error().Err(err).Msg("[TryCatchup] Failed to add block to chain")
			return err
		}
		select {
		// TODO: Remove this when removing dns sync and stream sync is fully up
		case consensus.VerifiedNewBlock <- blk:
		default:
			consensus.getLogger().Info().
				Str("blockHash", blk.Hash().String()).
				Msg("[TryCatchup] consensus verified block send to chan failed")
			continue
		}
	}
	return nil
}

func (consensus *Consensus) commitBlock(blk *types.Block, committedMsg *FBFTMessage) error {
	if consensus.Blockchain().CurrentBlock().NumberU64() < blk.NumberU64() {
		_, err := consensus.Blockchain().InsertChain([]*types.Block{blk}, !consensus.fBFTLog.IsBlockVerified(blk.Hash()))
		if err != nil && !errors.Is(err, core.ErrKnownBlock) {
			consensus.getLogger().Error().Err(err).Msg("[commitBlock] Failed to add block to chain")
			return err
		}
	}

	if !committedMsg.HasSingleSender() {
		consensus.getLogger().Error().Msg("[TryCatchup] Leader message can not have multiple sender keys")
		return errIncorrectSender
	}

	consensus.FinishFinalityCount()
	go func() {
		consensus.PostConsensusJob(blk)
	}()
	consensus.setupForNewConsensus(blk, committedMsg)
	utils.Logger().Info().Uint64("blockNum", blk.NumberU64()).
		Str("hash", blk.Header().Hash().Hex()).
		Msg("Added New Block to Blockchain!!!")

	return nil
}

// rotateLeader rotates the leader to the next leader in the committee.
// This function must be called with enabled leader rotation.
func (consensus *Consensus) rotateLeader(epoch *big.Int, defaultKey *bls.PublicKeyWrapper) *bls.PublicKeyWrapper {
	var (
		bc        = consensus.Blockchain()
		leader    = consensus.getLeaderPubKey()
		curBlock  = bc.CurrentBlock()
		curNumber = curBlock.NumberU64()
		curEpoch  = curBlock.Epoch().Uint64()
	)
	if epoch.Uint64() != curEpoch {
		return defaultKey
	}
	const blocksCountAliveness = 4
	utils.Logger().Info().Msgf("[Rotating leader] epoch: %v rotation:%v external rotation %v", epoch.Uint64(), bc.Config().IsLeaderRotationInternalValidators(epoch), bc.Config().IsLeaderRotationExternalValidatorsAllowed(epoch))
	ss, err := bc.ReadShardState(epoch)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to read shard state")
		return defaultKey
	}
	committee, err := ss.FindCommitteeByID(consensus.ShardID)
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to find committee")
		return defaultKey
	}
	slotsCount := len(committee.Slots)
	blocksPerEpoch := shard.Schedule.InstanceForEpoch(epoch).BlocksPerEpoch()
	if blocksPerEpoch == 0 {
		utils.Logger().Error().Msg("[Rotating leader] blocks per epoch is 0")
		return defaultKey
	}
	if slotsCount == 0 {
		utils.Logger().Error().Msg("[Rotating leader] slots count is 0")
		return defaultKey
	}
	numBlocksProducedByLeader := blocksPerEpoch / uint64(slotsCount)
	const minimumBlocksForLeaderInRow = blocksCountAliveness
	if numBlocksProducedByLeader < minimumBlocksForLeaderInRow {
		// mine no less than 3 blocks in a row
		numBlocksProducedByLeader = minimumBlocksForLeaderInRow
	}
	s := bc.LeaderRotationMeta()
	if !bytes.Equal(leader.Bytes[:], s.Pub) {
		// Another leader.
		return defaultKey
	}
	if s.Count < numBlocksProducedByLeader {
		// Not enough blocks produced by the leader, continue producing by the same leader.
		return defaultKey
	}
	// Passed all checks, we can change leader.
	// NthNext will move the leader to the next leader in the committee.
	// It does not know anything about external or internal validators.
	var (
		wasFound bool
		next     *bls.PublicKeyWrapper
		offset   = 1
	)

	for i := 0; i < len(committee.Slots); i++ {
		if bc.Config().IsLeaderRotationExternalValidatorsAllowed(epoch) {
			wasFound, next = consensus.decider.NthNextValidator(committee.Slots, leader, offset)
		} else {
			wasFound, next = consensus.decider.NthNextHmy(shard.Schedule.InstanceForEpoch(epoch), leader, offset)
		}
		if !wasFound {
			utils.Logger().Error().Msg("Failed to get next leader")
			// Seems like nothing we can do here.
			return defaultKey
		}
		members := consensus.decider.Participants()
		mask := bls.NewMask(members)
		skipped := 0
		for i := 0; i < blocksCountAliveness; i++ {
			header := bc.GetHeaderByNumber(curNumber - uint64(i))
			if header == nil {
				utils.Logger().Error().Msgf("Failed to get header by number %d", curNumber-uint64(i))
				return defaultKey
			}
			// if epoch is different, we should not check this block.
			if header.Epoch().Uint64() != curEpoch {
				break
			}
			// Populate the mask with the bitmap.
			err = mask.SetMask(header.LastCommitBitmap())
			if err != nil {
				utils.Logger().Err(err).Msg("Failed to set mask")
				return defaultKey
			}
			ok, err := mask.KeyEnabled(next.Bytes)
			if err != nil {
				utils.Logger().Err(err).Msg("Failed to get key enabled")
				return defaultKey
			}
			if !ok {
				skipped++
			}
		}

		// no signature from the next leader at all, we should skip it.
		if skipped >= blocksCountAliveness {
			// Next leader is not signing blocks, we should skip it.
			offset++
			continue
		}
		return next
	}
	return defaultKey
}

// SetupForNewConsensus sets the state for new consensus
func (consensus *Consensus) setupForNewConsensus(blk *types.Block, committedMsg *FBFTMessage) {
	atomic.StoreUint64(&consensus.blockNum, blk.NumberU64()+1)
	consensus.setCurBlockViewID(committedMsg.ViewID + 1)
	var epoch *big.Int
	if blk.IsLastBlockInEpoch() {
		epoch = new(big.Int).Add(blk.Epoch(), common.Big1)
	} else {
		epoch = blk.Epoch()
	}
	if consensus.Blockchain().Config().IsLeaderRotationInternalValidators(epoch) {
		if next := consensus.rotateLeader(epoch, committedMsg.SenderPubkeys[0]); next != nil {
			prev := consensus.getLeaderPubKey()
			consensus.setLeaderPubKey(next)
			if consensus.isLeader() {
				utils.Logger().Info().Msgf("We are block %d, I am the new leader %s", blk.NumberU64(), next.Bytes.Hex())
			} else {
				utils.Logger().Info().Msgf("We are block %d, the leader is %s", blk.NumberU64(), next.Bytes.Hex())
			}
			if consensus.isLeader() && !consensus.getLeaderPubKey().Object.IsEqual(prev.Object) {
				// leader changed
				blockPeriod := consensus.BlockPeriod
				go func() {
					<-time.After(blockPeriod)
					consensus.ReadySignal(SyncProposal)
				}()
			}
		}
	}

	// Update consensus keys at last so the change of leader status doesn't mess up normal flow
	if blk.IsLastBlockInEpoch() {
		consensus.setMode(consensus.updateConsensusInformation())
	}
	consensus.fBFTLog.PruneCacheBeforeBlock(blk.NumberU64())
	consensus.resetState()
}

func (consensus *Consensus) postCatchup(initBN uint64) {
	if initBN < consensus.getBlockNum() {
		consensus.getLogger().Info().
			Uint64("From", initBN).
			Uint64("To", consensus.getBlockNum()).
			Msg("[TryCatchup] Caught up!")
		consensus.switchPhase("TryCatchup", FBFTAnnounce)
	}
	// catch up and skip from view change trap
	if initBN < consensus.getBlockNum() && consensus.isViewChangingMode() {
		consensus.current.SetMode(Normal)
		consensus.consensusTimeout[timeoutViewChange].Stop()
	}
}

// GenerateVrfAndProof generates new VRF/Proof from hash of previous block
func (consensus *Consensus) GenerateVrfAndProof(newHeader *block.Header) error {
	key, err := consensus.getConsensusLeaderPrivateKey()
	if err != nil {
		return errors.New("[GenerateVrfAndProof] no leader private key provided")
	}
	sk := vrf_bls.NewVRFSigner(key.Pri)
	previousHeader := consensus.Blockchain().GetHeaderByNumber(
		newHeader.Number().Uint64() - 1,
	)
	if previousHeader == nil {
		return errors.New("[GenerateVrfAndProof] no parent header found")
	}

	previousHash := previousHeader.Hash()
	vrf, proof := sk.Evaluate(previousHash[:])
	if proof == nil {
		return errors.New("[GenerateVrfAndProof] failed to generate vrf")
	}

	newHeader.SetVrf(append(vrf[:], proof...))

	consensus.getLogger().Info().
		Uint64("BlockNum", newHeader.Number().Uint64()).
		Uint64("Epoch", newHeader.Epoch().Uint64()).
		Hex("VRF+Proof", newHeader.Vrf()).
		Msg("[GenerateVrfAndProof] Leader generated a VRF")

	return nil
}

// GenerateVdfAndProof generates new VDF/Proof from VRFs in the current epoch
func (consensus *Consensus) GenerateVdfAndProof(newBlock *types.Block, vrfBlockNumbers []uint64) {
	//derive VDF seed from VRFs generated in the current epoch
	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.Blockchain().GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	consensus.getLogger().Info().
		Uint64("MsgBlockNum", newBlock.NumberU64()).
		Uint64("Epoch", newBlock.Header().Epoch().Uint64()).
		Int("Num of VRF", len(vrfBlockNumbers)).
		Msg("[ConsensusMainLoop] VDF computation started")

	// TODO ek – limit concurrency
	go func() {
		vdf := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
		outputChannel := vdf.GetOutputChannel()
		start := time.Now()
		vdf.Execute()
		duration := time.Since(start)
		consensus.GetLogger().Info().
			Dur("duration", duration).
			Msg("[ConsensusMainLoop] VDF computation finished")
		output := <-outputChannel

		// The first 516 bytes are the VDF+proof and the last 32 bytes are XORed VRF as seed
		rndBytes := [548]byte{}
		copy(rndBytes[:516], output[:])
		copy(rndBytes[516:], seed[:])
		consensus.RndChannel <- rndBytes
	}()
}

// ValidateVdfAndProof validates the VDF/proof in the current epoch
func (consensus *Consensus) ValidateVdfAndProof(headerObj *block.Header) bool {
	vrfBlockNumbers, err := consensus.Blockchain().ReadEpochVrfBlockNums(headerObj.Epoch())
	if err != nil {
		consensus.getLogger().Error().Err(err).
			Str("MsgBlockNum", headerObj.Number().String()).
			Msg("[OnAnnounce] failed to read VRF block numbers for VDF computation")
	}

	//extra check to make sure there's no index out of range error
	//it can happen if epoch is messed up, i.e. VDF ouput is generated in the next epoch
	if consensus.VdfSeedSize() > len(vrfBlockNumbers) {
		return false
	}

	seed := [32]byte{}
	for i := 0; i < consensus.VdfSeedSize(); i++ {
		previousVrf := consensus.Blockchain().GetVrfByNumber(vrfBlockNumbers[i])
		for j := 0; j < len(seed); j++ {
			seed[j] = seed[j] ^ previousVrf[j]
		}
	}

	vdfObject := vdf_go.New(shard.Schedule.VdfDifficulty(), seed)
	vdfOutput := [516]byte{}
	copy(vdfOutput[:], headerObj.Vdf())
	if vdfObject.Verify(vdfOutput) {
		consensus.getLogger().Info().
			Str("MsgBlockNum", headerObj.Number().String()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] validated a new VDF")

	} else {
		consensus.getLogger().Warn().
			Str("MsgBlockNum", headerObj.Number().String()).
			Uint64("Epoch", headerObj.Epoch().Uint64()).
			Int("Num of VRF", consensus.VdfSeedSize()).
			Msg("[OnAnnounce] VDF proof is not valid")
		return false
	}

	return true
}

// DeleteBlocksLessThan deletes blocks less than given block number
func (consensus *Consensus) DeleteBlocksLessThan(number uint64) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.fBFTLog.deleteBlocksLessThan(number)
}

// DeleteMessagesLessThan deletes messages less than given block number.
func (consensus *Consensus) DeleteMessagesLessThan(number uint64) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.fBFTLog.deleteMessagesLessThan(number)
}
