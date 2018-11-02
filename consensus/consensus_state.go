package consensus

// ConsensusState is the consensus state enum for both leader and validator
// States for leader:
//     Finished, AnnounceDone, ChallengeDone
// States for validator:
//     Finished, CommitDone, ResponseDone
type ConsensusState int

// Followings are the set of states of validators or leaders during consensus.
const (
	Finished ConsensusState = iota
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
