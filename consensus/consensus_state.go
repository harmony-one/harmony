package consensus

// Consensus state enum for both leader and validator
// States for leader:
//     Finished, AnnounceDone, ChallengeDone
// States for validator:
//     Finished, CommitDone, ResponseDone
type ConsensusState int

const (
	Finished ConsensusState = iota // initial state or state after previous consensus is done.
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
		"Finished",
		"AnnounceDone",
		"CommitDone",
		"ChallengeDone",
		"ResponseDone",
		"CollectiveSigDone",
		"FinalCommitDone",
		"FinalChallengeDone",
		"FinalResponseDone"}

	if state < Finished || state > FinalResponseDone {
		return "Unknown"
	}
	return names[state]
}
