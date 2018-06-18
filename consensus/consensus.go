// Consensus package implements the Cosi PBFT consensus
package consensus // consensus

import (
	"harmony-benchmark/p2p"
	"regexp"
	"log"
	"strconv"
	"harmony-benchmark/message"
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
	blockHash []byte
	// BlockHeader to run consensus on
	blockHeader []byte
	// Shard Id which this node belongs to
	ShardId uint32

	// Signal channel for starting a new consensus process
	ReadySignal chan int

	//// Network related fields
	msgCategory byte
	actionType byte
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

// Create a new Consensus object
func NewConsensus(ip, port, shardId string, peers []p2p.Peer, leader p2p.Peer) Consensus {
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
		log.Fatal(err)
	}
	consensus.consensusId = 0
	myShardId, err := strconv.Atoi(shardId)
	if err != nil {
		panic("Unparseable shard Id" + shardId)
	}
	consensus.ShardId = uint32(myShardId)

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

	consensus.msgCategory = byte(message.COMMITTEE)
	consensus.actionType = byte(message.CONSENSUS)
	return consensus
}


// Reset the state of the consensus
func (consensus *Consensus) ResetState() {
	consensus.state = READY
	consensus.commits = make(map[string]string)
	consensus.responses = make(map[string]string)
}