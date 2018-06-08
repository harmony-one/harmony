package consensus

import (
	"log"
	"fmt"
	"../p2p"
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

