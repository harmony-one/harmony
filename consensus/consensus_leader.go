package consensus

import (
	"log"
	"sync"

	"../p2p"
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
	log.Printf("[Leader] Received and processing message: %s, %s\n", msgType, msg)
	switch msgType {
	case ANNOUNCE:
		log.Println("Unexpected message type: %s", msgType)
	case COMMIT:
		consensus.processCommitMessage(msg)
	case CHALLENGE:
		log.Println("Unexpected message type: %s", msgType)
	case RESPONSE:
		consensus.processResponseMessage(msg)
	case START_CONSENSUS:
		consensus.processStartConsensusMessage(msg)
	default:
		log.Println("Unexpected message type: %s", msgType)
	}
}

// Handler for message which triggers consensus process
func (consensus *Consensus) processStartConsensusMessage(msg string) {
	consensus.startConsensus(msg)
}

func (consensus *Consensus) startConsensus(msg string) {
	// prepare message and broadcast to validators

	msgToSend := ConstructConsensusMessage(ANNOUNCE, []byte("block"))
	p2p.BroadcastMessage(consensus.validators, msgToSend)
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE
}

func (consensus *Consensus) processCommitMessage(msg string) {
	// proceed only when the message is not received before and this consensus phase is not done.
	if _, ok := consensus.commits[msg]; !ok && consensus.state != CHALLENGE_DONE {
		mutex.Lock()
		consensus.commits[msg] = msg
		log.Printf("Number of commits received: %d", len(consensus.commits))
		mutex.Unlock()
	} else {
		return
	}

	if consensus.state != CHALLENGE_DONE && len(consensus.commits) >= (2*len(consensus.validators))/3+1 {
		mutex.Lock()
		if consensus.state == ANNOUNCE_DONE {
			// Set state to CHALLENGE_DONE
			consensus.state = CHALLENGE_DONE
		}
		mutex.Unlock()
		// Broadcast challenge
		msgToSend := ConstructConsensusMessage(CHALLENGE, []byte("challenge"))
		p2p.BroadcastMessage(consensus.validators, msgToSend)

		log.Printf("Enough commits received with %d signatures: %s", len(consensus.commits), consensus.commits)
	}
}

func (consensus *Consensus) processResponseMessage(msg string) {
	// proceed only when the message is not received before and this consensus phase is not done.
	if _, ok := consensus.responses[msg]; !ok && consensus.state != FINISHED {
		mutex.Lock()
		consensus.responses[msg] = msg
		log.Printf("Number of responses received: %d", len(consensus.responses))
		mutex.Unlock()
	} else {
		return
	}

	if consensus.state != FINISHED && len(consensus.responses) >= (2*len(consensus.validators))/3+1 {
		mutex.Lock()
		if consensus.state == CHALLENGE_DONE {
			// Set state to FINISHED
			consensus.state = FINISHED
			log.Println("Hooray! Consensus reached!!!!!!!!!!!!!")
		}
		mutex.Unlock()
		// TODO: composes new block and broadcast the new block to validators

		log.Printf("Consensus reached with %d signatures: %s", len(consensus.responses), consensus.responses)
	}
}
