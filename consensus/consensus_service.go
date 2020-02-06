package consensus

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/profiler"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/shard/committee"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

var (
	errValidNotInCommittee = errors.New("slot signer not this slot's subcommittee")
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

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(request *msg_pb.ConsensusRequest) {
	request.ViewId = consensus.viewID
	request.BlockNum = consensus.blockNum
	request.ShardId = consensus.ShardID

	// 32 byte block hash
	request.BlockHash = consensus.blockHash[:]

	// sender address
	request.SenderPubkey = consensus.PubKey.Serialize()
	consensus.getLogger().Debug().
		Str("senderKey", consensus.PubKey.SerializeToHexStr()).
		Msg("[populateMessageFields]")
}

// Signs the consensus message and returns the marshaled message.
func (consensus *Consensus) signAndMarshalConsensusMessage(message *msg_pb.Message) ([]byte, error) {
	err := consensus.signConsensusMessage(message)
	if err != nil {
		return []byte{}, err
	}

	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return []byte{}, err
	}
	return marshaledMessage, nil
}

// GetNodeIDs returns Node IDs of all nodes in the same shard
func (consensus *Consensus) GetNodeIDs() []libp2p_peer.ID {
	nodes := make([]libp2p_peer.ID, 0)
	nodes = append(nodes, consensus.host.GetID())
	consensus.validators.Range(func(k, v interface{}) bool {
		if peer, ok := v.(p2p.Peer); ok {
			nodes = append(nodes, peer.PeerID)
			return true
		}
		return false
	})
	return nodes
}

// GetViewID returns the consensus ID
func (consensus *Consensus) GetViewID() uint64 {
	return consensus.viewID
}

// DebugPrintPublicKeys print all the PublicKeys in string format in Consensus
func (consensus *Consensus) DebugPrintPublicKeys() {
	keys := consensus.Decider.DumpParticipants()
	utils.Logger().Debug().Strs("PublicKeys", keys).Int("count", len(keys)).Msgf("Debug Public Keys")
}

// UpdatePublicKeys updates the PublicKeys for quorum on current subcommittee, protected by a mutex
func (consensus *Consensus) UpdatePublicKeys(pubKeys []*bls.PublicKey) int64 {
	consensus.pubKeyLock.Lock()
	consensus.Decider.UpdateParticipants(pubKeys)
	utils.Logger().Info().Msg("My Committee updated")
	for i := range pubKeys {
		utils.Logger().Info().Int("index", i).Str("BLSPubKey", pubKeys[i].SerializeToHexStr()).Msg("Member")
	}
	consensus.LeaderPubKey = pubKeys[0]
	utils.Logger().Info().
		Str("info", consensus.LeaderPubKey.SerializeToHexStr()).Msg("My Leader")
	consensus.pubKeyLock.Unlock()
	// reset states after update public keys
	consensus.ResetState()
	consensus.ResetViewChangeState()
	return consensus.Decider.ParticipantsCount()
}

// NewFaker returns a faker consensus.
func NewFaker() *Consensus {
	return &Consensus{}
}

// Sign on the hash of the message
func (consensus *Consensus) signMessage(message []byte) []byte {
	hash := hash.Keccak256(message)
	signature := consensus.priKey.SignHash(hash[:])
	return signature.Serialize()
}

// Sign on the consensus message signature field.
func (consensus *Consensus) signConsensusMessage(message *msg_pb.Message) error {
	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	signature := consensus.signMessage(marshaledMessage)
	message.Signature = signature
	return nil
}

// GetValidatorPeers returns list of validator peers.
func (consensus *Consensus) GetValidatorPeers() []p2p.Peer {
	validatorPeers := make([]p2p.Peer, 0)

	consensus.validators.Range(func(k, v interface{}) bool {
		if peer, ok := v.(p2p.Peer); ok {
			validatorPeers = append(validatorPeers, peer)
			return true
		}
		return false
	})

	return validatorPeers
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
	consensus.getLogger().Debug().
		Str("Phase", consensus.phase.String()).
		Msg("[ResetState] Resetting consensus state")
	consensus.switchPhase(FBFTAnnounce, true)
	consensus.blockHash = [32]byte{}
	consensus.blockHeader = []byte{}
	consensus.block = []byte{}
	consensus.Decider.ResetPrepareAndCommitVotes()
	members := consensus.Decider.Participants()
	prepareBitmap, _ := bls_cosi.NewMask(members, nil)
	commitBitmap, _ := bls_cosi.NewMask(members, nil)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap
	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil
}

// Returns a string representation of this consensus
func (consensus *Consensus) String() string {
	var duty string
	if consensus.IsLeader() {
		duty = "LDR" // leader
	} else {
		duty = "VLD" // validator
	}
	return fmt.Sprintf("[duty:%s, PubKey:%s, ShardID:%v]",
		duty, consensus.PubKey.SerializeToHexStr(), consensus.ShardID)
}

// ToggleConsensusCheck flip the flag of whether ignore viewID check during consensus process
func (consensus *Consensus) ToggleConsensusCheck() {
	consensus.infoMutex.Lock()
	defer consensus.infoMutex.Unlock()
	consensus.ignoreViewIDCheck = !consensus.ignoreViewIDCheck
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
	senderKey, err := bls_cosi.BytesToBlsPublicKey(consensusMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, errValidNotInCommittee
	}
	return senderKey, nil
}

func (consensus *Consensus) verifyViewChangeSenderKey(msg *msg_pb.Message) (*bls.PublicKey, error) {
	vcMsg := msg.GetViewchange()
	senderKey, err := bls_cosi.BytesToBlsPublicKey(vcMsg.SenderPubkey)
	if err != nil {
		return nil, err
	}

	if !consensus.IsValidatorInCommittee(senderKey) {
		return nil, errValidNotInCommittee
	}
	return senderKey, nil
}

// SetViewID set the viewID to the height of the blockchain
func (consensus *Consensus) SetViewID(height uint64) {
	consensus.viewID = height
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
	if consensus.ignoreViewIDCheck {
		//in syncing mode, node accepts incoming messages without viewID/leaderKey checking
		//so only set mode to normal when new node enters consensus and need checking viewID
		consensus.current.SetMode(Normal)
		consensus.viewID = msg.ViewID
		consensus.current.SetViewID(msg.ViewID)
		consensus.LeaderPubKey = msg.SenderPubkey
		consensus.ignoreViewIDCheck = false
		consensus.consensusTimeout[timeoutConsensus].Start()
		utils.Logger().Debug().
			Uint64("viewID", consensus.viewID).
			Str("leaderKey", consensus.LeaderPubKey.SerializeToHexStr()[:20]).
			Msg("viewID and leaderKey override")
		utils.Logger().Debug().
			Uint64("viewID", consensus.viewID).
			Uint64("block", consensus.blockNum).
			Msg("Start consensus timer")
		return nil
	} else if msg.ViewID > consensus.viewID {
		return consensus_engine.ErrViewIDNotMatch
	} else if msg.ViewID < consensus.viewID {
		return errors.New("view ID belongs to the past")
	}
	return nil
}

// SetBlockNum sets the blockNum in consensus object, called at node bootstrap
func (consensus *Consensus) SetBlockNum(blockNum uint64) {
	consensus.infoMutex.Lock()
	defer consensus.infoMutex.Unlock()
	consensus.blockNum = blockNum
}

// SetEpochNum sets the epoch in consensus object
func (consensus *Consensus) SetEpochNum(epoch uint64) {
	consensus.infoMutex.Lock()
	defer consensus.infoMutex.Unlock()
	consensus.epoch = epoch
}

// ReadSignatureBitmapPayload read the payload for signature and bitmap; offset is the beginning position of reading
func (consensus *Consensus) ReadSignatureBitmapPayload(
	recvPayload []byte, offset int,
) (*bls.Sign, *bls_cosi.Mask, error) {
	if offset+96 > len(recvPayload) {
		return nil, nil, errors.New("payload not have enough length")
	}
	sigAndBitmapPayload := recvPayload[offset:]
	return chain.ReadSignatureBitmapByPublicKeys(
		sigAndBitmapPayload, consensus.Decider.Participants(),
	)
}

func (consensus *Consensus) reportMetrics(block types.Block) {
	endTime := time.Now()
	timeElapsed := endTime.Sub(startTime)
	numOfTxs := len(block.Transactions())
	tps := float64(numOfTxs) / timeElapsed.Seconds()
	consensus.getLogger().Info().
		Int("numOfTXs", numOfTxs).
		Time("startTime", startTime).
		Time("endTime", endTime).
		Dur("timeElapsed", endTime.Sub(startTime)).
		Float64("TPS", tps).
		Msg("TPS Report")

	// Post metrics
	profiler := profiler.GetProfiler()
	if profiler.MetricsReportURL == "" {
		return
	}

	txHashes := []string{}
	for i, end := 0, len(block.Transactions()); i < 3 && i < end; i++ {
		txHash := block.Transactions()[end-1-i].Hash()
		txHashes = append(txHashes, hex.EncodeToString(txHash[:]))
	}
	metrics := map[string]interface{}{
		"key":             hex.EncodeToString(consensus.PubKey.Serialize()),
		"tps":             tps,
		"txCount":         numOfTxs,
		"nodeCount":       consensus.Decider.ParticipantsCount() + 1,
		"latestBlockHash": hex.EncodeToString(consensus.blockHash[:]),
		"latestTxHashes":  txHashes,
		"blockLatency":    int(timeElapsed / time.Millisecond),
	}
	profiler.LogMetrics(metrics)
}

// getLogger returns logger for consensus contexts added
func (consensus *Consensus) getLogger() *zerolog.Logger {
	logger := utils.Logger().With().
		Uint64("myEpoch", consensus.epoch).
		Uint64("myBlock", consensus.blockNum).
		Uint64("myViewID", consensus.viewID).
		Interface("phase", consensus.phase).
		Str("mode", consensus.current.Mode().String()).
		Logger()
	return &logger
}

// retrieve corresponding blsPublicKey from Coinbase Address
func (consensus *Consensus) getLeaderPubKeyFromCoinbase(header *block.Header) (*bls.PublicKey, error) {
	shardState, err := consensus.ChainReader.ReadShardState(header.Epoch())
	if err != nil {
		return nil, ctxerror.New("cannot read shard state",
			"epoch", header.Epoch(),
			"coinbaseAddr", header.Coinbase(),
		).WithCause(err)
	}

	committee := shardState.FindCommitteeByID(header.ShardID())
	if committee == nil {
		return nil, ctxerror.New("cannot find shard in the shard state",
			"blockNum", header.Number(),
			"shardID", header.ShardID(),
			"coinbaseAddr", header.Coinbase(),
		)
	}
	committerKey := new(bls.PublicKey)
	isStaking := consensus.ChainReader.Config().IsStaking(header.Epoch())
	for _, member := range committee.Slots {
		if isStaking {
			// After staking the coinbase address will be the address of bls public key
			if utils.GetAddressFromBlsPubKeyBytes(member.BlsPublicKey[:]) == header.Coinbase() {
				err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
				if err != nil {
					return nil, ctxerror.New("cannot convert BLS public key",
						"blsPublicKey", member.BlsPublicKey,
						"coinbaseAddr", header.Coinbase()).WithCause(err)
				}
				return committerKey, nil
			}
		} else {
			if member.EcdsaAddress == header.Coinbase() {
				err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
				if err != nil {
					return nil, ctxerror.New("cannot convert BLS public key",
						"blsPublicKey", member.BlsPublicKey,
						"coinbaseAddr", header.Coinbase()).WithCause(err)
				}
				return committerKey, nil
			}
		}
	}
	return nil, ctxerror.New("cannot find corresponding BLS Public Key", "coinbaseAddr", header.Coinbase().Hex())
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
	prevSubCommitteeDump := consensus.Decider.JSON()

	isFirstTimeStaking := consensus.ChainReader.Config().IsStaking(nextEpoch) &&
		len(curHeader.ShardState()) > 0 &&
		!consensus.ChainReader.Config().IsStaking(curEpoch)

	haventUpdatedDecider := consensus.ChainReader.Config().IsStaking(curEpoch) &&
		consensus.Decider.Policy() != quorum.SuperMajorityStake

	// Only happens once, the flip-over to a new Decider policy
	if isFirstTimeStaking || haventUpdatedDecider {
		decider := quorum.NewDecider(quorum.SuperMajorityStake)
		decider.SetShardIDProvider(func() (uint32, error) {
			return consensus.ShardID, nil
		})
		decider.SetMyPublicKeyProvider(func() (*bls.PublicKey, error) {
			return consensus.PubKey, nil
		})
		consensus.Decider = decider
	}

	committeeToSet := &shard.Committee{}
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
	if len(curHeader.ShardState()) > 0 {
		// increase curEpoch by one if it's the last block
		consensus.SetEpochNum(curEpoch.Uint64() + 1)
		consensus.getLogger().Info().Uint64("headerNum", curHeader.Number().Uint64()).
			Msg("[UpdateConsensusInformation] Epoch updated for nextEpoch curEpoch")

		nextShardState, err := committee.WithStakingEnabled.ReadFromDB(
			nextEpoch, consensus.ChainReader,
		)
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Uint32("shard", consensus.ShardID).
				Msg("[UpdateConsensusInformation] Error retrieving nextEpoch shard state")
			return Syncing
		}

		committeeToSet = nextShardState.FindCommitteeByID(curHeader.ShardID())

	} else {
		consensus.SetEpochNum(curEpoch.Uint64())
		committeeToSet = curShardState.FindCommitteeByID(curHeader.ShardID())
	}

	if len(committeeToSet.Slots) == 0 {
		consensus.getLogger().Warn().
			Msg("[UpdateConsensusInformation] No members in the committee to update")
		hasError = true
	}

	// update public keys in the committee
	oldLeader := consensus.LeaderPubKey
	pubKeys := committee.WithStakingEnabled.GetCommitteePublicKeys(
		committeeToSet,
	)
	consensus.getLogger().Info().
		Int("numPubKeys", len(pubKeys)).
		Msg("[UpdateConsensusInformation] Successfully updated public keys")
	consensus.UpdatePublicKeys(pubKeys)

	// Update voters in the committee
	if _, err := consensus.Decider.SetVoters(
		committeeToSet.Slots, true,
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
		RawJSON("prev-subcommittee", []byte(prevSubCommitteeDump)).
		RawJSON("current-subcommittee", []byte(consensus.Decider.JSON())).
		Msg("[UpdateConsensusInformation] changing committee")

	// take care of possible leader change during the curEpoch
	if !shard.Schedule.IsLastBlock(curHeader.Number().Uint64()) &&
		curHeader.Number().Uint64() != 0 {
		leaderPubKey, err := consensus.getLeaderPubKeyFromCoinbase(curHeader)
		if err != nil || leaderPubKey == nil {
			consensus.getLogger().Debug().Err(err).
				Msg("[UpdateConsensusInformation] Unable to get leaderPubKey from coinbase")
			consensus.ignoreViewIDCheck = true
			hasError = true
		} else {
			consensus.getLogger().Debug().
				Str("leaderPubKey", leaderPubKey.SerializeToHexStr()).
				Msg("[UpdateConsensusInformation] Most Recent LeaderPubKey Updated Based on BlockChain")
			consensus.LeaderPubKey = leaderPubKey
		}
	}

	for i := range pubKeys {
		// in committee
		if pubKeys[i].IsEqual(consensus.PubKey) {
			if hasError {
				return Syncing
			}

			// If the leader changed and I myself become the leader
			if !consensus.LeaderPubKey.IsEqual(oldLeader) && consensus.LeaderPubKey.IsEqual(consensus.PubKey) {
				go func() {
					utils.Logger().Debug().
						Str("myKey", consensus.PubKey.SerializeToHexStr()).
						Uint64("viewID", consensus.viewID).
						Uint64("block", consensus.blockNum).
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
	if consensus.PubKey != nil && consensus.LeaderPubKey != nil {
		return consensus.PubKey.IsEqual(consensus.LeaderPubKey)
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
