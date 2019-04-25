// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
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
// TODO: put shardId into chain reader's chain config
func New(host p2p.Host, ShardID uint32, leader p2p.Peer, blsPriKey *bls.SecretKey) (*Consensus, error) {
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

	consensus.prepareSigs = map[common.Address]*bls.Sign{}
	consensus.commitSigs = map[common.Address]*bls.Sign{}

	consensus.validators.Store(utils.GetBlsAddress(leader.ConsensusPubKey).Hex(), leader)
	consensus.CommitteeAddresses[utils.GetBlsAddress(leader.ConsensusPubKey)] = true

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
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

	// consensusID has to be initialized as the height of the blockchain during initialization
	// as it was displayed on explorer as Height right now
	consensus.consensusID = 0
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

	//	consensus.Log.Info("New Consensus", "IP", ip, "Port", port, "NodeID", consensus.nodeID, "priKey", consensus.priKey, "PubKey", consensus.PubKey)
	return &consensus, nil
}
