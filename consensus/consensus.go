// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
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
	"golang.org/x/crypto/sha3"
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	// The current state of the consensus
	state State

	// Commits collected from validators.
	prepareSigs          map[common.Address]*bls.Sign // key is the validator's address
	commitSigs           map[common.Address]*bls.Sign // key is the validator's address
	aggregatedPrepareSig *bls.Sign
	aggregatedCommitSig  *bls.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask

	// The chain reader for the blockchain this consensus is working on
	ChainReader consensus_engine.ChainReader

	// map of nodeID to validator Peer object
	validators sync.Map // key is the hex string of the blsKey, value is p2p.Peer

	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers int

	// Leader's address
	leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys []*bls.PublicKey
	// The addresses of my committee
	CommitteeAddresses map[common.Address]bool
	pubKeyLock         sync.Mutex

	// private/public keys of current node
	priKey *bls.SecretKey
	PubKey *bls.PublicKey

	// Whether I am leader. False means I am validator
	IsLeader bool
	// Leader or validator address in hex
	SelfAddress common.Address
	// Consensus Id (View Id) - 4 byte
	consensusID uint32
	// Blockhash - 32 byte
	blockHash [32]byte
	// Block to run consensus on
	block []byte
	// Array of block hashes.
	blockHashes [][32]byte
	// Shard Id which this node belongs to
	ShardID uint32
	// whether to ignore consensusID check
	ignoreConsensusIDCheck bool

	// global consensus mutex
	mutex sync.Mutex

	// Validator specific fields
	// Blocks received but not done with consensus yet
	blocksReceived map[uint32]*BlockConsensusStatus

	// Signal channel for starting a new consensus process
	ReadySignal chan struct{}
	// The post-consensus processing func passed from Node object
	// Called when consensus on a new block is done
	OnConsensusDone func(*types.Block)
	// The verifier func passed from Node object
	BlockVerifier func(*types.Block) bool

	// verified block to state sync broadcast
	VerifiedNewBlock chan *types.Block

	// will trigger state syncing when consensus ID is low
	ConsensusIDLowChan chan struct{}

	// Channel for DRG protocol to send pRnd (preimage of randomness resulting from combined vrf randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	PRndChannel chan []byte
	// Channel for DRG protocol to send the final randomness to consensus. The first 32 bytes are the randomness and the last 32 bytes are the hash of the block where the corresponding pRnd was generated
	RndChannel  chan [64]byte
	pendingRnds [][64]byte // A list of pending randomness

	uniqueIDInstance *utils.UniqueValidatorID

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Signal channel for lost validators
	OfflinePeers chan p2p.Peer

	// List of offline Peers
	OfflinePeerList []p2p.Peer
}

// BlockConsensusStatus used to keep track of the consensus status of multiple blocks received so far
// This is mainly used in the case that this node is lagging behind and needs to catch up.
// For example, the consensus moved to round N and this node received message(N).
// However, this node may still not finished with round N-1, so the newly received message(N)
// should be stored in this temporary structure. In case the round N-1 finishes, it can catch
// up to the latest state of round N by using this structure.
type BlockConsensusStatus struct {
	block []byte // the block data
	state State  // the latest state of the consensus
}

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

// New creates a new Consensus object
// TODO: put shardId into chain reader's chain config
func New(host p2p.Host, ShardID uint32, peers []p2p.Peer, leader p2p.Peer, blsPriKey *bls.SecretKey) (*Consensus, error) {
	consensus := Consensus{}
	consensus.host = host
	consensus.ConsensusIDLowChan = make(chan struct{})

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		consensus.IsLeader = true
	} else {
		consensus.IsLeader = false
	}

	consensus.leader = leader
	consensus.CommitteeAddresses = map[common.Address]bool{}
	for _, peer := range peers {
		consensus.validators.Store(utils.GetBlsAddress(peer.ConsensusPubKey).Hex(), peer)

		consensus.CommitteeAddresses[utils.GetBlsAddress(peer.ConsensusPubKey)] = true
	}

	consensus.prepareSigs = map[common.Address]*bls.Sign{}
	consensus.commitSigs = map[common.Address]*bls.Sign{}

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
	for _, validatorPeer := range peers {
		allPublicKeys = append(allPublicKeys, validatorPeer.ConsensusPubKey)
	}
	allPublicKeys = append(allPublicKeys, leader.ConsensusPubKey)

	consensus.PublicKeys = allPublicKeys

	prepareBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	if err != nil {
		return nil, err
	}
	commitBitmap, err := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	if err != nil {
		return nil, err
	}
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap

	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil

	// For now use socket address as ID
	// TODO: populate Id derived from address
	consensus.SelfAddress = utils.GetBlsAddress(selfPeer.ConsensusPubKey)

	if blsPriKey != nil {
		consensus.priKey = blsPriKey
		consensus.PubKey = blsPriKey.GetPublicKey()
	}

	consensus.consensusID = 0 // or view Id in the original pbft paper
	consensus.ShardID = ShardID

	// For validators to keep track of all blocks received but not yet committed, so as to catch up to latest consensus if lagged behind.
	consensus.blocksReceived = make(map[uint32]*BlockConsensusStatus)

	if consensus.IsLeader {
		consensus.ReadySignal = make(chan struct{})
		// send a signal to indicate it's ready to run consensus
		// this signal is consumed by node object to create a new block and in turn trigger a new consensus on it
		// this is a goroutine because go channel without buffer will block
		go func() {
			consensus.ReadySignal <- struct{}{}
		}()
	}

	consensus.uniqueIDInstance = utils.GetUniqueValidatorIDInstance()
	consensus.OfflinePeerList = make([]p2p.Peer, 0)

	//	consensus.Log.Info("New Consensus", "IP", ip, "Port", port, "NodeID", consensus.nodeID, "priKey", consensus.priKey, "PubKey", consensus.PubKey)
	return &consensus, nil
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
	return nil
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
	consensus.prepareSigs = map[common.Address]*bls.Sign{}
	consensus.commitSigs = map[common.Address]*bls.Sign{}

	prepareBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	commitBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.ConsensusPubKey)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap

	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil

	// Clear the OfflinePeersList again
	consensus.OfflinePeerList = make([]p2p.Peer, 0)
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
	accumulateRewards(chain.Config(), state, header)
	header.Root = state.IntermediateRoot(false)
	return types.NewBlock(header, txs, receipts), nil
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

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(config *params.ChainConfig, state *state.DB, header *types.Header) {
	// TODO: implement mining rewards
}

// GetSelfAddress returns the address in hex
func (consensus *Consensus) GetSelfAddress() common.Address {
	return consensus.SelfAddress
}

// Populates the common basic fields for all consensus message.
func (consensus *Consensus) populateMessageFields(request *msg_pb.ConsensusRequest) {
	// TODO(minhdoan): Maybe look into refactor this.
	// 4 byte consensus id
	request.ConsensusId = consensus.consensusID

	// 32 byte block hash
	request.BlockHash = consensus.blockHash[:]

	// sender address
	request.SenderPubkey = consensus.PubKey.Serialize()

	utils.GetLogInstance().Debug("[populateMessageFields]", "myConsensusID", consensus.consensusID, "SenderAddress", consensus.SelfAddress)
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

// GetPeers returns PublicKeys
func (consensus *Consensus) GetPeers() []*bls.PublicKey {
	return consensus.PublicKeys
}

// GetConsensusID returns the consensus ID
func (consensus *Consensus) GetConsensusID() uint32 {
	return consensus.consensusID
}
