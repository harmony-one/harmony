package consensus

import (
	"harmony-benchmark/p2p"
	"log"
)

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
	log.Printf("[Validator] Received and processing message: %s, %s\n", msgType, msg)
	switch msgType {
	case ANNOUNCE:
		consensus.processAnnounceMessage(msg)
	case COMMIT:
		log.Println("Unexpected message type: %s", msgType)
	case CHALLENGE:
		consensus.processChallengeMessage(msg)
	case RESPONSE:
		log.Println("Unexpected message type: %s", msgType)
	default:
		log.Println("Unexpected message type: %s", msgType)
	}
}

func (consensus *Consensus) processAnnounceMessage(msg string) {
	// verify block data

	// sign block

	// TODO: return the signature(commit) to leader
	// For now, simply return the private key of this node.
	msgToSend := ConstructConsensusMessage(COMMIT, []byte(consensus.priKey))
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to COMMIT_DONE
	consensus.state = COMMIT_DONE

}

func (consensus *Consensus) processChallengeMessage(msg string) {
	// verify block data and the aggregated signatures

	// sign the message

	// TODO: return the signature(response) to leader
	// For now, simply return the private key of this node.
	msgToSend := ConstructConsensusMessage(RESPONSE, []byte(consensus.priKey))
	p2p.SendMessage(consensus.leader, msgToSend)

	// Set state to RESPONSE_DONE
	consensus.state = RESPONSE_DONE
}
