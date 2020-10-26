package consensus

import (
	"math/big"
	"sync/atomic"
	"time"

	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/signature"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
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
func (consensus *Consensus) UpdatePublicKeys(pubKeys []bls_cosi.PublicKeyWrapper) int64 {
	// TODO: use mutex for updating public keys pointer. No need to lock on all these logic.
	consensus.pubKeyLock.Lock()
	consensus.Decider.UpdateParticipants(pubKeys)
	utils.Logger().Info().Msg("My Committee updated")
	for i := range pubKeys {
		utils.Logger().Info().
			Int("index", i).
			Str("BLSPubKey", pubKeys[i].Bytes.Hex()).
			Msg("Member")
	}

	allKeys := consensus.Decider.Participants()
	if len(allKeys) != 0 {
		consensus.LeaderPubKey = &allKeys[0]
		utils.Logger().Info().
			Str("info", consensus.LeaderPubKey.Bytes.Hex()).Msg("My Leader")
	} else {
		utils.Logger().Error().
			Msg("[UpdatePublicKeys] Participants is empty")
	}
	consensus.pubKeyLock.Unlock()
	// reset states after update public keys
	// TODO: incorporate bitmaps in the decider, so their state can't be inconsistent.
	consensus.UpdateBitmaps()
	consensus.ResetState()

	consensus.ResetViewChangeState()
	return consensus.Decider.ParticipantsCount()
}

// NewFaker returns a faker consensus.
func NewFaker() *Consensus {
	return &Consensus{}
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
func (consensus *Consensus) UpdateBitmaps() {
	consensus.getLogger().Debug().
		Str("MessageType", consensus.phase.String()).
		Msg("[UpdateBitmaps] Updating consensus bitmaps")
	members := consensus.Decider.Participants()
	prepareBitmap, _ := bls_cosi.NewMask(members, nil)
	commitBitmap, _ := bls_cosi.NewMask(members, nil)
	multiSigBitmap, _ := bls_cosi.NewMask(members, nil)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap
	consensus.multiSigMutex.Lock()
	consensus.multiSigBitmap = multiSigBitmap
	consensus.multiSigMutex.Unlock()
}

// ResetState resets the state of the consensus
func (consensus *Consensus) ResetState() {
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

// ToggleConsensusCheck flip the flag of whether ignore viewID check during consensus process
func (consensus *Consensus) ToggleConsensusCheck() {
	consensus.IgnoreViewIDCheck.Toggle()
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (consensus *Consensus) IsValidatorInCommittee(pubKey bls.SerializedPublicKey) bool {
	return consensus.Decider.IndexOf(pubKey) != -1
}

// SetMode sets the mode of consensus
func (consensus *Consensus) SetMode(m Mode) {
	consensus.current.SetMode(m)
}

// Mode returns the mode of consensus
func (consensus *Consensus) Mode() Mode {
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
		consensus.current.SetMode(Normal)
		consensus.SetViewIDs(msg.ViewID)
		if !msg.HasSingleSender() {
			return errors.New("Leader message can not have multiple sender keys")
		}
		consensus.LeaderPubKey = msg.SenderPubkeys[0]
		consensus.IgnoreViewIDCheck.UnSet()
		consensus.consensusTimeout[timeoutConsensus].Start()
		consensus.getLogger().Debug().
			Str("leaderKey", consensus.LeaderPubKey.Bytes.Hex()).
			Msg("Start consensus timer")
		return nil
	} else if msg.ViewID > consensus.GetCurBlockViewID() {
		return consensus_engine.ErrViewIDNotMatch
	} else if msg.ViewID < consensus.GetCurBlockViewID() {
		return errors.New("view ID belongs to the past")
	}
	return nil
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (consensus *Consensus) SetBlockNum(blockNum uint64) {
	atomic.StoreUint64(&consensus.blockNum, blockNum)
}

// ReadSignatureBitmapPayload read the payload for signature and bitmap; offset is the beginning position of reading
func (consensus *Consensus) ReadSignatureBitmapPayload(
	recvPayload []byte, offset int,
) (*bls_core.Sign, *bls_cosi.Mask, error) {
	if offset+bls.BLSSignatureSizeInBytes > len(recvPayload) {
		return nil, nil, errors.New("payload not have enough length")
	}
	sigAndBitmapPayload := recvPayload[offset:]

	// TODO(audit): keep a Mask in the Decider so it won't be reconstructed on the fly.
	members := consensus.Decider.Participants()
	return chain.ReadSignatureBitmapByPublicKeys(
		sigAndBitmapPayload, members,
	)
}

// retrieve corresponding blsPublicKey from Coinbase Address
func (consensus *Consensus) getLeaderPubKeyFromCoinbase(
	header *block.Header,
) (*bls.PublicKeyWrapper, error) {
	shardState, err := consensus.ChainReader.ReadShardState(header.Epoch())
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read shard state %v %s",
			header.Epoch(),
			header.Coinbase().Hash().Hex(),
		)
	}

	committee, err := shardState.FindCommitteeByID(header.ShardID())
	if err != nil {
		return nil, err
	}

	committerKey := new(bls_core.PublicKey)
	isStaking := consensus.ChainReader.Config().IsStaking(header.Epoch())
	for _, member := range committee.Slots {
		if isStaking {
			// After staking the coinbase address will be the address of bls public key
			if utils.GetAddressFromBLSPubKeyBytes(member.BLSPublicKey[:]) == header.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		} else {
			if member.EcdsaAddress == header.Coinbase() {
				if committerKey, err = bls.BytesToBLSPublicKey(member.BLSPublicKey[:]); err != nil {
					return nil, err
				}
				return &bls.PublicKeyWrapper{Object: committerKey, Bytes: member.BLSPublicKey}, nil
			}
		}
	}
	return nil, errors.Errorf(
		"cannot find corresponding BLS Public Key coinbase %s",
		header.Coinbase().Hex(),
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
	curHeader := consensus.ChainReader.CurrentHeader()
	curEpoch := curHeader.Epoch()
	nextEpoch := new(big.Int).Add(curHeader.Epoch(), common.Big1)

	// Overwrite nextEpoch if the shard state has a epoch number
	if curHeader.IsLastBlockInEpoch() {
		nextShardState, err := curHeader.GetShardState()
		if err != nil {
			return Syncing
		}
		if nextShardState.Epoch != nil {
			nextEpoch = nextShardState.Epoch
		}
	}

	consensus.BlockPeriod = 5 * time.Second

	// Enable aggregate sig at epoch 1000 for mainnet, at epoch 53000 for testnet, and always for other nets.
	if (consensus.ChainReader.Config().ChainID == params.MainnetChainID && curEpoch.Cmp(big.NewInt(1000)) > 0) ||
		(consensus.ChainReader.Config().ChainID == params.TestnetChainID && curEpoch.Cmp(big.NewInt(54500)) > 0) ||
		(consensus.ChainReader.Config().ChainID != params.MainnetChainID && consensus.ChainReader.Config().ChainID != params.TestChainID) {
		consensus.AggregateSig = true
	}

	isFirstTimeStaking := consensus.ChainReader.Config().IsStaking(nextEpoch) &&
		curHeader.IsLastBlockInEpoch() && !consensus.ChainReader.Config().IsStaking(curEpoch)
	haventUpdatedDecider := consensus.ChainReader.Config().IsStaking(curEpoch) &&
		consensus.Decider.Policy() != quorum.SuperMajorityStake

	// Only happens once, the flip-over to a new Decider policy
	if isFirstTimeStaking || haventUpdatedDecider {
		decider := quorum.NewDecider(quorum.SuperMajorityStake, consensus.ShardID)
		decider.SetMyPublicKeyProvider(func() (multibls.PublicKeys, error) {
			return consensus.GetPublicKeys(), nil
		})
		consensus.Decider = decider
	}

	var committeeToSet *shard.Committee
	epochToSet := curEpoch
	hasError := false
	curShardState, err := committee.WithStakingEnabled.ReadFromDB(
		curEpoch, consensus.ChainReader,
	)
	if err != nil {
		utils.Logger().Error().
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
			nextEpoch, consensus.ChainReader,
		)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("Error retrieving nextEpoch shard state")
			return Syncing
		}

		subComm, err := nextShardState.FindCommitteeByID(curHeader.ShardID())
		if err != nil {
			utils.Logger().Error().
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
			utils.Logger().Error().
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
	consensus.UpdatePublicKeys(pubKeys)

	// Update voters in the committee
	if _, err := consensus.Decider.SetVoters(
		committeeToSet, epochToSet,
	); err != nil {
		utils.Logger().Error().
			Err(err).
			Uint32("shard", consensus.ShardID).
			Msg("Error when updating voters")
		return Syncing
	}

	utils.Logger().Info().
		Uint64("block-number", curHeader.Number().Uint64()).
		Uint64("curEpoch", curHeader.Epoch().Uint64()).
		Uint32("shard-id", consensus.ShardID).
		Msg("[UpdateConsensusInformation] changing committee")

	// take care of possible leader change during the epoch
	if !curHeader.IsLastBlockInEpoch() && curHeader.Number().Uint64() != 0 {
		leaderPubKey, err := consensus.getLeaderPubKeyFromCoinbase(curHeader)
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
		myPubKeys := consensus.GetPublicKeys()
		if myPubKeys.Contains(key.Object) {
			if hasError {
				return Syncing
			}

			// If the leader changed and I myself become the leader
			if (oldLeader != nil && consensus.LeaderPubKey != nil &&
				!consensus.LeaderPubKey.Object.IsEqual(oldLeader.Object)) && consensus.IsLeader() {
				go func() {
					consensus.getLogger().Info().
						Str("myKey", myPubKeys.SerializeToHexStr()).
						Msg("[UpdateConsensusInformation] I am the New Leader")
					consensus.ReadySignal <- struct{}{}
				}()
			}
			return Normal
		}
	}
	// not in committee
	return Listening
}

// IsLeader check if the node is a leader or not by comparing the public key of
// the node with the leader public key
func (consensus *Consensus) IsLeader() bool {
	for _, key := range consensus.priKey {
		if key.Pub.Object.IsEqual(consensus.LeaderPubKey.Object) {
			return true
		}
	}
	return false
}

// NeedsRandomNumberGeneration returns true if the current epoch needs random number generation
func (consensus *Consensus) NeedsRandomNumberGeneration(epoch *big.Int) bool {
	if consensus.ShardID == 0 && epoch.Uint64() >= shard.Schedule.RandomnessStartingEpoch() {
		return true
	}

	return false
}

// SetViewIDs set both current view ID and view changing ID to the height
// of the blockchain. It is used during client startup to recover the state
func (consensus *Consensus) SetViewIDs(height uint64) {
	consensus.SetCurBlockViewID(height)
	consensus.SetViewChangingID(height)
}

// SetCurBlockViewID set the current view ID
func (consensus *Consensus) SetCurBlockViewID(viewID uint64) {
	consensus.current.SetCurBlockViewID(viewID)
}

// SetViewChangingID set the current view change ID
func (consensus *Consensus) SetViewChangingID(viewID uint64) {
	consensus.current.SetViewChangingID(viewID)
}

// StartFinalityCount set the finality counter to current time
func (consensus *Consensus) StartFinalityCount() {
	consensus.finalityCounter = time.Now().UnixNano()
}

// FinishFinalityCount calculate the current finality
func (consensus *Consensus) FinishFinalityCount() {
	d := time.Now().UnixNano()
	consensus.finality = (d - consensus.finalityCounter) / 1000000
}

// GetFinality returns the finality time in milliseconds of previous consensus
func (consensus *Consensus) GetFinality() int64 {
	return consensus.finality
}

// switchPhase will switch FBFTPhase to nextPhase if the desirePhase equals the nextPhase
func (consensus *Consensus) switchPhase(subject string, desired FBFTPhase) {
	consensus.getLogger().Info().
		Str("from:", consensus.phase.String()).
		Str("to:", desired.String()).
		Str("switchPhase:", subject)

	consensus.phase = desired
	return
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
	block := consensus.FBFTLog.GetBlockByHash(blockHash)
	if block == nil {
		return errGetPreparedBlock
	}

	aggSig, mask, err := consensus.ReadSignatureBitmapPayload(payload, 32)
	if err != nil {
		return errReadBitmapPayload
	}

	// update consensus structure when succeeded
	// protect consensus data update logic
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()

	// Have to keep the block hash so the leader can finish the commit phase of prepared block
	consensus.ResetState()

	copy(consensus.blockHash[:], blockHash[:])
	consensus.switchPhase("selfCommit", FBFTCommit)
	consensus.aggregatedPrepareSig = aggSig
	consensus.prepareBitmap = mask
	commitPayload := signature.ConstructCommitPayload(consensus.ChainReader,
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

		if _, err := consensus.Decider.SubmitVote(
			quorum.Commit,
			[]bls.SerializedPublicKey{key.Pub.Bytes},
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

// getLogger returns logger for consensus contexts added
func (consensus *Consensus) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Uint64("myBlock", consensus.blockNum).
		Uint64("myViewID", consensus.GetCurBlockViewID()).
		Str("phase", consensus.phase.String()).
		Str("mode", consensus.current.Mode().String()).
		Logger()
	return &logger
}
