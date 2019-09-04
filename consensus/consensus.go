// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/memprofiling"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/shard"
)

const (
	vdFAndProofSize = 516 // size of VDF and Proof
	vdfAndSeedSize  = 548 // size of VDF/Proof and Seed
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	// PbftLog stores the pbft messages and blocks during PBFT process
	PbftLog *PbftLog
	// phase: different phase of PBFT protocol: pre-prepare, prepare, commit, finish etc
	phase PbftPhase
	// mode: indicate a node is in normal or viewchanging mode
	mode PbftMode

	// epoch: current epoch number
	epoch uint64

	// blockNum: the next blockNumber that PBFT is going to agree on, should be equal to the blockNumber of next block
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
	prepareSigs          map[string]*bls.Sign // key is the bls public key
	commitSigs           map[string]*bls.Sign // key is the bls public key
	aggregatedPrepareSig *bls.Sign
	aggregatedCommitSig  *bls.Sign
	prepareBitmap        *bls_cosi.Mask
	commitBitmap         *bls_cosi.Mask

	// Commits collected from view change
	bhpSigs      map[string]*bls.Sign // bhpSigs: blockHashPreparedSigs is the signature on m1 type message
	nilSigs      map[string]*bls.Sign // nilSigs: there is no prepared message when view change, it's signature on m2 type (i.e. nil) messages
	viewIDSigs   map[string]*bls.Sign // viewIDSigs: every validator sign on |viewID|blockHash| in view changing message
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

	// Leader's address
	leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys          []*bls.PublicKey
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
	OnConsensusDone func(*types.Block)
	// The verifier func passed from Node object
	BlockVerifier func(*types.Block) error

	// verified block to state sync broadcast
	VerifiedNewBlock chan *types.Block

	// will trigger state syncing when blockNum is low
	blockNumLowChan chan struct{}

	// Channel for DRG protocol to send pRnd (preimage of randomness resulting from combined vrf randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	PRndChannel chan []byte
	// Channel for DRG protocol to send VDF. The first 516 bytes are the VDF/Proof and the last 32 bytes are the seed for deriving VDF
	RndChannel  chan [vdfAndSeedSize]byte
	pendingRnds [][vdfAndSeedSize]byte // A list of pending randomness

	uniqueIDInstance *utils.UniqueValidatorID

	// The p2p host used to send/receive p2p messages
	host p2p.Host
	// MessageSender takes are of sending consensus message and the corresponding retry logic.
	msgSender *MessageSender

	// Staking information finder
	stakeInfoFinder StakeInfoFinder

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

// StakeInfoFinder returns the stake information finder instance this
// consensus uses, e.g. for block reward distribution.
func (consensus *Consensus) StakeInfoFinder() StakeInfoFinder {
	return consensus.stakeInfoFinder
}

// SetStakeInfoFinder sets the stake information finder instance this
// consensus uses, e.g. for block reward distribution.
func (consensus *Consensus) SetStakeInfoFinder(stakeInfoFinder StakeInfoFinder) {
	consensus.stakeInfoFinder = stakeInfoFinder
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

// Quorum returns the consensus quorum of the current committee (2f+1).
func (consensus *Consensus) Quorum() int {
	return len(consensus.PublicKeys)*2/3 + 1
}

// PreviousQuorum returns the quorum size of previous epoch
func (consensus *Consensus) PreviousQuorum() int {
	return consensus.numPrevPubKeys*2/3 + 1
}

// VdfSeedSize returns the number of VRFs for VDF computation
func (consensus *Consensus) VdfSeedSize() int {
	return len(consensus.PublicKeys) * 2 / 3
}

// RewardThreshold returns the threshold to stop accepting commit messages
// when leader receives enough signatures for block reward
func (consensus *Consensus) RewardThreshold() int {
	return len(consensus.PublicKeys) * 9 / 10
}

// GetBlockReward returns last node block reward
func (consensus *Consensus) GetBlockReward() *big.Int {
	return consensus.lastBlockReward
}

// StakeInfoFinder finds the staking account for the given consensus key.
type StakeInfoFinder interface {
	// FindStakeInfoByNodeKey returns a list of staking information matching
	// the given node key.  Caller may modify the returned slice of StakeInfo
	// struct pointers, but must not modify the StakeInfo structs themselves.
	FindStakeInfoByNodeKey(key *bls.PublicKey) []*structs.StakeInfo

	// FindStakeInfoByAccount returns a list of staking information matching
	// the given account.  Caller may modify the returned slice of StakeInfo
	// struct pointers, but must not modify the StakeInfo structs themselves.
	FindStakeInfoByAccount(addr common.Address) []*structs.StakeInfo
}

// New creates a new Consensus object
// TODO: put shardId into chain reader's chain config
func New(host p2p.Host, ShardID uint32, leader p2p.Peer, blsPriKey *bls.SecretKey) (*Consensus, error) {
	consensus := Consensus{}
	consensus.host = host
	consensus.msgSender = NewMessageSender(host)
	consensus.blockNumLowChan = make(chan struct{})

	// pbft related
	consensus.PbftLog = NewPbftLog()
	consensus.phase = Announce
	consensus.mode = PbftMode{mode: Normal}
	// pbft timeout
	consensus.consensusTimeout = createTimeout()

	consensus.prepareSigs = map[string]*bls.Sign{}
	consensus.commitSigs = map[string]*bls.Sign{}

	consensus.CommitteePublicKeys = make(map[string]bool)

	consensus.validators.Store(leader.ConsensusPubKey.SerializeToHexStr(), leader)

	if blsPriKey != nil {
		consensus.priKey = blsPriKey
		consensus.PubKey = blsPriKey.GetPublicKey()
		utils.Logger().Info().Str("publicKey", consensus.PubKey.SerializeToHexStr()).Msg("My Public Key")
	} else {
		utils.Logger().Error().Msg("the bls key is nil")
		return nil, fmt.Errorf("nil bls key, aborting")
	}

	// viewID has to be initialized as the height of the blockchain during initialization
	// as it was displayed on explorer as Height right now
	consensus.viewID = 0
	consensus.ShardID = ShardID

	consensus.MsgChan = make(chan []byte)
	consensus.syncReadyChan = make(chan struct{})
	consensus.syncNotReadyChan = make(chan struct{})
	consensus.commitFinishChan = make(chan uint64)

	consensus.ReadySignal = make(chan struct{})
	consensus.lastBlockReward = big.NewInt(0)

	// channel for receiving newly generated VDF
	consensus.RndChannel = make(chan [vdfAndSeedSize]byte)

	consensus.uniqueIDInstance = utils.GetUniqueValidatorIDInstance()

	memprofiling.GetMemProfiling().Add("consensus.pbftLog", consensus.PbftLog)
	return &consensus, nil
}

// GenesisStakeInfoFinder is a stake info finder implementation using only
// genesis accounts.
// When used for block reward, it rewards only foundational nodes.
type GenesisStakeInfoFinder struct {
	byNodeKey map[shard.BlsPublicKey][]*structs.StakeInfo
	byAccount map[common.Address][]*structs.StakeInfo
}

// FindStakeInfoByNodeKey returns the genesis account matching the given node
// key, as a single-item StakeInfo list.
// It returns nil if the key is not a genesis node key.
func (f *GenesisStakeInfoFinder) FindStakeInfoByNodeKey(
	key *bls.PublicKey,
) []*structs.StakeInfo {
	var pk shard.BlsPublicKey
	if err := pk.FromLibBLSPublicKey(key); err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot convert BLS public key")
		return nil
	}
	l, _ := f.byNodeKey[pk]
	return l
}

// FindStakeInfoByAccount returns the genesis account matching the given
// address, as a single-item StakeInfo list.
// It returns nil if the address is not a genesis account.
func (f *GenesisStakeInfoFinder) FindStakeInfoByAccount(
	addr common.Address,
) []*structs.StakeInfo {
	l, _ := f.byAccount[addr]
	return l
}

// NewGenesisStakeInfoFinder returns a stake info finder that can look up
// genesis nodes.
func NewGenesisStakeInfoFinder() (*GenesisStakeInfoFinder, error) {
	f := &GenesisStakeInfoFinder{
		byNodeKey: make(map[shard.BlsPublicKey][]*structs.StakeInfo),
		byAccount: make(map[common.Address][]*structs.StakeInfo),
	}
	for idx, account := range genesis.HarmonyAccounts {
		pub := &bls.PublicKey{}
		pub.DeserializeHexStr(account.BlsPublicKey)
		var blsPublicKey shard.BlsPublicKey
		if err := blsPublicKey.FromLibBLSPublicKey(pub); err != nil {
			return nil, ctxerror.New("cannot convert BLS public key",
				"accountIndex", idx,
			).WithCause(err)
		}
		addressBytes, err := hexutil.Decode(account.Address)
		if err != nil {
			return nil, ctxerror.New("cannot decode account address",
				"accountIndex", idx,
			).WithCause(err)
		}
		var address common.Address
		address.SetBytes(addressBytes)
		stakeInfo := &structs.StakeInfo{
			Account:         address,
			BlsPublicKey:    blsPublicKey,
			BlockNum:        common.Big0,
			LockPeriodCount: big.NewInt(0x7fffffffffffffff),
			Amount:          common.Big0,
		}
		f.byNodeKey[blsPublicKey] = append(f.byNodeKey[blsPublicKey], stakeInfo)
		f.byAccount[address] = append(f.byAccount[address], stakeInfo)
	}
	return f, nil
}
