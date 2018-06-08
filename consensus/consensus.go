// Consensus package implements the Cosi PBFT consensus
package consensus // consensus

import (
	"../p2p"
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
