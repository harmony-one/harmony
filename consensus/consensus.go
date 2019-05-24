// Package consensus implements the Cosi PBFT consensus
package consensus // consensus

import (
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/bls/ffi/go/bls"

	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/internal/utils/contract"
	"github.com/harmony-one/harmony/p2p"
)

const (
	// ConsensusVersion : v1 old version without viewchange, v2 new version with viewchange
	ConsensusVersion = "v2"
)

// Block reward per block signature.
// TODO ek – per sig per stake
var (
	BlockReward = big.NewInt(params.Ether / 10)
)

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	ConsensusVersion string
	// pbftLog stores the pbft messages and blocks during PBFT process
	pbftLog *PbftLog
	// phase: different phase of PBFT protocol: pre-prepare, prepare, commit, finish etc
	phase PbftPhase
	// mode: indicate a node is in normal or viewchanging mode
	mode PbftMode
	// seqNum: the next blockNumber that PBFT is going to agree on, should be equal to the blockNumber of next block
	seqNum uint64
	// channel to receive consensus message
	MsgChan chan []byte
	// timer to make sure leader publishes block in a timely manner; if not
	// then this node will propose view change
	idleTimeout utils.Timeout

	// timer to make sure this node commit a block in a timely manner; if not
	// this node will initialize a view change
	commitTimeout utils.Timeout

	// When doing view change, timer to make sure a valid view change message
	// sent by new leader in a timely manner; if not this node will start
	// a new view change
	viewChangeTimeout utils.Timeout

	// The duration of viewChangeTimeout; when a view change is initialized with v+1
	// timeout will be equal to viewChangeDuration; if view change failed and start v+2
	// timeout will be 2*viewChangeDuration; timeout of view change v+n is n*viewChangeDuration
	viewChangeDuration time.Duration

	//TODO depreciate it after implement PbftPhase
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
	// the publickey of leader
	LeaderPubKey *bls.PublicKey

	// Leader or validator address in hex
	SelfAddress common.Address
	// Consensus Id (View Id) - 4 byte
	consensusID uint32 // TODO(chao): change it to uint64 or add overflow checking mechanism

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
	BlockVerifier func(*types.Block) error

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

	// Staking information finder
	stakeInfoFinder StakeInfoFinder
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
	consensus.ConsensusVersion = ConsensusVersion

	consensus.pbftLog = NewPbftLog()
	consensus.phase = Announce
	consensus.mode = PbftMode{mode: Normal}

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		nodeconfig.GetDefaultConfig().SetIsLeader(true)
	} else {
		nodeconfig.GetDefaultConfig().SetIsLeader(false)
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

	consensus.MsgChan = make(chan []byte)

	// For validators to keep track of all blocks received but not yet committed, so as to catch up to latest consensus if lagged behind.
	consensus.blocksReceived = make(map[uint32]*BlockConsensusStatus)

	if nodeconfig.GetDefaultConfig().IsLeader() {
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
	shardState, err := bc.ReadShardState(parentHeader.Epoch)
	if err != nil {
		return ctxerror.New("cannot read shard state",
			"epoch", parentHeader.Epoch,
		).WithCause(err)
	}
	parentShardState := shardState.FindCommitteeByID(parentHeader.ShardID)
	if parentShardState == nil {
		return ctxerror.New("cannot find shard in the shard state",
			"parentBlockNumber", parentHeader.Number,
			"shardID", parentHeader.ShardID,
		)
	}
	var committerKeys []*bls.PublicKey
	for _, member := range parentShardState.NodeList {
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
	for idx, member := range parentShardState.NodeList {
		if signed, err := mask.IndexEnabled(idx); err != nil {
			return ctxerror.New("cannot check for committer bit",
				"committerIndex", idx,
			).WithCause(err)
		} else if !signed {
			continue
		}
		numAccounts++
		account := common.HexToAddress(member.EcdsaAddress)
		getLogger().Info("rewarding block signer",
			"account", account,
			"node", member.BlsPublicKey.Hex(),
			"amount", BlockReward)
		state.AddBalance(account, BlockReward)
		totalAmount = new(big.Int).Add(totalAmount, BlockReward)
	}
	getLogger().Debug("paid out block reward",
		"numAccounts", numAccounts,
		"totalAmount", totalAmount)
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
	for idx, account := range contract.GenesisAccounts {
		blsSecretKeyHex := contract.GenesisBLSAccounts[idx].Private
		blsSecretKey := bls.SecretKey{}
		if err := blsSecretKey.SetHexString(blsSecretKeyHex); err != nil {
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
