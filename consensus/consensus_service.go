package consensus

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	"github.com/pkg/errors"
)

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

// Signs the consensus message and returns the marshaled message.
func (consensus *Consensus) signAndMarshalConsensusMessage(
	message *msg_pb.Message, priKey *bls.SecretKey,
) ([]byte, error) {
	if err := consensus.signConsensusMessage(message, priKey); err != nil {
		return nil, err
	}
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return nil, err
	}
	return marshaledMessage, nil
}

// UpdatePublicKeys updates the PublicKeys for
// quorum on current subcommittee, protected by a mutex
func (consensus *Consensus) UpdatePublicKeys(pubKeys []*bls.PublicKey) int64 {
	consensus.Locks.PubKey.Lock()
	consensus.Decider.UpdateParticipants(pubKeys)
	utils.Logger().Info().Msg("My Committee updated")
	consensus.SetLeaderPubKey(pubKeys[0])
	consensus.Locks.PubKey.Unlock()

	// reset states after update public keys
	consensus.ResetState()
	consensus.ResetViewChangeState()
	return consensus.Decider.ParticipantsCount()
}

// Sign on the hash of the message
func (consensus *Consensus) signMessage(message []byte, priKey *bls.SecretKey) []byte {
	hash := hash.Keccak256(message)
	signature := priKey.SignHash(hash[:])
	return signature.Serialize()
}

// Sign on the consensus message signature field.
func (consensus *Consensus) signConsensusMessage(message *msg_pb.Message,
	priKey *bls.SecretKey) error {
	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage, priKey)
	message.Signature = signature
	return nil
}

// GetViewIDSigsArray returns the signatures for viewID in viewchange
func (consensus *Consensus) GetViewIDSigsArray(viewID uint64) []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.viewIDSigs[viewID] {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetNilSigsArray returns the signatures for nil prepared message in viewchange
func (consensus *Consensus) GetNilSigsArray(viewID uint64) []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.nilSigs[viewID] {
		sigs = append(sigs, sig)
	}
	return sigs
}

// ResetState resets the state of the consensus
func (consensus *Consensus) ResetState() {
	utils.Logger().Debug().Msg("[ResetState] Resetting consensus state")
	consensus.switchPhase(FBFTAnnounce)
	consensus.SetBlockHash(common.Hash{})
	consensus.SetBlockHeader([]byte{})
	consensus.SetBlock([]byte{})
	consensus.Decider.ResetPrepareAndCommitVotes()
	members := consensus.Decider.Participants()
	prepareBitmap, _ := bls_cosi.NewMask(members, nil)
	commitBitmap, _ := bls_cosi.NewMask(members, nil)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap
	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (consensus *Consensus) IsValidatorInCommittee(pubKey *bls.PublicKey) bool {
	return consensus.Decider.IndexOf(pubKey) != -1
}

// Verify the signature of the message are valid from the signer's public key.
func verifyMessageSig(signerPubKey *bls.PublicKey, message *msg_pb.Message) error {
	signature := message.Signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}

	msgSig := bls.Sign{}
	err = msgSig.Deserialize(signature)
	if err != nil {
		return err
	}
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(signerPubKey, msgHash[:]) {
		return errors.New("failed to verify the signature")
	}
	message.Signature = signature
	return nil
}

// verifySenderKey verifys the message senderKey is properly signed and senderAddr is valid
func (consensus *Consensus) verifySenderKey(msg *msg_pb.Message) (*bls.PublicKey, error) {
	consensusMsg := msg.GetConsensus()
	senderKey, err := bls_cosi.BytesToBLSPublicKey(
		consensusMsg.GetSenderPubkey(),
	)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, shard.ErrValidNotInCommittee
	}
	return senderKey, nil
}

func (consensus *Consensus) verifyViewChangeSenderKey(msg *msg_pb.Message) (*bls.PublicKey, error) {
	vcMsg := msg.GetViewchange()
	senderKey, err := bls_cosi.BytesToBLSPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, shard.ErrValidNotInCommittee
	}
	return senderKey, nil
}

// SetViewID set the viewID to the height of the blockchain
func (consensus *Consensus) SetViewID(height uint64) {
	consensus.viewID.Store(height)
	consensus.Current.SetViewID(height)
}

// SetMode sets the mode of consensus
func (consensus *Consensus) SetMode(m Mode) {
	consensus.Current.SetMode(m)
}

// Mode returns the mode of consensus
func (consensus *Consensus) Mode() Mode {
	return consensus.Current.Mode()
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
	if consensus.ignoreViewIDCheck {
		//in syncing mode, node accepts incoming messages without viewID/leaderKey checking
		//so only set mode to normal when new node enters consensus and need checking viewID
		consensus.Current.SetMode(Normal)
		consensus.SetViewID(msg.ViewID)
		consensus.Current.SetViewID(msg.ViewID)
		consensus.SetLeaderPubKey(msg.SenderPubkey)
		consensus.ignoreViewIDCheck = false
		consensus.ResetConsensusTimeout <- struct{}{}
		return nil
	} else if msg.ViewID > consensus.ViewID() {
		return consensus_engine.ErrViewIDNotMatch
	} else if msg.ViewID < consensus.ViewID() {
		return errors.New("view ID belongs to the past")
	}
	return nil
}

// ReadSignatureBitmapPayload read the payload
// for signature and bitmap; offset is the beginning position of reading
func (consensus *Consensus) ReadSignatureBitmapPayload(
	recvPayload []byte, offset int,
) (*bls.Sign, *bls_cosi.Mask, error) {
	if offset+shard.BLSSignatureSizeInBytes > len(recvPayload) {
		return nil, nil, errors.New("payload not have enough length")
	}
	sigAndBitmapPayload := recvPayload[offset:]
	return chain.ReadSignatureBitmapByPublicKeys(
		sigAndBitmapPayload, consensus.Decider.Participants(),
	)
}

// retrieve corresponding blsPublicKey from Coinbase Address
func (consensus *Consensus) getLeaderPubKeyFromCoinbase(
	header *block.Header,
) (*bls.PublicKey, error) {
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

	committerKey := new(bls.PublicKey)
	isStaking := consensus.ChainReader.Config().IsStaking(header.Epoch())
	for _, member := range committee.Slots {
		if isStaking {
			// After staking the coinbase address will be the address of bls public key
			if utils.GetAddressFromBLSPubKeyBytes(member.BLSPublicKey[:]) == header.Coinbase() {
				if err := member.BLSPublicKey.ToLibBLSPublicKey(committerKey); err != nil {
					return nil, err
				}
				return committerKey, nil
			}
		} else {
			if member.EcdsaAddress == header.Coinbase() {
				if err := member.BLSPublicKey.ToLibBLSPublicKey(committerKey); err != nil {
					return nil, err
				}
				return committerKey, nil
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
	isFirstTimeStaking := consensus.ChainReader.Config().IsStaking(nextEpoch) &&
		len(curHeader.ShardState()) > 0 &&
		!consensus.ChainReader.Config().IsStaking(curEpoch)
	haventUpdatedDecider := consensus.ChainReader.Config().IsStaking(curEpoch) &&
		consensus.Decider.Policy() != quorum.SuperMajorityStake

	// Only happens once, the flip-over to a new Decider policy
	if isFirstTimeStaking || haventUpdatedDecider {
		decider := quorum.NewDecider(quorum.SuperMajorityStake, consensus.ShardID)
		decider.SetMyPublicKeyProvider(func() (*multibls.PublicKey, error) {
			return consensus.PubKey, nil
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

	utils.Logger().Info().Msg("[UpdateConsensusInformation] Updating.....")
	// genesis block is a special case that will have shard state and needs to skip processing
	isNotGenesisBlock := curHeader.Number().Cmp(big.NewInt(0)) > 0
	if len(curHeader.ShardState()) > 0 && isNotGenesisBlock {
		// increase curEpoch by one if it's the last block
		consensus.SetEpoch(curEpoch.Uint64() + 1)
		utils.Logger().Info().
			Uint64("headerNum", curHeader.Number().Uint64()).
			Msg("Epoch updated for nextEpoch curEpoch")

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
		consensus.SetEpoch(curEpoch.Uint64())
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
		utils.Logger().Warn().
			Msg("[UpdateConsensusInformation] No members in the committee to update")
		hasError = true
	}

	// update public keys in the committee
	oldLeader := &bls.PublicKey{}
	if err := oldLeader.DeserializeHexStr(
		consensus.LeaderPubKey().SerializeToHexStr(),
	); err != nil {
		return Syncing
	}

	pubKeys, _ := committeeToSet.BLSPublicKeys()

	utils.Logger().Info().
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

	// take care of possible leader change during the curEpoch
	if !shard.Schedule.IsLastBlock(curHeader.Number().Uint64()) &&
		curHeader.Number().Uint64() != 0 {
		leaderPubKey, err := consensus.getLeaderPubKeyFromCoinbase(curHeader)
		if err != nil || leaderPubKey == nil {
			utils.Logger().Debug().Err(err).
				Msg("[UpdateConsensusInformation] Unable to get leaderPubKey from coinbase")
			consensus.ignoreViewIDCheck = true
			hasError = true
		} else {
			utils.Logger().Debug().
				Msg("[UpdateConsensusInformation] Most Recent LeaderPubKey Updated Based on BlockChain")
			consensus.SetLeaderPubKey(leaderPubKey)
		}
	}

	for _, key := range pubKeys {
		// in committee
		if consensus.PubKey.Contains(key) {
			if hasError {
				return Syncing
			}

			// If the leader changed and I myself become the leader
			if !consensus.LeaderPubKey().IsEqual(oldLeader) && consensus.IsLeader() {
				go func() {
					utils.Logger().Debug().
						Uint64("viewID", consensus.ViewID()).
						Uint64("block", consensus.BlockNum()).
						Msg("[UpdateConsensusInformation] I am the New Leader")
					consensus.ProposalNewBlock <- struct{}{}
				}()
			}
			return Normal
		}
	}

	//
	// not in committee
	return Listening
}

// IsLeader check if the node is a leader or not by comparing the public key of
// the node with the leader public key
func (consensus *Consensus) IsLeader() bool {
	consensus.Locks.PubKey.Lock()
	defer consensus.Locks.PubKey.Unlock()

	for i := range consensus.PubKey.PublicKey {
		if consensus.priKey.PrivateKey[i].GetPublicKey().IsEqual(
			consensus.LeaderPubKey(),
		) {
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

func (consensus *Consensus) addViewIDKeyIfNotExist(viewID uint64) {
	members := consensus.Decider.Participants()
	if _, ok := consensus.bhpSigs[viewID]; !ok {
		consensus.bhpSigs[viewID] = map[string]*bls.Sign{}
	}
	if _, ok := consensus.nilSigs[viewID]; !ok {
		consensus.nilSigs[viewID] = map[string]*bls.Sign{}
	}
	if _, ok := consensus.viewIDSigs[viewID]; !ok {
		consensus.viewIDSigs[viewID] = map[string]*bls.Sign{}
	}
	if _, ok := consensus.bhpBitmap[viewID]; !ok {
		bhpBitmap, _ := bls_cosi.NewMask(members, nil)
		consensus.bhpBitmap[viewID] = bhpBitmap
	}
	if _, ok := consensus.nilBitmap[viewID]; !ok {
		nilBitmap, _ := bls_cosi.NewMask(members, nil)
		consensus.nilBitmap[viewID] = nilBitmap
	}
	if _, ok := consensus.viewIDBitmap[viewID]; !ok {
		viewIDBitmap, _ := bls_cosi.NewMask(members, nil)
		consensus.viewIDBitmap[viewID] = viewIDBitmap
	}
}
