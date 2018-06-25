// Consensus package implements the Cosi PBFT consensus
package consensus // consensus

import (
	"fmt"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/common"
	"harmony-benchmark/log"
	"harmony-benchmark/p2p"
	"regexp"
	"strconv"
	"sync"
)

// Consensus data containing all info related to one consensus process
type Consensus struct {
	state ConsensusState
	// Signatures collected from validators
	commits map[string]string
	// Signatures collected from validators
	responses map[string]string
	// Actual block data to reach consensus on
	data string
	// List of validators
	validators []p2p.Peer
	// Leader
	leader p2p.Peer
	// private key of current node
	priKey string
	// Whether I am leader. False means I am validator
	IsLeader bool
	// Leader or validator Id - 2 byte
	nodeId uint16
	// Consensus Id (View Id) - 4 byte
	consensusId uint32
	// Blockhash - 32 byte
	blockHash [32]byte
	// BlockHeader to run consensus on
	blockHeader []byte
	// Shard Id which this node belongs to
	ShardID uint32

	// global consensus mutex
	mutex sync.Mutex

	// Validator specific fields
	// Blocks received but not done with consensus yet
	blocksReceived map[uint32]*BlockConsensusStatus

	// Signal channel for starting a new consensus process
	ReadySignal chan int
	// The verifier func passed from Node object
	BlockVerifier func(*blockchain.Block) bool
	// The post-consensus processing func passed from Node object
	// Called when consensus on a new block is done
	OnConsensusDone func(*blockchain.Block)

	//// Network related fields
	msgCategory byte
	actionType  byte

	Log log.Logger
}

// This used to keep track of the consensus status of multiple blocks received so far
// This is mainly used in the case that this node is lagging behind and needs to catch up.
// For example, the consensus moved to round N and this node received message(N).
// However, this node may still not finished with round N-1, so the newly received message(N)
// should be stored in this temporary structure. In case the round N-1 finishes, it can catch
// up to the latest state of round N by using this structure.
type BlockConsensusStatus struct {
	// BlockHeader to run consensus on
	blockHeader []byte

	state ConsensusState
}

// Consensus state enum for both leader and validator
// States for leader:
//     FINISHED, ANNOUNCE_DONE, CHALLENGE_DONE
// States for validator:
//     FINISHED, COMMIT_DONE, RESPONSE_DONE
type ConsensusState int

const (
	FINISHED ConsensusState = iota // initial state or state after previous consensus is done.
	ANNOUNCE_DONE
	COMMIT_DONE
	CHALLENGE_DONE
	RESPONSE_DONE
)

// Returns string name for the ConsensusState enum
func (state ConsensusState) String() string {
	names := [...]string{
		"FINISHED",
		"ANNOUNCE_DONE",
		"COMMIT_DONE",
		"CHALLENGE_DONE",
		"RESPONSE_DONE"}

	if state < FINISHED || state > RESPONSE_DONE {
		return "Unknown"
	}
	return names[state]
}

// NewConsensus creates a new Consensus object
// TODO(minhdoan): Maybe convert it into just New
// FYI, see https://golang.org/doc/effective_go.html?#package-names
func NewConsensus(ip, port, ShardID string, peers []p2p.Peer, leader p2p.Peer) Consensus {
	// The first Ip, port passed will be leader.
	consensus := Consensus{}
	peer := p2p.Peer{Port: port, Ip: ip}
	Peers := peers
	leaderPeer := leader
	if leaderPeer == peer {
		consensus.IsLeader = true
	} else {
		consensus.IsLeader = false
	}
	consensus.commits = make(map[string]string)
	consensus.responses = make(map[string]string)
	consensus.leader = leaderPeer
	consensus.validators = Peers

	consensus.priKey = ip + ":" + port // use ip:port as unique key for now

	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		consensus.Log.Crit("Regex Compilation Failed", "err", err, "consensus", consensus)
	}
	consensus.consensusId = 0
	myShardID, err := strconv.Atoi(ShardID)
	if err != nil {
		panic("Unparseable shard Id" + ShardID)
	}
	consensus.ShardID = uint32(myShardID)

	// For validators
	consensus.blocksReceived = make(map[uint32]*BlockConsensusStatus)

	// For now use socket address as 16 byte Id
	// TODO: populate with correct Id
	socketId := reg.ReplaceAllString(consensus.priKey, "")
	value, err := strconv.Atoi(socketId)
	consensus.nodeId = uint16(value)

	if consensus.IsLeader {
		consensus.ReadySignal = make(chan int)
		// send a signal to indicate it's ready to run consensus
		go func() {
			consensus.ReadySignal <- 1
		}()
	}

	consensus.msgCategory = byte(common.COMMITTEE)
	consensus.actionType = byte(CONSENSUS)

	consensus.Log = log.New()
	return consensus
}

// Reset the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.state = FINISHED
	consensus.commits = make(map[string]string)
	consensus.responses = make(map[string]string)
}

// Returns ID of this consensus
func (consensus *Consensus) String() string {
	var duty string
	if consensus.IsLeader {
		duty = "LDR" // leader
	} else {
		duty = "VLD" // validator
	}
	return fmt.Sprintf("[%s, %s, %v, %v, %s]", duty, consensus.priKey, consensus.ShardID, consensus.nodeId, consensus.state)
}
