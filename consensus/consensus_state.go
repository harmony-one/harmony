package consensus

// State is the consensus state enum for both leader and validator
type State int

// Followings are the set of states of validators or leaders during consensus.
const (
	Finished State = iota
	AnnounceDone
	CommitDone
	ChallengeDone
	ResponseDone
	CollectiveSigDone
	FinalCommitDone
	FinalChallengeDone
	FinalResponseDone
)

// Returns string name for the State enum
func (state State) String() string {
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
