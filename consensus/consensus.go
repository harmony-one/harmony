package consensus

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/consensus/quorum"
	"github.com/harmony-one/harmony/consensus/timeouts"
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

var errLeaderPriKeyNotFound = errors.New(
	"getting leader private key from consensus public keys failed",
)

type blkComeback struct {
	Blk *types.Block
	Err chan error
}

type processBlock struct {
	Request chan blkComeback
}

// BlockHash ..
func (consensus *Consensus) BlockHash() common.Hash {
	return consensus.blockHash.Load().(common.Hash)
}

// SetBlockHash ..
func (consensus *Consensus) SetBlockHash(h common.Hash) {
	consensus.blockHash.Store(h)
}

// BlockNum ..
func (consensus *Consensus) BlockNum() uint64 {
	return consensus.blockNum.Load().(uint64)
}

// SetBlockNum ..
func (consensus *Consensus) SetBlockNum(num uint64) {
	consensus.blockNum.Store(num)
}

// Block ..
func (consensus *Consensus) Block() []byte {
	return consensus.block.Load().([]byte)
}

// SetBlock ..
func (consensus *Consensus) SetBlock(b []byte) {
	consensus.block.Store(b)
}

// BlockHeader ..
func (consensus *Consensus) BlockHeader() []byte {
	return consensus.blockHeader.Load().([]byte)
}

// SetBlockHeader ..
func (consensus *Consensus) SetBlockHeader(b []byte) {
	consensus.blockHeader.Store(b)
}

// Epoch ..
func (consensus *Consensus) Epoch() uint64 {
	return consensus.epoch.Load().(uint64)
}

// SetEpoch ..
func (consensus *Consensus) SetEpoch(e uint64) {
	consensus.epoch.Store(e)
}

// LeaderPubKey ..
func (consensus *Consensus) LeaderPubKey() *bls.PublicKey {
	return consensus.leaderPubKey.Load().(*bls.PublicKey)
}

// SetLeaderPubKey ..
func (consensus *Consensus) SetLeaderPubKey(k *bls.PublicKey) {
	consensus.leaderPubKey.Store(k)
}

// ViewID ..
func (consensus *Consensus) ViewID() uint64 {
	return consensus.viewID.Load().(uint64)
}

// Finished ..
type Finished struct {
	ViewID    uint64
	ShardID   uint32
	BlockNum  uint64
	BlockHash common.Hash
}

type Range struct {
	Start uint64
	End   uint64
}

// Consensus is the main struct with all states and data related to consensus process.
type Consensus struct {
	Decider quorum.Decider
	// FBFTLog stores the pbft messages and blocks during FBFT process
	FBFTLog *FBFTLog
	// phase: different phase of FBFT protocol: pre-prepare, prepare, commit, finish etc
	phase atomic.Value
	// current indicates what state a node is in
	Current State
	// epoch: current epoch number
	epoch atomic.Value
	// blockNum: the next blockNumber that FBFT is going to agree on,
	// should be equal to the blockNumber of next block
	// blockNum, viewID are both uint64
	blockNum atomic.Value
	viewID   atomic.Value
	// How long to delay sending commit messages.
	delayCommit time.Duration
	// Consensus rounds whose commit phase finished
	CommitFinishChan chan Finished
	// Commits collected from validators.
	aggregatedPrepareSig *bls.Sign
	aggregatedCommitSig  *bls.Sign
	// TODO Make these two be atomic.Value as well
	prepareBitmap *bls_cosi.Mask
	commitBitmap  *bls_cosi.Mask
	// Commits collected from view change
	// for each viewID, we need keep track of corresponding sigs and bitmap
	// until one of the viewID has enough votes (>=2f+1)
	// after one of viewID has enough votes, we can reset and clean the map
	// honest nodes will never double votes on different viewID
	// bhpSigs: blockHashPreparedSigs is the signature on m1 type message
	bhpSigs map[uint64]map[string]*bls.Sign
	// nilSigs: there is no prepared message when view change,
	// it's signature on m2 type (i.e. nil) messages
	nilSigs      map[uint64]map[string]*bls.Sign
	viewIDSigs   map[uint64]map[string]*bls.Sign
	bhpBitmap    map[uint64]*bls_cosi.Mask
	nilBitmap    map[uint64]*bls_cosi.Mask
	viewIDBitmap map[uint64]*bls_cosi.Mask
	// message payload for
	// type m1 := |vcBlockHash|prepared_agg_sigs|prepared_bitmap|, new leader only need one
	m1Payload []byte
	// The chain reader for the blockchain this consensus is working on
	ChainReader *core.BlockChain
	// Minimal number of peers in the shard
	// If the number of validators is less than minPeers, the consensus won't start
	MinPeers int
	// private/public keys of current node
	priKey *multibls.PrivateKey
	PubKey *multibls.PublicKey
	// the publickey of leader
	leaderPubKey atomic.Value
	// Blockhash - 32 byte
	blockHash atomic.Value
	// Block to run consensus on
	block atomic.Value
	// BlockHeader to run consensus on
	blockHeader atomic.Value
	// Shard Id which this node belongs to
	ShardID uint32
	// whether to ignore viewID check
	ignoreViewIDCheck bool
	Locks             struct {
		VC     sync.Mutex // mutex for view change
		Global sync.Mutex
		Leader sync.Mutex
		PubKey sync.Mutex
	}
	// Signal channel for starting a new consensus process
	ProposalNewBlock chan struct{}
	RoundCompleted   processBlock
	Verify           processBlock
	// Channel for DRG protocol to send pRnd (preimage of randomness resulting from combined vrf
	// randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	PRndChannel chan []byte
	// Channel for DRG protocol to send VDF. The first 516 bytes are the VDF/Proof and the last 32
	// bytes are the seed for deriving VDF
	RndChannel  chan [vdfAndSeedSize]byte
	pendingRnds [][vdfAndSeedSize]byte // A list of pending randomness
	// The p2p host used to send/receive p2p messages
	host     *p2p.Host
	Timeouts *timeouts.Notifier
	// If true, this consensus will not propose view change.
	disableViewChange bool
	// Have a dedicated reader thread pull from this chan, like in node
	SlashChan chan slash.Record
	// The time due for next block proposal
	nextBlockDue             atomic.Value
	IncomingConsensusMessage chan []byte
	SyncNeeded               chan Range
}

// VdfSeedSize returns the number of VRFs for VDF computation
func (consensus *Consensus) VdfSeedSize() int {
	return int(consensus.Decider.ParticipantsCount()) * 2 / 3
}

// GetLeaderPrivateKey returns leader private key if node is the leader
func (consensus *Consensus) GetLeaderPrivateKey(
	leaderKey *bls.PublicKey,
) (*bls.SecretKey, error) {
	for i := range consensus.PubKey.PublicKey {
		if consensus.PubKey.PublicKey[i].IsEqual(leaderKey) {
			return consensus.priKey.PrivateKey[i], nil
		}
	}

	return nil, errors.Wrapf(errLeaderPriKeyNotFound, leaderKey.SerializeToHexStr())
}

// GetConsensusLeaderPrivateKey returns consensus leader private key if node is the leader
func (consensus *Consensus) GetConsensusLeaderPrivateKey() (*bls.SecretKey, error) {
	if consensus.IsLeader() {
		return consensus.GetLeaderPrivateKey(consensus.LeaderPubKey())
	}
	return nil, errors.New("this node is not leader")
}

// New ..
func New(
	host *p2p.Host,
	shard uint32,
	multiBLSPriKey *multibls.PrivateKey,
	Decider quorum.Decider,
	commitDelay time.Duration,
	minPeer int,
) (*Consensus, error) {

	var phase, blk, view, epoch, leader atomic.Value

	phase.Store(FBFTAnnounce)
	blk.Store(uint64(0))
	view.Store(uint64(0))
	epoch.Store(uint64(0))
	leader.Store(&bls.PublicKey{})

	consensus := Consensus{
		Decider:          Decider,
		FBFTLog:          NewFBFTLog(),
		phase:            phase,
		Current:          NewState(),
		epoch:            epoch,
		blockNum:         blk,
		viewID:           view,
		CommitFinishChan: make(chan Finished),
		host:             host,
		leaderPubKey:     leader,
		Timeouts:         timeouts.NewNotifier(),
		SlashChan:        make(chan slash.Record),
		ProposalNewBlock: make(chan struct{}),
		RoundCompleted:   processBlock{make(chan blkComeback)},
		Verify:           processBlock{make(chan blkComeback)},
		// channel for receiving newly generated VDF
		RndChannel:               make(chan [vdfAndSeedSize]byte),
		ShardID:                  shard,
		IncomingConsensusMessage: make(chan []byte),
		SyncNeeded:               make(chan Range, 1),
		delayCommit:              commitDelay,
		MinPeers:                 minPeer,
	}

	if multiBLSPriKey != nil {
		consensus.priKey = multiBLSPriKey
		consensus.PubKey = multiBLSPriKey.GetPublicKey()
	} else {
		utils.Logger().Error().Msg("the bls key is nil")
		return nil, fmt.Errorf("nil bls key, aborting")
	}

	return &consensus, nil
}
