package consensus

// Consensus state enum for both leader and validator
// States for leader:
//     FINISHED, AnnounceDone, ChallengeDone
// States for validator:
//     FINISHED, CommitDone, ResponseDone
type ConsensusState int

const (
	FINISHED ConsensusState = iota // initial state or state after previous consensus is done.
	AnnounceDone
	CommitDone
	ChallengeDone
	ResponseDone
	CollectiveSigDone
	FinalCommitDone
	FinalChallengeDone
	FinalResponseDone
)

// Returns string name for the ConsensusState enum
func (state ConsensusState) String() string {
	names := [...]string{
		"FINISHED",
		"AnnounceDone",
		"CommitDone",
		"ChallengeDone",
		"ResponseDone",
		"CollectiveSigDone",
		"FinalCommitDone",
		"FinalChallengeDone",
		"FinalResponseDone"}

	if state < FINISHED || state > FinalResponseDone {
		return "Unknown"
	}
	return names[state]
}
