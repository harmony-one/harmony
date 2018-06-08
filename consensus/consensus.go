// Consensus package implements the Cosi PBFT consensus
package consensus // consensus

import (
	"../p2p"
	"fmt"
	"log"
	"sync"
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

var mutex = &sync.Mutex{}

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
func (consensus *Consensus) ProcessMessageLeader(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		log.Print(err)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		log.Print(err)
	}

	msg := string(payload)
	fmt.Printf("[Leader] Received and processing message: %s, %s\n", msgType, msg)
	switch msgType {
	case ANNOUNCE:
		fmt.Println("Unexpected message type: %s", msgType)
	case COMMIT:
		consensus.processCommitMessage(msg)
	case CHALLENGE:
		fmt.Println("Unexpected message type: %s", msgType)
	case RESPONSE:
		consensus.processResponseMessage(msg)
	case START_CONSENSUS:
		consensus.processStartConsensusMessage(msg)
	default:
		fmt.Println("Unexpected message type: %s", msgType)
	}
}

// Handler for message which triggers consensus process
func (consensus *Consensus) processStartConsensusMessage(msg string) {
	consensus.startConsensus(msg)
}

func (consensus *Consensus) startConsensus(msg string) {
	// prepare message and broadcast to validators

	msgToSend := ConstructConsensusMessage(ANNOUNCE, []byte("block"))
	p2p.BroadcastMessage(consensus.Validators, msgToSend)
	// Set state to ANNOUNCE_DONE
	consensus.State = ANNOUNCE_DONE
}

func (consensus *Consensus) processCommitMessage(msg string) {
	// verify and aggregate all the signatures
	mutex.Lock()
	consensus.Signatures = append(consensus.Signatures, msg)

	// Broadcast challenge
	// Set state to CHALLENGE_DONE
	consensus.State = CHALLENGE_DONE
	mutex.Unlock()

	log.Printf("Number of signatures received: %d", len(consensus.Signatures))
	if len(consensus.Signatures) >= (2 * len(consensus.Validators)) / 3 + 1 {
		log.Printf("Consensus reached with %d signatures: %s", len(consensus.Signatures), consensus.Signatures)
	}

}

func (consensus *Consensus) processResponseMessage(msg string) {
	// verify and aggregate all signatures

	// Set state to FINISHED
	consensus.State = FINISHED

}

// Validator's consensus message dispatcher
func (consensus *Consensus) ProcessMessageValidator(message []byte) {
	msgType, err := GetConsensusMessageType(message)
	if err != nil {
		log.Print(err)
	}

	payload, err := GetConsensusMessagePayload(message)
	if err != nil {
		log.Print(err)
	}

	msg := string(payload)
	fmt.Printf("[Validator] Received and processing message: %s, %s\n", msgType, msg)
	switch msgType {
	case ANNOUNCE:
		consensus.processAnnounceMessage(msg)
	case COMMIT:
		fmt.Println("Unexpected message type: %s", msgType)
	case CHALLENGE:
		consensus.processChallengeMessage(msg)
	case RESPONSE:
		fmt.Println("Unexpected message type: %s", msgType)
	default:
		fmt.Println("Unexpected message type: %s", msgType)
	}
}

func (consensus *Consensus) processAnnounceMessage(msg string) {
	// verify block data

	// sign block

	// TODO: return the signature(commit) to leader
	// For now, simply return the private key of this node.
	msgToSend := ConstructConsensusMessage(COMMIT, []byte(consensus.PriKey))
	p2p.SendMessage(consensus.Leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.State = COMMIT_DONE

}

func (consensus *Consensus) processChallengeMessage(msg string) {
	// verify block data and the aggregated signatures

	// sign the message

	// return the signature(response) to leader

	// Set state to RESPONSE_DONE
	consensus.State = RESPONSE_DONE
}
