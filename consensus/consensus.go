package consensus

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/registry"

	"github.com/harmony-one/abool"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/multibls"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/staking/slash"
	"github.com/pkg/errors"
)

const (
	vdFAndProofSize = 516 // size of VDF and Proof
	vdfAndSeedSize  = 548 // size of VDF/Proof and Seed
)

var errLeaderPriKeyNotFound = errors.New("leader private key not found locally")

// ProposalType is to indicate the type of signal for new block proposal
type ProposalType byte

// Constant of the type of new block proposal
const (
	SyncProposal ProposalType = iota
	AsyncProposal
)

// VerifyBlockFunc is a function used to verify the block and keep trace of verified blocks
type VerifyBlockFunc func(*types.Block) error

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	Decider quorum.Decider
	// FBFTLog stores the pbft messages and blocks during FBFT process
	fBFTLog *FBFTLog
	// phase: different phase of FBFT protocol: pre-prepare, prepare, commit, finish etc
	phase FBFTPhase
	// current indicates what state a node is in
	current State
	// isBackup declarative the node is in backup mode
	isBackup bool
	// 2 types of timeouts: normal and viewchange
	consensusTimeout map[TimeoutType]*utils.Timeout
	// Commits collected from validators.
	aggregatedPrepareSig *bls_core.Sign
	aggregatedCommitSig  *bls_core.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask

	multiSigBitmap *bls_cosi.Mask // Bitmap for parsing multisig bitmap from validators

	// Registry for services.
	registry *registry.Registry
	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers int
	// private/public keys of current node
	priKey multibls.PrivateKeys
	// the publickey of leader
	LeaderPubKey *bls.PublicKeyWrapper
	// blockNum: the next blockNumber that FBFT is going to agree on,
	// should be equal to the blockNumber of next block
	blockNum uint64
	// Blockhash - 32 byte
	blockHash [32]byte
	// Block to run consensus on
	block []byte
	// Shard Id which this node belongs to
	ShardID uint32
	// IgnoreViewIDCheck determines whether to ignore viewID check
	IgnoreViewIDCheck *abool.AtomicBool
	// consensus mutex
	mutex *sync.RWMutex
	// ViewChange struct
	vc *viewChange
	// Signal channel for proposing a new block and start new consensus
	readySignal chan ProposalType
	// Channel to send full commit signatures to finish new block proposal
	commitSigChannel chan []byte
	// The post-consensus job func passed from Node object
	// Called when consensus on a new block is done
	PostConsensusJob func(*types.Block) error
	// The verifier func passed from Node object
	BlockVerifier VerifyBlockFunc
	// verified block to state sync broadcast
	VerifiedNewBlock chan *types.Block
	// will trigger state syncing when blockNum is low
	BlockNumLowChan chan struct{}
	// Channel for DRG protocol to send pRnd (preimage of randomness resulting from combined vrf
	// randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	PRndChannel chan []byte
	// Channel for DRG protocol to send VDF. The first 516 bytes are the VDF/Proof and the last 32
	// bytes are the seed for deriving VDF
	RndChannel  chan [vdfAndSeedSize]byte
	pendingRnds [][vdfAndSeedSize]byte // A list of pending randomness
	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// MessageSender takes are of sending consensus message and the corresponding retry logic.
	msgSender *MessageSender
	// If true, this consensus will not propose view change.
	disableViewChange bool
	// Have a dedicated reader thread pull from this chan, like in node
	SlashChan chan slash.Record
	// How long in second the leader needs to wait to propose a new block.
	BlockPeriod time.Duration
	// The time due for next block proposal
	NextBlockDue time.Time
	// Temporary flag to control whether aggregate signature signing is enabled
	AggregateSig bool

	// TODO (leo): an new metrics system to keep track of the consensus/viewchange
	// finality of previous consensus in the unit of milliseconds
	finality int64
	// finalityCounter keep tracks of the finality time
	finalityCounter atomic.Value //int64

	dHelper *downloadHelper

	// Both flags only for initialization state.
	start           bool
	isInitialLeader bool
}

// Blockchain returns the blockchain.
func (consensus *Consensus) Blockchain() core.BlockChain {
	return consensus.registry.GetBlockchain()
}

func (consensus *Consensus) FBFTLog() FBFT {
	return threadsafeFBFTLog{
		log: consensus.fBFTLog,
		mu:  consensus.mutex,
	}
}

// ChainReader returns the chain reader.
// This is mostly the same as Blockchain, but it returns only read methods, so we assume it's safe for concurrent use.
func (consensus *Consensus) ChainReader() engine.ChainReader {
	return consensus.Blockchain()
}

func (consensus *Consensus) ReadySignal(p ProposalType) {
	consensus.readySignal <- p
}

func (consensus *Consensus) GetReadySignal() chan ProposalType {
	return consensus.readySignal
}

func (consensus *Consensus) GetCommitSigChannel() chan []byte {
	return consensus.commitSigChannel
}

// Beaconchain returns the beaconchain.
func (consensus *Consensus) Beaconchain() core.BlockChain {
	return consensus.registry.GetBeaconchain()
}

// VerifyBlock is a function used to verify the block and keep trace of verified blocks.
func (consensus *Consensus) verifyBlock(block *types.Block) error {
	if !consensus.fBFTLog.IsBlockVerified(block.Hash()) {
		if err := consensus.BlockVerifier(block); err != nil {
			return errors.Errorf("Block verification failed: %s", err)
		}
		consensus.fBFTLog.MarkBlockVerified(block)
	}
	return nil
}

// BlocksSynchronized lets the main loop know that block synchronization finished
// thus the blockchain is likely to be up to date.
func (consensus *Consensus) BlocksSynchronized() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.syncReadyChan()
}

// BlocksNotSynchronized lets the main loop know that block is not synchronized
func (consensus *Consensus) BlocksNotSynchronized() {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.syncNotReadyChan()
}

// VdfSeedSize returns the number of VRFs for VDF computation
func (consensus *Consensus) VdfSeedSize() int {
	return int(consensus.Decider.ParticipantsCount()) * 2 / 3
}

// GetPublicKeys returns the public keys
func (consensus *Consensus) GetPublicKeys() multibls.PublicKeys {
	return consensus.getPublicKeys()
}

func (consensus *Consensus) getPublicKeys() multibls.PublicKeys {
	return consensus.priKey.GetPublicKeys()
}

func (consensus *Consensus) GetLeaderPubKey() *bls_cosi.PublicKeyWrapper {
	consensus.mutex.RLock()
	defer consensus.mutex.RUnlock()
	return consensus.getLeaderPubKey()
}

func (consensus *Consensus) getLeaderPubKey() *bls_cosi.PublicKeyWrapper {
	return consensus.LeaderPubKey
}

func (consensus *Consensus) SetLeaderPubKey(pub *bls_cosi.PublicKeyWrapper) {
	consensus.mutex.Lock()
	defer consensus.mutex.Unlock()
	consensus.setLeaderPubKey(pub)
}

func (consensus *Consensus) setLeaderPubKey(pub *bls_cosi.PublicKeyWrapper) {
	consensus.LeaderPubKey = pub
}

func (consensus *Consensus) GetPrivateKeys() multibls.PrivateKeys {
	return consensus.priKey
}

// GetLeaderPrivateKey returns leader private key if node is the leader
func (consensus *Consensus) getLeaderPrivateKey(leaderKey *bls_core.PublicKey) (*bls.PrivateKeyWrapper, error) {
	for i, key := range consensus.priKey {
		if key.Pub.Object.IsEqual(leaderKey) {
			return &consensus.priKey[i], nil
		}
	}
	return nil, errors.Wrapf(errLeaderPriKeyNotFound, leaderKey.SerializeToHexStr())
}

// getConsensusLeaderPrivateKey returns consensus leader private key if node is the leader
func (consensus *Consensus) getConsensusLeaderPrivateKey() (*bls.PrivateKeyWrapper, error) {
	return consensus.getLeaderPrivateKey(consensus.LeaderPubKey.Object)
}

func (consensus *Consensus) IsBackup() bool {
	return consensus.isBackup
}

func (consensus *Consensus) BlockNum() uint64 {
	return atomic.LoadUint64(&consensus.blockNum)
}

func (consensus *Consensus) getBlockNum() uint64 {
	return atomic.LoadUint64(&consensus.blockNum)
}

// New create a new Consensus record
func New(
	host p2p.Host, shard uint32, multiBLSPriKey multibls.PrivateKeys,
	registry *registry.Registry,
	Decider quorum.Decider, minPeers int, aggregateSig bool,
) (*Consensus, error) {
	consensus := Consensus{
		mutex:           &sync.RWMutex{},
		ShardID:         shard,
		fBFTLog:         NewFBFTLog(),
		phase:           FBFTAnnounce,
		current:         State{mode: Normal},
		Decider:         Decider,
		registry:        registry,
		MinPeers:        minPeers,
		AggregateSig:    aggregateSig,
		host:            host,
		msgSender:       NewMessageSender(host),
		BlockNumLowChan: make(chan struct{}, 1),
		// FBFT timeout
		consensusTimeout: createTimeout(),
	}

	if multiBLSPriKey != nil {
		consensus.priKey = multiBLSPriKey
		utils.Logger().Info().
			Str("publicKey", consensus.GetPublicKeys().SerializeToHexStr()).Msg("My Public Key")
	} else {
		utils.Logger().Error().Msg("the bls key is nil")
		return nil, fmt.Errorf("nil bls key, aborting")
	}

	// viewID has to be initialized as the height of
	// the blockchain during initialization as it was
	// displayed on explorer as Height right now
	consensus.SetCurBlockViewID(0)
	consensus.SlashChan = make(chan slash.Record)
	consensus.readySignal = make(chan ProposalType)
	consensus.commitSigChannel = make(chan []byte)
	// channel for receiving newly generated VDF
	consensus.RndChannel = make(chan [vdfAndSeedSize]byte)
	consensus.IgnoreViewIDCheck = abool.NewBool(false)
	// Make Sure Verifier is not null
	consensus.vc = newViewChange()
	// TODO: reference to blockchain/beaconchain should be removed.
	verifier := VerifyNewBlock(registry.GetWebHooks(), consensus.Blockchain(), consensus.Beaconchain())
	consensus.BlockVerifier = verifier
	consensus.vc.verifyBlock = consensus.verifyBlock

	// init prometheus metrics
	initMetrics()
	consensus.AddPubkeyMetrics()

	return &consensus, nil
}

func (consensus *Consensus) GetHost() p2p.Host {
	return consensus.host
}

func (consensus *Consensus) Registry() *registry.Registry {
	return consensus.registry
}
