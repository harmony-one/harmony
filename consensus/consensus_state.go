package consensus

// State is the consensus state enum for both leader and validator
type State int

// Followings are the set of states of validators or leaders during consensus.
const (
	Finished State = iota
	AnnounceDone
	PrepareDone
	PreparedDone
	CommitDone
	CommittedDone
)

// Returns string name for the State enum
func (state State) String() string {
	names := [...]string{
		"Finished",
		"AnnounceDone",
		"PrepareDone",
		"PreparedDone",
		"CommitDone",
		"CommittedDone"}

	if state < Finished || state > CommittedDone {
		return "Unknown"
	}
	return names[state]
}
