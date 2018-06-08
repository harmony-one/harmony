package consensus

import (
	"log"
	"fmt"
	"../p2p"
	"sync"
)

var mutex = &sync.Mutex{}

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
	if _, ok := consensus.Signatures[msg]; !ok {
		mutex.Lock()
		consensus.Signatures[msg] = msg
		mutex.Unlock()
	}


	log.Printf("Number of signatures received: %d", len(consensus.Signatures))
	if consensus.State != CHALLENGE_DONE && len(consensus.Signatures) >= (2 * len(consensus.Validators)) / 3 + 1 {

		mutex.Lock()
		if consensus.State == ANNOUNCE_DONE {
			// Set state to CHALLENGE_DONE
			consensus.State = CHALLENGE_DONE
		}
		mutex.Unlock()
		// Broadcast challenge
		msgToSend := ConstructConsensusMessage(CHALLENGE, []byte("challenge"))
		p2p.BroadcastMessage(consensus.Validators, msgToSend)

		log.Printf("Consensus reached with %d signatures: %s", len(consensus.Signatures), consensus.Signatures)
	}

}

func (consensus *Consensus) processResponseMessage(msg string) {
	// verify and aggregate all signatures

	// Set state to FINISHED
	consensus.State = FINISHED

}