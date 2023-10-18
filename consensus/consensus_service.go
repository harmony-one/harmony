package consensus

import (
	"math/big"
	"sync/atomic"
	"time"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/webhooks"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

// WaitForNewRandomness listens to the RndChannel to receive new VDF randomness.
func (consensus *Consensus) WaitForNewRandomness() {
	go func() {
		for {
			vdfOutput := <-consensus.RndChannel
			consensus.pendingRnds = append(consensus.pendingRnds, vdfOutput)
		}
	}()
}

// GetNextRnd returns the oldest available randomness along with the hash of the block there randomness preimage is committed.
func (consensus *Consensus) GetNextRnd() ([vdFAndProofSize]byte, [32]byte, error) {
	if len(consensus.pendingRnds) == 0 {
		return [vdFAndProofSize]byte{}, [32]byte{}, errors.New("No available randomness")
	}
	vdfOutput := consensus.pendingRnds[0]

	vdfBytes := [vdFAndProofSize]byte{}
	seed := [32]byte{}
	copy(vdfBytes[:], vdfOutput[:vdFAndProofSize])
	copy(seed[:], vdfOutput[vdFAndProofSize:])

	//pop the first vdfOutput from the list
	consensus.pendingRnds = consensus.pendingRnds[1:]

	return vdfBytes, seed, nil
}

var (
	empty = []byte{}
)

// Signs the consensus message and returns the marshaled message.
func (consensus *Consensus) signAndMarshalConsensusMessage(message *msg_pb.Message,
	priKey *bls_core.SecretKey) ([]byte, error) {
	if err := consensus.signConsensusMessage(message, priKey); err != nil {
		return empty, err
	}
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return empty, err
	}
	return marshaledMessage, nil
}

// UpdatePublicKeys updates the PublicKeys for
// quorum on current subcommittee, protected by a mutex
func (consensus *Consensus) UpdatePublicKeys(pubKeys, allowlist []bls_cosi.PublicKeyWrapper) int64 {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	return consensus.updatePublicKeys(pubKeys, allowlist)
}

func (consensus *Consensus) updatePublicKeys(pubKeys, allowlist []bls_cosi.PublicKeyWrapper) int64 {
	consensus.Decider.UpdateParticipants(pubKeys, allowlist)
	consensus.getLogger().Info().Msg("My Committee updated")
	for i := range pubKeys {
		consensus.getLogger().Info().
			Int("index", i).
			Str("BLSPubKey", pubKeys[i].Bytes.Hex()).
			Msg("Member")
	}

	allKeys := consensus.Decider.Participants()
	if len(allKeys) != 0 {
		consensus.LeaderPubKey = &allKeys[0]
		consensus.getLogger().Info().
			Str("info", consensus.LeaderPubKey.Bytes.Hex()).Msg("Setting leader as first validator, because provided new keys")
	} else {
		consensus.getLogger().Error().
			Msg("[UpdatePublicKeys] Participants is empty")
	}
	for i := range pubKeys {
		consensus.getLogger().Info().
			Int("index", i).
			Str("BLSPubKey", pubKeys[i].Bytes.Hex()).
			Msg("Member")
	}
	// reset states after update public keys
	// TODO: incorporate bitmaps in the decider, so their state can't be inconsistent.
	consensus.updateBitmaps()
	consensus.resetState()

	// do not reset view change state if it is in view changing mode
	if !consensus.isViewChangingMode() {
		consensus.resetViewChangeState()
	}
	return consensus.Decider.ParticipantsCount()
}

// Sign on the hash of the message
func (consensus *Consensus) signMessage(message []byte, priKey *bls_core.SecretKey) []byte {
	hash := hash.Keccak256(message)
	signature := priKey.SignHash(hash[:])
	return signature.Serialize()
}

// Sign on the consensus message signature field.
func (consensus *Consensus) signConsensusMessage(message *msg_pb.Message,
	priKey *bls_core.SecretKey) error {
	message.Signature = nil
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage, priKey)
	message.Signature = signature
	return nil
}

// UpdateBitmaps update the bitmaps for prepare and commit phase
func (consensus *Consensus) updateBitmaps() {
	consensus.getLogger().Debug().
		Str("MessageType", consensus.phase.String()).
		Msg("[UpdateBitmaps] Updating consensus bitmaps")
	members := consensus.Decider.Participants()
	prepareBitmap := bls_cosi.NewMask(members)
	commitBitmap := bls_cosi.NewMask(members)
	multiSigBitmap := bls_cosi.NewMask(members)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap
	consensus.multiSigBitmap = multiSigBitmap

}

// ResetState resets the state of the consensus
func (consensus *Consensus) resetState() {
	consensus.switchPhase("ResetState", FBFTAnnounce)

	consensus.blockHash = [32]byte{}
	consensus.block = []byte{}
	consensus.Decider.ResetPrepareAndCommitVotes()
	if consensus.prepareBitmap != nil {
		consensus.prepareBitmap.Clear()
	}
	if consensus.commitBitmap != nil {
		consensus.commitBitmap.Clear()
	}
	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (consensus *Consensus) IsValidatorInCommittee(pubKey bls.SerializedPublicKey) bool {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.isValidatorInCommittee(pubKey)
}

func (consensus *Consensus) isValidatorInCommittee(pubKey bls.SerializedPublicKey) bool {
	return consensus.Decider.IndexOf(pubKey) != -1
}

// SetMode sets the mode of consensus
func (consensus *Consensus) SetMode(m Mode) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.setMode(m)
}

// SetMode sets the mode of consensus
func (consensus *Consensus) setMode(m Mode) {
	if m == Normal && consensus.isBackup {
		m = NormalBackup
	}

	consensus.getLogger().Debug().
		Str("Mode", m.String()).
		Msg("[SetMode]")
	consensus.current.SetMode(m)
}

// SetIsBackup sets the mode of consensus
func (consensus *Consensus) SetIsBackup(isBackup bool) {
	consensus.getLogger().Debug().
		Bool("IsBackup", isBackup).
		Msg("[SetIsBackup]")
	consensus.isBackup = isBackup
	consensus.current.SetIsBackup(isBackup)
}

// Mode returns the mode of consensus
func (consensus *Consensus) Mode() Mode {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.mode()
}

// mode returns the mode of consensus
func (consensus *Consensus) mode() Mode {
	return consensus.current.Mode()
}

// RegisterPRndChannel registers the channel for receiving randomness preimage from DRG protocol
func (consensus *Consensus) RegisterPRndChannel(pRndChannel chan []byte) {
	consensus.PRndChannel = pRndChannel
}

// RegisterRndChannel registers the channel for receiving final randomness from DRG protocol
func (consensus *Consensus) RegisterRndChannel(rndChannel chan [548]byte) {
	consensus.RndChannel = rndChannel
}

// Check viewID, caller's responsibility to hold lock when change ignoreViewIDCheck
func (consensus *Consensus) checkViewID(msg *FBFTMessage) error {
	// just ignore consensus check for the first time when node join
	if consensus.IgnoreViewIDCheck.IsSet() {
		//in syncing mode, node accepts incoming messages without viewID/leaderKey checking
		//so only set mode to normal when new node enters consensus and need checking viewID
		consensus.setMode(Normal)
		consensus.setViewIDs(msg.ViewID)
		if !msg.HasSingleSender() {
			return errors.New("Leader message can not have multiple sender keys")
		}
		consensus.LeaderPubKey = msg.SenderPubkeys[0]
		consensus.IgnoreViewIDCheck.UnSet()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Info().
			Str("leaderKey", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("[checkViewID] Start consensus timer")
		return nil
	} else if msg.ViewID > consensus.getCurBlockViewID() {
		return consensus_engine.ErrViewIDNotMatch
	} else if msg.ViewID < consensus.getCurBlockViewID() {
		return errors.New("view ID belongs to the past")
	}
	return nil
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (consensus *Consensus) SetBlockNum(blockNum uint64) {
	atomic.StoreUint64(&consensus.blockNum, blockNum)
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (consensus *Consensus) setBlockNum(blockNum uint64) {
	atomic.StoreUint64(&consensus.blockNum, blockNum)
}

// ReadSignatureBitmapPayload read the payload for signature and bitmap; offset is the beginning position of reading
func (consensus *Consensus) ReadSignatureBitmapPayload(recvPayload []byte, offset int) (*bls_core.Sign, *bls_cosi.Mask, error) {
	consensus.mutex.RLock()
	members := consensus.Decider.Participants()
	consensus.mutex.RUnlock()
	return consensus.readSignatureBitmapPayload(recvPayload, offset, members)
}

func (consensus *Consensus) readSignatureBitmapPayload(recvPayload []byte, offset int, members multibls.PublicKeys) (*bls_core.Sign, *bls_cosi.Mask, error) {
	if offset+bls.BLSSignatureSizeInBytes > len(recvPayload) {
		return nil, nil, errors.New("payload not have enough length")
	}
	sigAndBitmapPayload := recvPayload[offset:]

	// TODO(audit): keep a Mask in the Decider so it won't be reconstructed on the fly.
	return chain.ReadSignatureBitmapByPublicKeys(
		sigAndBitmapPayload, members,
	)
}

// UpdateConsensusInformation will update shard information (epoch, publicKeys, blockNum, viewID)
// based on the local blockchain. It is called in two cases for now:
// 1. consensus object initialization. because of current dependency where chainreader is only available
// after node is initialized; node is only available after consensus is initialized
// we need call this function separately after create consensus object
// 2. after state syncing is finished
// It will return the mode:
// (a) node not in committed: Listening mode
// (b) node in committed but has any err during processing: Syncing mode
// (c) node in committed and everything looks good: Normal mode
func (consensus *Consensus) UpdateConsensusInformation() Mode {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	return consensus.updateConsensusInformation()
}

func (consensus *Consensus) updateConsensusInformation() Mode {
	curHeader := consensus.Blockchain().CurrentHeader()
	curEpoch := curHeader.Epoch()
	nextEpoch := new(big.Int).Add(curHeader.Epoch(), common.Big1)

	// Overwrite nextEpoch if the shard state has a epoch number
	if curHeader.IsLastBlockInEpoch() {
		nextShardState, err := curHeader.GetShardState()
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("[UpdateConsensusInformation] Error retrieving current shard state in the first block")
			return Syncing
		}
		if nextShardState.Epoch != nil {
			nextEpoch = nextShardState.Epoch
		}
	}

	consensus.BlockPeriod = 5 * time.Second

	// Enable 2s block time at the twoSecondsEpoch
	if consensus.Blockchain().Config().IsTwoSeconds(nextEpoch) {
		consensus.BlockPeriod = 2 * time.Second
	}

	isFirstTimeStaking := consensus.Blockchain().Config().IsStaking(nextEpoch) &&
		curHeader.IsLastBlockInEpoch() && !consensus.Blockchain().Config().IsStaking(curEpoch)
	haventUpdatedDecider := consensus.Blockchain().Config().IsStaking(curEpoch) &&
		consensus.Decider.Policy() != quorum.SuperMajorityStake

	// Only happens once, the flip-over to a new Decider policy
	if isFirstTimeStaking || haventUpdatedDecider {
		decider := quorum.NewDecider(quorum.SuperMajorityStake, consensus.ShardID)
		consensus.Decider = decider
	}

	var committeeToSet *shard.Committee
	epochToSet := curEpoch
	hasError := false
	curShardState, err := committee.WithStakingEnabled.ReadFromDB(
		curEpoch, consensus.Blockchain(),
	)
	if err != nil {
		consensus.getLogger().Error().
			Err(err).
			Uint32("shard", consensus.ShardID).
			Msg("[UpdateConsensusInformation] Error retrieving current shard state")
		return Syncing
	}

	consensus.getLogger().Info().Msg("[UpdateConsensusInformation] Updating.....")
	// genesis block is a special case that will have shard state and needs to skip processing
	isNotGenesisBlock := curHeader.Number().Cmp(big.NewInt(0)) > 0
	if curHeader.IsLastBlockInEpoch() && isNotGenesisBlock {

		nextShardState, err := committee.WithStakingEnabled.ReadFromDB(
			nextEpoch, consensus.Blockchain(),
		)
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("Error retrieving nextEpoch shard state")
			return Syncing
		}

		subComm, err := nextShardState.FindCommitteeByID(curHeader.ShardID())
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("Error retrieving nextEpoch shard state")
			return Syncing
		}

		committeeToSet = subComm
		epochToSet = nextEpoch
	} else {
		subComm, err := curShardState.FindCommitteeByID(curHeader.ShardID())
		if err != nil {
			consensus.getLogger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("Error retrieving current shard state")
			return Syncing
		}

		committeeToSet = subComm
	}

	if len(committeeToSet.Slots) == 0 {
		consensus.getLogger().Warn().
			Msg("[UpdateConsensusInformation] No members in the committee to update")
		hasError = true
	}

	// update public keys in the committee
	oldLeader := consensus.LeaderPubKey
	pubKeys, _ := committeeToSet.BLSPublicKeys()

	consensus.getLogger().Info().
		Int("numPubKeys", len(pubKeys)).
		Msg("[UpdateConsensusInformation] Successfully updated public keys")
	consensus.updatePublicKeys(pubKeys, shard.Schedule.InstanceForEpoch(nextEpoch).ExternalAllowlist())

	// Update voters in the committee
	if _, err := consensus.Decider.SetVoters(
		committeeToSet, epochToSet,
	); err != nil {
		consensus.getLogger().Error().
			Err(err).
			Uint32("shard", consensus.ShardID).
			Msg("Error when updating voters")
		return Syncing
	}

	// take care of possible leader change during the epoch
	// TODO: in a very rare case, when a M1 view change happened, the block contains coinbase for last leader
	// but the new leader is actually recognized by most of the nodes. At this time, if a node sync to this
	// exact block and set its leader, it will set with the failed leader as in the coinbase of the block.
	// This is a very rare case scenario and not likely to cause any issue in mainnet. But we need to think about
	// a solution to take care of this case because the coinbase of the latest block doesn't really represent the
	// the real current leader in case of M1 view change.
	if !curHeader.IsLastBlockInEpoch() && curHeader.Number().Uint64() != 0 {
		leaderPubKey, err := chain.GetLeaderPubKeyFromCoinbase(consensus.Blockchain(), curHeader)
		if err != nil || leaderPubKey == nil {
			consensus.getLogger().Error().Err(err).
				Msg("[UpdateConsensusInformation] Unable to get leaderPubKey from coinbase")
			consensus.IgnoreViewIDCheck.Set()
			hasError = true
		} else {
			consensus.getLogger().Info().
				Str("leaderPubKey", leaderPubKey.Bytes.Hex()).
				Msg("[UpdateConsensusInformation] Most Recent LeaderPubKey Updated Based on BlockChain")
			consensus.LeaderPubKey = leaderPubKey
		}
	}

	for _, key := range pubKeys {
		// in committee
		myPubKeys := consensus.getPublicKeys()
		if myPubKeys.Contains(key.Object) {
			if hasError {
				consensus.getLogger().Error().
					Str("myKey", myPubKeys.SerializeToHexStr()).
					Msg("[UpdateConsensusInformation] hasError")

				return Syncing
			}

			// If the leader changed and I myself become the leader
			if (oldLeader != nil && consensus.LeaderPubKey != nil &&
				!consensus.LeaderPubKey.Object.IsEqual(oldLeader.Object)) && consensus.isLeader() {
				go func() {
					consensus.GetLogger().Info().
						Str("myKey", myPubKeys.SerializeToHexStr()).
						Msg("[UpdateConsensusInformation] I am the New Leader")
					consensus.ReadySignal(SyncProposal)
				}()
			}
			return Normal
		}
	}
	consensus.getLogger().Info().
		Msg("[UpdateConsensusInformation] not in committee, Listening")

	// not in committee
	return Listening
}

// IsLeader check if the node is a leader or not by comparing the public key of
// the node with the leader public key
func (consensus *Consensus) IsLeader() bool {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()

	return consensus.isLeader()
}

// isLeader check if the node is a leader or not by comparing the public key of
// the node with the leader public key. This function assume it runs under lock.
func (consensus *Consensus) isLeader() bool {
	obj := consensus.LeaderPubKey.Object
	for _, key := range consensus.priKey {
		if key.Pub.Object.IsEqual(obj) {
			return true
		}
	}
	return false
}

// SetViewIDs set both current view ID and view changing ID to the height
// of the blockchain. It is used during client startup to recover the state
func (consensus *Consensus) SetViewIDs(height uint64) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.setViewIDs(height)
}

// SetViewIDs set both current view ID and view changing ID to the height
// of the blockchain. It is used during client startup to recover the state
func (consensus *Consensus) setViewIDs(height uint64) {
	consensus.setCurBlockViewID(height)
	consensus.setViewChangingID(height)
}

// SetCurBlockViewID set the current view ID
func (consensus *Consensus) SetCurBlockViewID(viewID uint64) uint64 {
	return consensus.current.SetCurBlockViewID(viewID)
}

// SetCurBlockViewID set the current view ID
func (consensus *Consensus) setCurBlockViewID(viewID uint64) {
	consensus.current.SetCurBlockViewID(viewID)
}

// SetViewChangingID set the current view change ID
func (consensus *Consensus) SetViewChangingID(viewID uint64) {
	consensus.current.SetViewChangingID(viewID)
}

// SetViewChangingID set the current view change ID
func (consensus *Consensus) setViewChangingID(viewID uint64) {
	consensus.current.SetViewChangingID(viewID)
}

// StartFinalityCount set the finality counter to current time
func (consensus *Consensus) StartFinalityCount() {
	consensus.finalityCounter.Store(time.Now().UnixNano())
}

// FinishFinalityCount calculate the current finality
func (consensus *Consensus) FinishFinalityCount() {
	d := time.Now().UnixNano()
	if prior, ok := consensus.finalityCounter.Load().(int64); ok {
		consensus.finality = (d - prior) / 1000000
		consensusFinalityHistogram.Observe(float64(consensus.finality))
	}
}

// GetFinality returns the finality time in milliseconds of previous consensus
func (consensus *Consensus) GetFinality() int64 {
	return consensus.finality
}

// switchPhase will switch FBFTPhase to desired phase.
func (consensus *Consensus) switchPhase(subject string, desired FBFTPhase) {
	consensus.getLogger().Info().
		Str("from:", consensus.phase.String()).
		Str("to:", desired.String()).
		Str("switchPhase:", subject)

	consensus.phase = desired
}

var (
	errGetPreparedBlock  = errors.New("failed to get prepared block for self commit")
	errReadBitmapPayload = errors.New("failed to read signature bitmap payload")
)

// selfCommit will create a commit message and commit it locally
// it is used by the new leadder of the view change routine
// when view change is succeeded and the new leader
// received prepared payload from other validators or from local
func (consensus *Consensus) selfCommit(payload []byte) error {
	var blockHash [32]byte
	copy(blockHash[:], payload[:32])

	// Leader sign and add commit message
	block := consensus.fBFTLog.GetBlockByHash(blockHash)
	if block == nil {
		return errGetPreparedBlock
	}

	aggSig, mask, err := consensus.readSignatureBitmapPayload(payload, 32, consensus.Decider.Participants())
	if err != nil {
		return errReadBitmapPayload
	}

	// Have to keep the block hash so the leader can finish the commit phase of prepared block
	consensus.resetState()

	copy(consensus.blockHash[:], blockHash[:])
	consensus.switchPhase("selfCommit", FBFTCommit)
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask
	commitPayload := signature.ConstructCommitPayload(consensus.ChainReader().Config(),
		block.Epoch(), block.Hash(), block.NumberU64(), block.Header().ViewID().Uint64())
	for i, key := range consensus.priKey {
		if err := consensus.commitBitmap.SetKey(key.Pub.Bytes, true); err != nil {
			consensus.getLogger().Error().
				Err(err).
				Int("Index", i).
				Str("Key", key.Pub.Bytes.Hex()).
				Msg("[selfCommit] New Leader commit bitmap set failed")
			continue
		}

		if _, err := consensus.Decider.AddNewVote(
			quorum.Commit,
			[]*bls_cosi.PublicKeyWrapper{key.Pub},
			key.Pri.SignHash(commitPayload),
			common.BytesToHash(consensus.blockHash[:]),
			block.NumberU64(),
			block.Header().ViewID().Uint64(),
		); err != nil {
			consensus.getLogger().Warn().
				Err(err).
				Int("Index", i).
				Str("Key", key.Pub.Bytes.Hex()).
				Msg("[selfCommit] submit vote on viewchange commit failed")
		}
	}
	return nil
}

// NumSignaturesIncludedInBlock returns the number of signatures included in the block
func (consensus *Consensus) NumSignaturesIncludedInBlock(block *types.Block) uint32 {
	count := uint32(0)
	members := consensus.Decider.Participants()
	// TODO(audit): do not reconstruct the Mask
	mask := bls.NewMask(members)
	err := mask.SetMask(block.Header().LastCommitBitmap())
	if err != nil {
		return count
	}
	for _, key := range consensus.GetPublicKeys() {
		if ok, err := mask.KeyEnabled(key.Bytes); err == nil && ok {
			count++
		}
	}
	return count
}

// GetLogger returns logger for consensus contexts added.
func (consensus *Consensus) GetLogger() *zerolog.Logger {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.getLogger()
}

// getLogger returns logger for consensus contexts added
func (consensus *Consensus) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Uint64("myBlock", consensus.blockNum).
		Uint64("myViewID", consensus.getCurBlockViewID()).
		Str("phase", consensus.phase.String()).
		Str("mode", consensus.current.Mode().String()).
		Logger()
	return &logger
}

// VerifyNewBlock is called by consensus participants to verify the block (account model) they are
// running consensus on.
func VerifyNewBlock(hooks *webhooks.Hooks, blockChain core.BlockChain, beaconChain core.BlockChain) func(*types.Block) error {
	return func(newBlock *types.Block) error {
		if err := blockChain.ValidateNewBlock(newBlock, beaconChain); err != nil {
			switch {
			case errors.Is(err, core.ErrKnownBlock):
				return nil
			default:
			}

			if hooks := hooks; hooks != nil {
				if p := hooks.ProtocolIssues; p != nil {
					url := p.OnCannotCommit
					go func() {
						webhooks.DoPost(url, map[string]interface{}{
							"bad-header": newBlock.Header(),
							"reason":     err.Error(),
						})
					}()
				}
			}
			utils.Logger().Error().
				Str("blockHash", newBlock.Hash().Hex()).
				Int("numTx", len(newBlock.Transactions())).
				Int("numStakingTx", len(newBlock.StakingTransactions())).
				Err(err).
				Msgf("[VerifyNewBlock] Cannot Verify New Block!!!, blockHeight %d, myHeight %d", newBlock.NumberU64(), blockChain.CurrentHeader().NumberU64())
			return errors.WithMessagef(err,
				"[VerifyNewBlock] Cannot Verify New Block!!! block-hash %s txn-count %d",
				newBlock.Hash().Hex(),
				len(newBlock.Transactions()),
			)
		}
		return nil
	}
}
