package consensus

import (
	"log"
	"sync"

	"harmony-benchmark/p2p"
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
	// Set state to ANNOUNCE_DONE
	consensus.state = ANNOUNCE_DONE
	p2p.BroadcastMessage(consensus.validators, msgToSend)
}

func (consensus *Consensus) processCommitMessage(msg string) {
	// proceed only when the message is not received before and this consensus phase is not done.
	mutex.Lock()
	_, ok := consensus.commits[msg]
	shouldProcess := !ok && consensus.state == ANNOUNCE_DONE
	if shouldProcess {
		consensus.commits[msg] = msg
		log.Printf("Number of commits received: %d", len(consensus.commits))
	}
	mutex.Unlock()

	if !shouldProcess {
		return
	}

	mutex.Lock()
	if len(consensus.commits) >= (2*len(consensus.validators))/3+1 {
		log.Printf("Enough commits received with %d signatures: %s", len(consensus.commits), consensus.commits)
		if consensus.state == ANNOUNCE_DONE {
			// Set state to CHALLENGE_DONE
			consensus.state = CHALLENGE_DONE
		}
		// Broadcast challenge
		msgToSend := ConstructConsensusMessage(CHALLENGE, []byte("challenge"))
		p2p.BroadcastMessage(consensus.validators, msgToSend)
	}
	mutex.Unlock()
}

func (consensus *Consensus) processResponseMessage(msg string) {
	// proceed only when the message is not received before and this consensus phase is not done.
	mutex.Lock()
	_, ok := consensus.responses[msg]
	shouldProcess := !ok && consensus.state == CHALLENGE_DONE
	if shouldProcess {
		consensus.responses[msg] = msg
		log.Printf("Number of responses received: %d", len(consensus.responses))
	}
	mutex.Unlock()

	if !shouldProcess {
		return
	}

	mutex.Lock()
	if len(consensus.responses) >= (2*len(consensus.validators))/3+1 {
		log.Printf("Consensus reached with %d signatures: %s", len(consensus.responses), consensus.responses)
		if consensus.state == CHALLENGE_DONE {
			// Set state to FINISHED
			consensus.state = FINISHED
			// TODO: do followups on the consensus
			log.Printf("HOORAY!!! CONSENSUS REACHED AMONG %d NODES!!!\n", len(consensus.validators))
			consensus.ResetState()
		}
		// TODO: composes new block and broadcast the new block to validators
	}
	mutex.Unlock()
}
