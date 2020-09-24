package consensus

import (
	"fmt"
	"sync"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/harmony-one/abool"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
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

var errLeaderPriKeyNotFound = errors.New("getting leader private key from consensus public keys failed")

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	Decider quorum.Decider
	// FBFTLog stores the pbft messages and blocks during FBFT process
	FBFTLog *FBFTLog
	// phase: different phase of FBFT protocol: pre-prepare, prepare, commit, finish etc
	phase FBFTPhase
	// current indicates what state a node is in
	current State
	// How long to delay sending commit messages.
	delayCommit time.Duration
	// Consensus rounds whose commit phase finished
	commitFinishChan chan uint64
	// 2 types of timeouts: normal and viewchange
	consensusTimeout map[TimeoutType]*utils.Timeout
	// Commits collected from validators.
	aggregatedPrepareSig *bls_core.Sign
	aggregatedCommitSig  *bls_core.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask
	// The chain reader for the blockchain this consensus is working on
	ChainReader *core.BlockChain
	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers   int
	pubKeyLock sync.Mutex
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
	mutex sync.Mutex
	// ViewChange struct
	VC *ViewChange
	// Signal channel for starting a new consensus process
	ReadySignal chan struct{}
	// The post-consensus processing func passed from Node object
	// Called when consensus on a new block is done
	OnConsensusDone func(*types.Block)
	// The verifier func passed from Node object
	BlockVerifier types.BlockVerifier
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
	// Used to convey to the consensus main loop that block syncing has finished.
	syncReadyChan chan struct{}
	// Used to convey to the consensus main loop that node is out of sync
	syncNotReadyChan chan struct{}
	// If true, this consensus will not propose view change.
	disableViewChange bool
	// Have a dedicated reader thread pull from this chan, like in node
	SlashChan chan slash.Record
	// How long in second the leader needs to wait to propose a new block.
	BlockPeriod time.Duration
	// The time due for next block proposal
	NextBlockDue time.Time
	// Temporary flag to control whether multi-sig signing is enabled
	MultiSig bool

	// TODO (leo): an new metrics system to keep track of the consensus/viewchange
	// finality of previous consensus in the unit of milliseconds
	finality int64
	// finalityCounter keep tracks of the finality time
	finalityCounter int64
}

// SetCommitDelay sets the commit message delay.  If set to non-zero,
// validator delays commit message by the amount.
func (consensus *Consensus) SetCommitDelay(delay time.Duration) {
	consensus.delayCommit = delay
}

// BlocksSynchronized lets the main loop know that block synchronization finished
// thus the blockchain is likely to be up to date.
func (consensus *Consensus) BlocksSynchronized() {
	consensus.syncReadyChan <- struct{}{}
}

// BlocksNotSynchronized lets the main loop know that block is not synchronized
func (consensus *Consensus) BlocksNotSynchronized() {
	consensus.syncNotReadyChan <- struct{}{}
}

// VdfSeedSize returns the number of VRFs for VDF computation
func (consensus *Consensus) VdfSeedSize() int {
	return int(consensus.Decider.ParticipantsCount()) * 2 / 3
}

// GetPublicKeys returns the public keys
func (consensus *Consensus) GetPublicKeys() multibls.PublicKeys {
	return consensus.priKey.GetPublicKeys()
}

// GetLeaderPrivateKey returns leader private key if node is the leader
func (consensus *Consensus) GetLeaderPrivateKey(leaderKey *bls_core.PublicKey) (*bls.PrivateKeyWrapper, error) {
	for i, key := range consensus.priKey {
		if key.Pub.Object.IsEqual(leaderKey) {
			return &consensus.priKey[i], nil
		}
	}
	return nil, errors.Wrapf(errLeaderPriKeyNotFound, leaderKey.SerializeToHexStr())
}

// GetConsensusLeaderPrivateKey returns consensus leader private key if node is the leader
func (consensus *Consensus) GetConsensusLeaderPrivateKey() (*bls.PrivateKeyWrapper, error) {
	return consensus.GetLeaderPrivateKey(consensus.LeaderPubKey.Object)
}

// New create a new Consensus record
func New(
	host p2p.Host, shard uint32, leader p2p.Peer, multiBLSPriKey multibls.PrivateKeys,
	Decider quorum.Decider,
) (*Consensus, error) {
	consensus := Consensus{}
	consensus.Decider = Decider
	consensus.host = host
	consensus.msgSender = NewMessageSender(host)
	consensus.BlockNumLowChan = make(chan struct{})
	// FBFT related
	consensus.FBFTLog = NewFBFTLog()
	consensus.phase = FBFTAnnounce
	// TODO Refactor consensus.block* into State?
	consensus.current = State{mode: Normal}
	// FBFT timeout
	consensus.consensusTimeout = createTimeout()

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
	consensus.ShardID = shard
	consensus.syncReadyChan = make(chan struct{})
	consensus.syncNotReadyChan = make(chan struct{})
	consensus.SlashChan = make(chan slash.Record)
	consensus.commitFinishChan = make(chan uint64)
	consensus.ReadySignal = make(chan struct{})
	// channel for receiving newly generated VDF
	consensus.RndChannel = make(chan [vdfAndSeedSize]byte)
	consensus.IgnoreViewIDCheck = abool.NewBool(false)
	// Make Sure Verifier is not null
	consensus.VC = NewViewChange()

	return &consensus, nil
}
