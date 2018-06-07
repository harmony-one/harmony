// Consensus package implements the Cosi PBFT consensus
package consensus // consensus

import (
	"../p2p"
	"fmt"
)

// Consensus data containing all info related to one consensus process
type Consensus struct {
	State ConsensusState
	// Signatures collected from validators
	Signatures []string
	// Actual block data to reach consensus on
	Data string
	// List of validators
	Validators []p2p.Peer
	// Leader
	Leader p2p.Peer
	// private key of current node
	PriKey string
	// Whether I am leader. False means I am validator
	IsLeader bool
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

// Consensus communication message type.
// Leader and validator dispatch messages based on incoming message type
type MessageType int

const (
	ANNOUNCE MessageType = iota
	COMMIT
	CHALLENGE
	RESPONSE
	START_CONSENSUS
)

// Returns string name for the MessageType enum
func (msgType MessageType) String() string {
	names := [...]string{
		"ANNOUNCE",
		"COMMIT",
		"CHALLENGE",
		"RESPONSE",
		"START_CONSENSUS"}

	if msgType < ANNOUNCE || msgType > START_CONSENSUS {
		return "Unknown"
	}
	return names[msgType]
}

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

// Leader's consensus message dispatcher
func (consensus Consensus) ProcessMessageLeader(msgType MessageType, msg string) {
	switch msgType {
	case ANNOUNCE:
		fmt.Println("Unexpected message type: %s", msgType)
	case COMMIT:
		fmt.Println("Received and processing message with type: %s", msgType)
		consensus.processCommitMessage(msg)
	case CHALLENGE:
		fmt.Println("Unexpected message type: %s", msgType)
	case RESPONSE:
		fmt.Println("Received and processing message with type: %s", msgType)
		consensus.processResponseMessage(msg)
	case START_CONSENSUS:
		fmt.Printf("Received and processing message with type: %s\n", msgType)
		consensus.processStartConsensusMessage(msg)
	default:
		fmt.Println("Unexpected message type: %s", msgType)
	}
}

// Handler for message which triggers consensus process
func (consensus Consensus) processStartConsensusMessage(msg string) {
	consensus.startConsensus(msg)
}

func (consensus Consensus) startConsensus(msg string) {
	// prepare message and broadcast to validators

	p2p.BroadcastMessage(consensus.Validators, "hello")
	// Set state to ANNOUNCE_DONE
	consensus.State = ANNOUNCE_DONE
}

func (consensus Consensus) processCommitMessage(msg string) {
	// verify and aggregate all the signatures

	// Broadcast challenge
	// Set state to CHALLENGE_DONE
	consensus.State = CHALLENGE_DONE
}

func (consensus Consensus) processResponseMessage(msg string) {
	// verify and aggregate all signatures

	// Set state to FINISHED
	consensus.State = FINISHED

}

// Validator's consensus message dispatcher
func (consensus Consensus) ProcessMessageValidator(msgType MessageType, msg string) {
	switch msgType {
	case ANNOUNCE:
		fmt.Println("Received and processing message with type: %s", msgType)
		consensus.processAnnounceMessage(msg)
	case COMMIT:
		fmt.Println("Unexpected message type: %s", msgType)
	case CHALLENGE:
		fmt.Println("Received and processing message with type: %s", msgType)
		consensus.processChallengeMessage(msg)
	case RESPONSE:
		fmt.Println("Received and processing message with type: %s", msgType)
		consensus.processResponseMessage(msg)
	default:
		fmt.Println("Unexpected message type: %s", msgType)
	}
}

func (consensus Consensus) processAnnounceMessage(msg string) {
	// verify block data

	// sign block

	// return the signature(commit) to leader

	// Set state to COMMIT_DONE
	consensus.State = COMMIT_DONE
	peer := consensus.Leader
	fmt.Println("my port is:", peer.Port)
	fmt.Println(peer.Port)
	p2p.SendMessage(consensus.Leader, "hi")

}

func (consensus Consensus) processChallengeMessage(msg string) {
	// verify block data and the aggregated signatures

	// sign the message

	// return the signature(response) to leader

	// Set state to RESPONSE_DONE
	consensus.State = RESPONSE_DONE
}
