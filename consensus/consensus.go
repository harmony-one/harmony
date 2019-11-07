package consensus

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/memprofiling"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

const (
	vdFAndProofSize = 516 // size of VDF and Proof
	vdfAndSeedSize  = 548 // size of VDF/Proof and Seed
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	Decider quorum.Decider

	// FBFTLog stores the pbft messages and blocks during FBFT process
	FBFTLog *FBFTLog
	// phase: different phase of FBFT protocol: pre-prepare, prepare, commit, finish etc
	phase FBFTPhase
	// current indicates what state a node is in
	current State

	// epoch: current epoch number
	epoch uint64

	// blockNum: the next blockNumber that FBFT is going to agree on,
	// should be equal to the blockNumber of next block
	blockNum uint64
	// channel to receive consensus message
	MsgChan chan []byte

	// How long to delay sending commit messages.
	delayCommit time.Duration

	// Consensus rounds whose commit phase finished
	commitFinishChan chan uint64

	// 2 types of timeouts: normal and viewchange
	consensusTimeout map[TimeoutType]*utils.Timeout

	// Commits collected from validators.
	aggregatedPrepareSig *bls.Sign
	aggregatedCommitSig  *bls.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask

	// Commits collected from view change
	// bhpSigs: blockHashPreparedSigs is the signature on m1 type message
	bhpSigs map[string]*bls.Sign
	// nilSigs: there is no prepared message when view change,
	// it's signature on m2 type (i.e. nil) messages
	nilSigs      map[string]*bls.Sign
	bhpBitmap    *bls_cosi.Mask
	nilBitmap    *bls_cosi.Mask
	viewIDBitmap *bls_cosi.Mask
	m1Payload    []byte     // message payload for type m1 := |vcBlockHash|prepared_agg_sigs|prepared_bitmap|, new leader only need one
	vcLock       sync.Mutex // mutex for view change

	// The chain reader for the blockchain this consensus is working on
	ChainReader *core.BlockChain

	// map of nodeID to validator Peer object
	validators sync.Map // key is the hex string of the blsKey, value is p2p.Peer

	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers int

	CommitteePublicKeys map[string]bool

	pubKeyLock sync.Mutex

	// private/public keys of current node
	priKey *bls.SecretKey
	PubKey *bls.PublicKey

	SelfAddress common.Address
	// the publickey of leader
	LeaderPubKey *bls.PublicKey

	// number of publickeys of previous epoch
	numPrevPubKeys int

	viewID uint64

	// Blockhash - 32 byte
	blockHash [32]byte
	// Block to run consensus on
	block []byte
	// BlockHeader to run consensus on
	blockHeader []byte
	// Array of block hashes.
	blockHashes [][32]byte
	// Shard Id which this node belongs to
	ShardID uint32
	// whether to ignore viewID check
	ignoreViewIDCheck bool

	// global consensus mutex
	mutex sync.Mutex

	// consensus information update mutex
	infoMutex sync.Mutex

	// Signal channel for starting a new consensus process
	ReadySignal chan struct{}
	// The post-consensus processing func passed from Node object
	// Called when consensus on a new block is done
	OnConsensusDone func(*types.Block, []byte)
	// The verifier func passed from Node object
	BlockVerifier func(*types.Block) error

	// verified block to state sync broadcast
	VerifiedNewBlock chan *types.Block

	// will trigger state syncing when blockNum is low
	blockNumLowChan chan struct{}

	// Channel for DRG protocol to send pRnd (preimage of randomness resulting from combined vrf
	// randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	PRndChannel chan []byte
	// Channel for DRG protocol to send VDF. The first 516 bytes are the VDF/Proof and the last 32
	// bytes are the seed for deriving VDF
	RndChannel  chan [vdfAndSeedSize]byte
	pendingRnds [][vdfAndSeedSize]byte // A list of pending randomness

	uniqueIDInstance *utils.UniqueValidatorID

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

	// last node block reward for metrics
	lastBlockReward *big.Int
}

// SetCommitDelay sets the commit message delay.  If set to non-zero,
// validator delays commit message by the amount.
func (consensus *Consensus) SetCommitDelay(delay time.Duration) {
	consensus.delayCommit = delay
}

// DisableViewChangeForTestingOnly makes the receiver not propose view
// changes when it should, e.g. leader timeout.
//
// As the name implies, this is intended for testing only,
// and should not be used on production network.
// This is also not part of the long-term consensus API and may go away later.
func (consensus *Consensus) DisableViewChangeForTestingOnly() {
	consensus.disableViewChange = true
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

// WaitForSyncing informs the node syncing service to start syncing
func (consensus *Consensus) WaitForSyncing() {
	<-consensus.blockNumLowChan
}

// VdfSeedSize returns the number of VRFs for VDF computation
func (consensus *Consensus) VdfSeedSize() int {
	return int(consensus.Decider.ParticipantsCount()) * 2 / 3
}

// GetBlockReward returns last node block reward
func (consensus *Consensus) GetBlockReward() *big.Int {
	return consensus.lastBlockReward
}

// TODO: put shardId into chain reader's chain config

// New create a new Consensus record
func New(
	host p2p.Host, shard uint32, leader p2p.Peer, blsPriKey *bls.SecretKey,
	Decider quorum.Decider,
) (*Consensus, error) {
	consensus := Consensus{}
	consensus.Decider = Decider
	consensus.host = host
	consensus.msgSender = NewMessageSender(host)
	consensus.blockNumLowChan = make(chan struct{})
	// FBFT related
	consensus.FBFTLog = NewFBFTLog()
	consensus.phase = FBFTAnnounce
	consensus.current = State{mode: Normal}
	// FBFT timeout
	consensus.consensusTimeout = createTimeout()
	consensus.CommitteePublicKeys = make(map[string]bool)
	consensus.validators.Store(leader.ConsensusPubKey.SerializeToHexStr(), leader)

	if blsPriKey != nil {
		consensus.priKey = blsPriKey
		consensus.PubKey = blsPriKey.GetPublicKey()
		utils.Logger().Info().
			Str("publicKey", consensus.PubKey.SerializeToHexStr()).Msg("My Public Key")
	} else {
		utils.Logger().Error().Msg("the bls key is nil")
		return nil, fmt.Errorf("nil bls key, aborting")
	}

	// viewID has to be initialized as the height of
	// the blockchain during initialization as it was
	// displayed on explorer as Height right now
	consensus.viewID = 0
	consensus.ShardID = shard
	consensus.MsgChan = make(chan []byte)
	consensus.syncReadyChan = make(chan struct{})
	consensus.syncNotReadyChan = make(chan struct{})
	consensus.commitFinishChan = make(chan uint64)
	consensus.ReadySignal = make(chan struct{})
	consensus.lastBlockReward = big.NewInt(0)
	// channel for receiving newly generated VDF
	consensus.RndChannel = make(chan [vdfAndSeedSize]byte)
	consensus.uniqueIDInstance = utils.GetUniqueValidatorIDInstance()
	memprofiling.GetMemProfiling().Add("consensus.FBFTLog", consensus.FBFTLog)
	return &consensus, nil
}
