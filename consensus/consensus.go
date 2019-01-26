// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
	"golang.org/x/crypto/sha3"

	proto_node "github.com/harmony-one/harmony/api/proto/node"
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	// The current state of the consensus
	state State

	// Commits collected from validators.
	prepareSigs          *map[uint32]*bls.Sign
	commitSigs           *map[uint32]*bls.Sign
	aggregatedPrepareSig *bls.Sign
	aggregatedCommitSig  *bls.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask

	// map of nodeID to validator Peer object
	// FIXME: should use PubKey of p2p.Peer as the hashkey
	validators sync.Map // key is uint16, value is p2p.Peer

	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers int

	// Leader's address
	leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys []*bls.PublicKey
	pubKeyLock sync.Mutex

	// private/public keys of current node
	priKey *bls.SecretKey
	pubKey *bls.PublicKey

	// Whether I am leader. False means I am validator
	IsLeader bool
	// Leader or validator Id - 4 byte
	nodeID uint32
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

	// global consensus mutex
	mutex sync.Mutex

	// Validator specific fields
	// Blocks received but not done with consensus yet
	blocksReceived map[uint32]*BlockConsensusStatus

	// Signal channel for starting a new consensus process
	ReadySignal chan struct{}
	// The verifier func passed from Node object
	BlockVerifier func(*types.Block) bool
	// The post-consensus processing func passed from Node object
	// Called when consensus on a new block is done
	OnConsensusDone func(*types.Block)

	// current consensus block to check if out of sync
	ConsensusBlock chan *types.Block
	// verified block to state sync broadcast
	VerifiedNewBlock chan *types.Block

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

// New creates a new Consensus object
func New(host p2p.Host, ShardID string, peers []p2p.Peer, leader p2p.Peer) *Consensus {
	consensus := Consensus{}
	consensus.host = host

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		consensus.IsLeader = true
	} else {
		consensus.IsLeader = false
	}

	consensus.leader = leader
	for _, peer := range peers {
		consensus.validators.Store(utils.GetUniqueIDFromPeer(peer), peer)
	}

	consensus.prepareSigs = &map[uint32]*bls.Sign{}
	consensus.commitSigs = &map[uint32]*bls.Sign{}

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
	for _, validatorPeer := range peers {
		allPublicKeys = append(allPublicKeys, validatorPeer.PubKey)
	}
	allPublicKeys = append(allPublicKeys, leader.PubKey)

	consensus.PublicKeys = allPublicKeys

	prepareBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.PubKey)
	commitBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.PubKey)
	consensus.prepareBitmap = prepareBitmap
	consensus.commitBitmap = commitBitmap

	consensus.aggregatedPrepareSig = nil
	consensus.aggregatedCommitSig = nil

	// For now use socket address as ID
	// TODO: populate Id derived from address
	consensus.nodeID = utils.GetUniqueIDFromPeer(selfPeer)

	// Set private key for myself so that I can sign messages.
	nodeIDBytes := make([]byte, 32)
	binary.LittleEndian.PutUint32(nodeIDBytes, consensus.nodeID)
	privateKey := bls.SecretKey{}
	err := privateKey.SetLittleEndian(nodeIDBytes)
	consensus.priKey = &privateKey
	consensus.pubKey = privateKey.GetPublicKey()

	consensus.consensusID = 0 // or view Id in the original pbft paper

	myShardID, err := strconv.Atoi(ShardID)
	if err != nil {
		panic("Unparseable shard Id" + ShardID)
	}
	consensus.ShardID = uint32(myShardID)

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

	//	consensus.Log.Info("New Consensus", "IP", ip, "Port", port, "NodeID", consensus.nodeID, "priKey", consensus.priKey, "pubKey", consensus.pubKey)
	return &consensus
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
	for _, sig := range *consensus.prepareSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetCommitSigsArray returns the signatures for commit as a array
func (consensus *Consensus) GetCommitSigsArray() []*bls.Sign {
	sigs := []*bls.Sign{}
	for _, sig := range *consensus.commitSigs {
		sigs = append(sigs, sig)
	}
	return sigs
}

// ResetState resets the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.state = Finished
	consensus.prepareSigs = &map[uint32]*bls.Sign{}
	consensus.commitSigs = &map[uint32]*bls.Sign{}

	prepareBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.PubKey)
	commitBitmap, _ := bls_cosi.NewMask(consensus.PublicKeys, consensus.leader.PubKey)
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
	return fmt.Sprintf("[duty:%s, pubKey:%s, ShardID:%v, nodeID:%v, state:%s]",
		duty, hex.EncodeToString(consensus.pubKey.Serialize()), consensus.ShardID, consensus.nodeID, consensus.state)
}

// AddPeers adds new peers into the validator map of the consensus
// and add the public keys
func (consensus *Consensus) AddPeers(peers []*p2p.Peer) int {
	count := 0

	for _, peer := range peers {
		_, ok := consensus.validators.Load(utils.GetUniqueIDFromPeer(*peer))
		if !ok {
			if peer.ValidatorID == -1 {
				peer.ValidatorID = int(consensus.uniqueIDInstance.GetUniqueID())
			}
			consensus.validators.Store(utils.GetUniqueIDFromPeer(*peer), *peer)
			consensus.pubKeyLock.Lock()
			consensus.PublicKeys = append(consensus.PublicKeys, peer.PubKey)
			consensus.pubKeyLock.Unlock()
			utils.GetLogInstance().Debug("[SYNC] new peer added", "pubKey", peer.PubKey, "ip", peer.IP, "port", peer.Port)
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
			if reflect.DeepEqual(peer.PubKey, pp) {
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
		pong := proto_node.NewPongMessage(validators, consensus.PublicKeys)
		buffer := pong.ConstructPongMessage()

		host.BroadcastMessageFromLeader(consensus.host, validators, buffer, consensus.OfflinePeers)
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
			str2 := fmt.Sprintf("%s", p.PubKey.Serialize())
			utils.GetLogInstance().Debug("validator:", "IP", p.IP, "Port", p.Port, "VID", p.ValidatorID, "Key", str2)
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
	//	consensus.PublicKeys = make([]kyber.Point, len(pubKeys))
	consensus.PublicKeys = append(pubKeys[:0:0], pubKeys...)
	consensus.pubKeyLock.Unlock()

	return len(consensus.PublicKeys)
}

// NewFaker returns a faker consensus.
func NewFaker() *Consensus {
	return &Consensus{}
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock bft engine.
func (consensus *Consensus) VerifyHeader(chain ChainReader, header *types.Header, seal bool) error {
	// TODO: implement this
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (consensus *Consensus) VerifyHeaders(chain ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))
	for i := 0; i < len(headers); i++ {
		results <- nil
	}
	return abort, results
}

func (consensus *Consensus) verifyHeaderWorker(chain ChainReader, headers []*types.Header, seals []bool, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
		return nil // known block
	}
	return consensus.verifyHeader(chain, headers[index], parent, false, seals[index])
}

// verifyHeader checks whether a header conforms to the consensus rules of the
// stock bft engine.
func (consensus *Consensus) verifyHeader(chain ChainReader, header, parent *types.Header, uncle bool, seal bool) error {
	return nil
}

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (consensus *Consensus) VerifySeal(chain ChainReader, header *types.Header) error {
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (consensus *Consensus) Finalize(chain ChainReader, header *types.Header, state *state.DB, txs []*types.Transaction, receipts []*types.Receipt) (*types.Block, error) {
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
func (consensus *Consensus) Seal(chain ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	// TODO: implement final block sealing
	return nil
}

// Prepare is to prepare ...
// TODO(RJ): fix it.
func (consensus *Consensus) Prepare(chain ChainReader, header *types.Header) error {
	// TODO: implement prepare method
	return nil
}

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(config *params.ChainConfig, state *state.DB, header *types.Header) {
	// TODO: implement mining rewards
}

// GetNodeID returns the nodeID
func (consensus *Consensus) GetNodeID() uint32 {
	return consensus.nodeID
}

// GetPeerFromID will get peer from peerID, bool value in return true means success and false means fail
func (consensus *Consensus) GetPeerFromID(peerID uint32) (p2p.Peer, bool) {
	v, ok := consensus.validators.Load(peerID)
	if !ok {
		return p2p.Peer{}, false
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		return p2p.Peer{}, false
	}
	return value, true
}

// SendMessage sends message thru p2p host to peer.
func (consensus *Consensus) SendMessage(peer p2p.Peer, message []byte) {
	host.SendMessage(consensus.host, peer, message, nil)
}
