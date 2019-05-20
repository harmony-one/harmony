package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	proto_discovery "github.com/harmony-one/harmony/api/proto/discovery"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	"golang.org/x/crypto/sha3"
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
func (consensus *Consensus) GetNextRnd() ([32]byte, [32]byte, error) {
	if len(consensus.pendingRnds) == 0 {
		return [32]byte{}, [32]byte{}, errors.New("No available randomness")
	}
	vdfOutput := consensus.pendingRnds[0]
	rnd := [32]byte{}
	blockHash := [32]byte{}
	copy(rnd[:], vdfOutput[:32])
	copy(blockHash[:], vdfOutput[32:])
	return rnd, blockHash, nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (consensus *Consensus) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	})
	hasher.Sum(hash[:0])
	return hash
}

// Seal is to seal final block.
func (consensus *Consensus) Seal(chain consensus_engine.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	// TODO: implement final block sealing
	return nil
}

// Prepare is to prepare ...
// TODO(RJ): fix it.
func (consensus *Consensus) Prepare(chain consensus_engine.ChainReader, header *types.Header) error {
	// TODO: implement prepare method
	return nil
}

// GetSelfAddress returns the address in hex
func (consensus *Consensus) GetSelfAddress() common.Address {
	return consensus.SelfAddress
}

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(request *msg_pb.ConsensusRequest) {
	request.ConsensusId = consensus.consensusID
	request.SeqNum = consensus.seqNum

	// 32 byte block hash
	request.BlockHash = consensus.blockHash[:]

	// sender address
	request.SenderPubkey = consensus.PubKey.Serialize()

	utils.GetLogInstance().Debug("[populateMessageFields]", "myConsensusID", consensus.consensusID, "SenderAddress", consensus.SelfAddress, "seqNum", consensus.seqNum)
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

// SetLeaderPubKey deserialize the public key of consensus leader
func (consensus *Consensus) SetLeaderPubKey(k []byte) error {
	consensus.leader.ConsensusPubKey = &bls.PublicKey{}
	return consensus.leader.ConsensusPubKey.Deserialize(k)
}

// GetLeaderPubKey returns the public key of consensus leader
func (consensus *Consensus) GetLeaderPubKey() *bls.PublicKey {
	return consensus.leader.ConsensusPubKey
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

// GetConsensusID returns the consensus ID
func (consensus *Consensus) GetConsensusID() uint32 {
	return consensus.consensusID
}

// DebugPrintPublicKeys print all the PublicKeys in string format in Consensus
func (consensus *Consensus) DebugPrintPublicKeys() {
	for _, k := range consensus.PublicKeys {
		str := fmt.Sprintf("%s", hex.EncodeToString(k.Serialize()))
		utils.GetLogInstance().Debug("pk:", "string", str)
	}

	utils.GetLogInstance().Debug("PublicKeys:", "#", len(consensus.PublicKeys))
}

// DebugPrintValidators print all validator ip/port/key in string format in Consensus
func (consensus *Consensus) DebugPrintValidators() {
	count := 0
	consensus.validators.Range(func(k, v interface{}) bool {
		if p, ok := v.(p2p.Peer); ok {
			str2 := fmt.Sprintf("%s", p.ConsensusPubKey.Serialize())
			utils.GetLogInstance().Debug("validator:", "IP", p.IP, "Port", p.Port, "address", utils.GetBlsAddress(p.ConsensusPubKey), "Key", str2)
			count++
			return true
		}
		return false
	})
	utils.GetLogInstance().Debug("Validators", "#", count)
}

// UpdatePublicKeys updates the PublicKeys variable, protected by a mutex
func (consensus *Consensus) UpdatePublicKeys(pubKeys []*bls.PublicKey) int {
	consensus.pubKeyLock.Lock()
	consensus.PublicKeys = append(pubKeys[:0:0], pubKeys...)
	consensus.CommitteeAddresses = map[common.Address]bool{}
	for _, pubKey := range consensus.PublicKeys {
		consensus.CommitteeAddresses[utils.GetBlsAddress(pubKey)] = true
	}
	// TODO: use pubkey to identify leader rather than p2p.Peer.
	consensus.leader = p2p.Peer{ConsensusPubKey: pubKeys[0]}
	consensus.LeaderPubKey = pubKeys[0]
	prepareBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	if err == nil {
		consensus.prepareBitmap = prepareBitmap
	}

	commitBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	if err == nil {
		consensus.commitBitmap = commitBitmap
	}

	utils.GetLogInstance().Info("My Leader", "info", hex.EncodeToString(consensus.leader.ConsensusPubKey.Serialize()))
	utils.GetLogInstance().Info("My Committee", "info", consensus.PublicKeys)
	consensus.pubKeyLock.Unlock()

	return len(consensus.PublicKeys)
}

// NewFaker returns a faker consensus.
func NewFaker() *Consensus {
	return &Consensus{}
}

// VerifyHeader checks whether a header conforms to the consensus rules of the bft engine.
func (consensus *Consensus) VerifyHeader(chain consensus_engine.ChainReader, header *types.Header, seal bool) error {
	parentHeader := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parentHeader == nil {
		return consensus_engine.ErrUnknownAncestor
	}
	if seal {
		if err := consensus.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (consensus *Consensus) VerifyHeaders(chain consensus_engine.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))
	for i := 0; i < len(headers); i++ {
		results <- nil
	}
	return abort, results
}

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (consensus *Consensus) VerifySeal(chain consensus_engine.ChainReader, header *types.Header) error {
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (consensus *Consensus) Finalize(chain consensus_engine.ChainReader, header *types.Header, state *state.DB, txs []*types.Transaction, receipts []*types.Receipt) (*types.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	// Header seems complete, assemble into a block and return
	consensus.accumulateRewards(chain.Config(), state, header)
	header.Root = state.IntermediateRoot(false)
	return types.NewBlock(header, txs, receipts), nil
}

// Author returns the author of the block header.
func (consensus *Consensus) Author(header *types.Header) (common.Address, error) {
	// TODO: implement this
	return common.Address{}, nil
}

// Sign on the hash of the message
func (consensus *Consensus) signMessage(message []byte) []byte {
	hash := sha256.Sum256(message)
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

// GetPrepareSigsArray returns the signatures for prepare as a array
func (consensus *Consensus) GetPrepareSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.prepareSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetCommitSigsArray returns the signatures for commit as a array
func (consensus *Consensus) GetCommitSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range consensus.commitSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// ResetState resets the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.state = Finished
	consensus.phase = Announce
	consensus.prepareSigs = map[common.Address]*bls.Sign{}
	consensus.commitSigs = map[common.Address]*bls.Sign{}

	prepareBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	commitBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap

	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil
}

// Returns a string representation of this consensus
func (consensus *Consensus) String() string {
	var duty string
	if consensus.IsLeader {
		duty = "LDR" // leader
	} else {
		duty = "VLD" // validator
	}
	return fmt.Sprintf("[duty:%s, PubKey:%s, ShardID:%v, Address:%v, state:%s]",
		duty, hex.EncodeToString(consensus.PubKey.Serialize()), consensus.ShardID, consensus.SelfAddress, consensus.state)
}

// AddPeers adds new peers into the validator map of the consensus
// and add the public keys
func (consensus *Consensus) AddPeers(peers []*p2p.Peer) int {
	count := 0

	for _, peer := range peers {
		_, ok := consensus.validators.LoadOrStore(utils.GetBlsAddress(peer.ConsensusPubKey).Hex(), *peer)
		if !ok {
			consensus.pubKeyLock.Lock()
			if _, ok := consensus.CommitteeAddresses[peer.ConsensusPubKey.GetAddress()]; !ok {
				consensus.PublicKeys = append(consensus.PublicKeys, peer.ConsensusPubKey)
				consensus.CommitteeAddresses[peer.ConsensusPubKey.GetAddress()] = true
			}
			consensus.pubKeyLock.Unlock()
		}
		count++
	}
	return count
}

// RemovePeers will remove the peer from the validator list and PublicKeys
// It will be called when leader/node lost connection to peers
func (consensus *Consensus) RemovePeers(peers []p2p.Peer) int {
	// early return as most of the cases no peers to remove
	if len(peers) == 0 {
		return 0
	}

	count := 0
	count2 := 0
	newList := append(consensus.PublicKeys[:0:0], consensus.PublicKeys...)

	for _, peer := range peers {
		consensus.validators.Range(func(k, v interface{}) bool {
			if p, ok := v.(p2p.Peer); ok {
				// We are using peer.IP and peer.Port to identify the unique peer
				// FIXME (lc): use a generic way to identify a peer
				if p.IP == peer.IP && p.Port == peer.Port {
					consensus.validators.Delete(k)
					count++
				}
				return true
			}
			return false
		})

		for i, pp := range newList {
			// Not Found the pubkey, if found pubkey, ignore it
			if reflect.DeepEqual(peer.ConsensusPubKey, pp) {
				//				consensus.Log.Debug("RemovePeers", "i", i, "pp", pp, "peer.PubKey", peer.PubKey)
				newList = append(newList[:i], newList[i+1:]...)
				count2++
			}
		}
	}

	if count2 > 0 {
		consensus.UpdatePublicKeys(newList)

		// Send out Pong messages to everyone in the shard to keep the publickeys in sync
		// Or the shard won't be able to reach consensus if public keys are mismatch

		validators := consensus.GetValidatorPeers()
		pong := proto_discovery.NewPongMessage(validators, consensus.PublicKeys, consensus.leader.ConsensusPubKey, consensus.ShardID)
		buffer := pong.ConstructPongMessage()

		consensus.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(consensus.ShardID))}, host.ConstructP2pMessage(byte(17), buffer))
	}

	return count2
}

// ToggleConsensusCheck flip the flag of whether ignore consensusID check during consensus process
func (consensus *Consensus) ToggleConsensusCheck() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.ignoreConsensusIDCheck = !consensus.ignoreConsensusIDCheck
}

// GetPeerByAddress the validator peer based on validator Address.
// TODO: deprecate this, as validators network info shouldn't known to everyone
func (consensus *Consensus) GetPeerByAddress(validatorAddress string) *p2p.Peer {
	v, ok := consensus.validators.Load(validatorAddress)
	if !ok {
		utils.GetLogInstance().Warn("Unrecognized validator", "validatorAddress", validatorAddress, "consensus", consensus)
		return nil
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		utils.GetLogInstance().Warn("Invalid validator", "validatorAddress", validatorAddress, "consensus", consensus)
		return nil
	}
	return &value
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (consensus *Consensus) IsValidatorInCommittee(validatorBlsAddress common.Address) bool {
	_, ok := consensus.CommitteeAddresses[validatorBlsAddress]
	return ok
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
	msgHash := sha256.Sum256(messageBytes)
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
	addrBytes := senderKey.GetAddress()
	senderAddr := common.BytesToAddress(addrBytes[:])

	if !consensus.IsValidatorInCommittee(senderAddr) {
		return nil, fmt.Errorf("Validator address %s is not in committee", senderAddr)
	}
	return senderKey, nil
}

// SetConsensusID set the consensusID to the height of the blockchain
func (consensus *Consensus) SetConsensusID(height uint32) {
	consensus.consensusID = height
}

// RegisterPRndChannel registers the channel for receiving randomness preimage from DRG protocol
func (consensus *Consensus) RegisterPRndChannel(pRndChannel chan []byte) {
	consensus.PRndChannel = pRndChannel
}

// RegisterRndChannel registers the channel for receiving final randomness from DRG protocol
func (consensus *Consensus) RegisterRndChannel(rndChannel chan [64]byte) {
	consensus.RndChannel = rndChannel
}

// Checks the basic meta of a consensus message, including the signature.
func (consensus *Consensus) checkConsensusMessage(message *msg_pb.Message, publicKey *bls.PublicKey) error {
	consensusMsg := message.GetConsensus()
	consensusID := consensusMsg.ConsensusId
	blockHash := consensusMsg.BlockHash

	// Verify message signature
	err := verifyMessageSig(publicKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to verify the message signature", "Error", err)
		return consensus_engine.ErrInvalidConsensusMessage
	}
	if !bytes.Equal(blockHash, consensus.blockHash[:]) {
		utils.GetLogInstance().Warn("Wrong blockHash", "consensus", consensus)
		return consensus_engine.ErrInvalidConsensusMessage
	}

	// just ignore consensus check for the first time when node join
	if consensus.ignoreConsensusIDCheck {
		consensus.consensusID = consensusID
		consensus.ToggleConsensusCheck()
		return nil
	} else if consensusID != consensus.consensusID {
		utils.GetLogInstance().Warn("Wrong consensus Id", "myConsensusId", consensus.consensusID, "theirConsensusId", consensusID, "consensus", consensus)
		// notify state syncing to start
		select {
		case consensus.ConsensusIDLowChan <- struct{}{}:
		default:
		}

		return consensus_engine.ErrConsensusIDNotMatch
	}
	return nil
}

// SetSeqNum sets the seqNum in consensus object, called at node bootstrap
func (consensus *Consensus) SetSeqNum(seqNum uint64) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.seqNum = seqNum
}
