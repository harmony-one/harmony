package consensus

import "testing"

func TestConsensusState(t *testing.T) {
	tests := []struct {
		name      string
		state     State
		stateName string
	}{
		{"Finished", Finished, "Finished"},
		{"AnnounceDone", AnnounceDone, "AnnounceDone"},
		{"PrepareDone", PrepareDone, "PrepareDone"},
		{"PreparedDone", PreparedDone, "PreparedDone"},
		{"CommitDone", CommitDone, "CommitDone"},
		{"CommittedDone", CommittedDone, "CommittedDone"},
		{"Unknown", State(10), "Unknown"},
		{"Unknown", State(-1), "Unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.String(); got != tt.stateName {
				t.Errorf("ActionType.String() = %v, expected %v", got, tt.stateName)
			}
		})
	}
}
