// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/genesis"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// Block reward per block signature.
// TODO ek – per sig per stake
var (
	BlockReward = big.NewInt(denominations.One / 10)
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	// pbftLog stores the pbft messages and blocks during PBFT process
	pbftLog *PbftLog
	// phase: different phase of PBFT protocol: pre-prepare, prepare, commit, finish etc
	phase PbftPhase
	// mode: indicate a node is in normal or viewchanging mode
	mode PbftMode
	// blockNum: the next blockNumber that PBFT is going to agree on, should be equal to the blockNumber of next block
	blockNum uint64
	// channel to receive consensus message
	MsgChan chan []byte

	// How long to delay sending commit messages.
	delayCommit time.Duration

	// Consensus rounds whose commit phase finished
	commitFinishChan chan uint32

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
	ChainReader consensus_engine.ChainReader

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

	// Consensus Id (View Id) - 4 byte
	viewID uint32 // TODO(chao): change it to uint64 or add overflow checking mechanism

	// Blockhash - 32 byte
	blockHash [32]byte
	// Block to run consensus on
	block []byte
	// Array of block hashes.
	blockHashes [][32]byte
	// Shard Id which this node belongs to
	ShardID uint32
	// whether to ignore viewID check
	ignoreViewIDCheck bool

	// global consensus mutex
	mutex sync.Mutex

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
	// Channel for DRG protocol to send the final randomness to consensus. The first 32 bytes are the randomness and the last 32 bytes are the hash of the block where the corresponding pRnd was generated
	RndChannel  chan [64]byte
	pendingRnds [][64]byte // A list of pending randomness

	uniqueIDInstance *utils.UniqueValidatorID

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Staking information finder
	stakeInfoFinder StakeInfoFinder

	// Used to convey to the consensus main loop that block syncing has finished.
	syncReadyChan chan struct{}
	// Used to convey to the consensus main loop that node is out of sync
	syncNotReadyChan chan struct{}

	// If true, this consensus will not propose view change.
	disableViewChange bool
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

// RewardThreshold returns the threshold to stop accepting commit messages
// when leader receives enough signatures for block reward
func (consensus *Consensus) RewardThreshold() int {
	return len(consensus.PublicKeys) * 9 / 10
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
	consensus.blockNumLowChan = make(chan struct{})

	// pbft related
	consensus.pbftLog = NewPbftLog()
	consensus.phase = Announce
	consensus.mode = PbftMode{mode: Normal}
	// pbft timeout
	consensus.consensusTimeout = createTimeout()

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		nodeconfig.GetDefaultConfig().SetIsLeader(true)
	} else {
		nodeconfig.GetDefaultConfig().SetIsLeader(false)
	}

	consensus.prepareSigs = map[string]*bls.Sign{}
	consensus.commitSigs = map[string]*bls.Sign{}

	consensus.CommitteePublicKeys = make(map[string]bool)

	consensus.validators.Store(leader.ConsensusPubKey.SerializeToHexStr(), leader)

	consensus.SelfAddress = utils.GetBlsAddress(selfPeer.ConsensusPubKey)

	if blsPriKey != nil {
		consensus.priKey = blsPriKey
		consensus.PubKey = blsPriKey.GetPublicKey()
		utils.GetLogInstance().Info("my pubkey is", "pubkey", consensus.PubKey.SerializeToHexStr())
	}

	// viewID has to be initialized as the height of the blockchain during initialization
	// as it was displayed on explorer as Height right now
	consensus.viewID = 0
	consensus.ShardID = ShardID

	consensus.MsgChan = make(chan []byte)
	consensus.syncReadyChan = make(chan struct{})
	consensus.syncNotReadyChan = make(chan struct{})
	consensus.commitFinishChan = make(chan uint32)

	consensus.ReadySignal = make(chan struct{})
	if nodeconfig.GetDefaultConfig().IsLeader() {
		// send a signal to indicate it's ready to run consensus
		// this signal is consumed by node object to create a new block and in turn trigger a new consensus on it
		go func() {
			consensus.ReadySignal <- struct{}{}
		}()
	}

	consensus.uniqueIDInstance = utils.GetUniqueValidatorIDInstance()

	return &consensus, nil
}

// accumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func accumulateRewards(
	bc consensus_engine.ChainReader, state *state.DB, header *types.Header,
) error {
	logger := header.Logger(utils.GetLogInstance())
	getLogger := func() log.Logger { return utils.WithCallerSkip(logger, 1) }
	blockNum := header.Number.Uint64()
	if blockNum == 0 {
		// Epoch block has no parent to reward.
		return nil
	}
	// TODO ek – retrieving by parent number (blockNum - 1) doesn't work,
	//  while it is okay with hash.  Sounds like DB inconsistency.
	//  Figure out why.
	parentHeader := bc.GetHeaderByHash(header.ParentHash)
	if parentHeader == nil {
		return ctxerror.New("cannot find parent block header in DB",
			"parentHash", header.ParentHash)
	}
	if parentHeader.Number.Cmp(common.Big0) == 0 {
		// Parent is an epoch block,
		// which is not signed in the usual manner therefore rewards nothing.
		return nil
	}
	parentShardState, err := bc.ReadShardState(parentHeader.Epoch)
	if err != nil {
		return ctxerror.New("cannot read shard state",
			"epoch", parentHeader.Epoch,
		).WithCause(err)
	}
	parentCommittee := parentShardState.FindCommitteeByID(parentHeader.ShardID)
	if parentCommittee == nil {
		return ctxerror.New("cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number,
			"shardID", parentHeader.ShardID,
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range parentCommittee.NodeList {
		committerKey := new(bls.PublicKey)
		err := member.BlsPublicKey.ToLibBLSPublicKey(committerKey)
		if err != nil {
			return ctxerror.New("cannot convert BLS public key",
				"blsPublicKey", member.BlsPublicKey).WithCause(err)
		}
		committerKeys = append(committerKeys, committerKey)
	}
	mask, err := bls_cosi.NewMask(committerKeys, nil)
	if err != nil {
		return ctxerror.New("cannot create group sig mask").WithCause(err)
	}
	if err := mask.SetMask(parentHeader.CommitBitmap); err != nil {
		return ctxerror.New("cannot set group sig mask bits").WithCause(err)
	}
	totalAmount := big.NewInt(0)
	numAccounts := 0
	signers := []string{}
	for idx, member := range parentCommittee.NodeList {
		if signed, err := mask.IndexEnabled(idx); err != nil {
			return ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		} else if !signed {
			continue
		}
		numAccounts++
		account := member.EcdsaAddress
		signers = append(signers, account.Hex())
		state.AddBalance(account, BlockReward)
		totalAmount = new(big.Int).Add(totalAmount, BlockReward)
	}
	getLogger().Debug("【Block Reward] Successfully paid out block reward",
		"NumAccounts", numAccounts,
		"TotalAmount", totalAmount,
		"Signers", signers)
	return nil
}

// GenesisStakeInfoFinder is a stake info finder implementation using only
// genesis accounts.
// When used for block reward, it rewards only foundational nodes.
type GenesisStakeInfoFinder struct {
	byNodeKey map[types.BlsPublicKey][]*structs.StakeInfo
	byAccount map[common.Address][]*structs.StakeInfo
}

// FindStakeInfoByNodeKey returns the genesis account matching the given node
// key, as a single-item StakeInfo list.
// It returns nil if the key is not a genesis node key.
func (f *GenesisStakeInfoFinder) FindStakeInfoByNodeKey(
	key *bls.PublicKey,
) []*structs.StakeInfo {
	var pk types.BlsPublicKey
	if err := pk.FromLibBLSPublicKey(key); err != nil {
		ctxerror.Log15(utils.GetLogInstance().Warn, ctxerror.New(
			"cannot convert BLS public key",
		).WithCause(err))
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
		byNodeKey: make(map[types.BlsPublicKey][]*structs.StakeInfo),
		byAccount: make(map[common.Address][]*structs.StakeInfo),
	}
	for idx, account := range genesis.GenesisAccounts {
		blsSecretKeyHex := account.DummyKey
		blsSecretKey := bls.SecretKey{}
		if err := blsSecretKey.DeserializeHexStr(blsSecretKeyHex); err != nil {
			return nil, ctxerror.New("cannot convert BLS secret key",
				"accountIndex", idx,
			).WithCause(err)
		}
		pub := blsSecretKey.GetPublicKey()
		var blsPublicKey types.BlsPublicKey
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
