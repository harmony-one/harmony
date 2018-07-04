package consensus

// Consensus state enum for both leader and validator
// States for leader:
//     FINISHED, ANNOUNCE_DONE, CHALLENGE_DONE
// States for validator:
//     FINISHED, COMMIT_DONE, RESPONSE_DONE
type ConsensusState int

const (
	FINISHED ConsensusState = iota // initial state or state after previous consensus is done.
	ANNOUNCE_DONE
	COMMIT_DONE
	CHALLENGE_DONE
	RESPONSE_DONE
)

// Returns string name for the ConsensusState enum
func (state ConsensusState) String() string {
	names := [...]string{
		"FINISHED",
		"ANNOUNCE_DONE",
		"COMMIT_DONE",
		"CHALLENGE_DONE",
		"RESPONSE_DONE"}

	if state < FINISHED || state > RESPONSE_DONE {
		return "Unknown"
	}
	return names[state]
}
