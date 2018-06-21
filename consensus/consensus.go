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
)

// Consensus data containing all info related to one consensus process
type Consensus struct {
	state           ConsensusState
	commits         map[string]string            // Signatures collected from validators
	responses       map[string]string            // Signatures collected from validators
	data            string                       // Actual block data to reach consensus on
	validators      []p2p.Peer                   // List of validators
	leader          p2p.Peer                     // Leader
	priKey          string                       // private key of current node
	IsLeader        bool                         // Whether I am leader. False means I am validator
	nodeId          uint16                       // Leader or validator Id - 2 byte
	consensusId     uint32                       // Consensus Id (View Id) - 4 byte
	blockHash       [32]byte                     // Blockhash - 32 byte
	blockHeader     []byte                       // BlockHeader to run consensus on
	ShardIDShardID  uint32                       // Shard Id which this node belongs to
	ReadySignal     chan int                     // Signal channel for starting a new consensus process
	BlockVerifier   func(*blockchain.Block) bool // The verifier func passed from Node object
	OnConsensusDone func(*blockchain.Block)      // The post-consensus processing func passed from Node object. Called when consensus on a new block is done
	msgCategory     byte                         // Network related fields
	actionType      byte
	Log             log.Logger
}

// Consensus state enum for both leader and validator
// States for leader:
//     READY, ANNOUNCE_DONE, CHALLENGE_DONE, FINISHED
// States for validator:
//     READY, COMMIT_DONE, RESPONSE_DONE, FINISHED
type ConsensusState int

const (
	READY ConsensusState = iota
	ANNOUNCE_DONE
	COMMIT_DONE
	CHALLENGE_DONE
	RESPONSE_DONE
	FINISHED
)

// Returns string name for the ConsensusState enum
func (state ConsensusState) String() string {
	names := [...]string{
		"READY",
		"ANNOUNCE_DONE",
		"COMMIT_DONE",
		"CHALLENGE_DONE",
		"RESPONSE_DONE",
		"FINISHED"}

	if state < READY || state > RESPONSE_DONE {
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
	myShardIDShardID, err := strconv.Atoi(ShardID)
	if err != nil {
		panic("Unparseable shard Id" + ShardID)
	}
	consensus.ShardIDShardID = uint32(myShardIDShardID)

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
	consensus.actionType = byte(common.CONSENSUS)

	consensus.Log = log.New()
	return consensus
}

// Reset the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.state = READY
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
	return fmt.Sprintf("[%s, %s, %v, %v]", duty, consensus.priKey, consensus.ShardIDShardID, consensus.nodeId)
}
