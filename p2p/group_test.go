package p2p

import "testing"

func TestGroupID_String(t *testing.T) {
	tests := []struct {
		name string
		id   GroupID
		want string
	}{
		{"empty", GroupID(""), ""},
		{"ABC", GroupID("ABC"), "414243"},
		{"binary", GroupID([]byte{1, 2, 3}), "010203"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.String(); got != tt.want {
				t.Errorf("GroupID.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestActionTypeString(t *testing.T) {
	tests := []struct {
		name               string
		actionType         ActionType
		expectedActionName string
	}{
		{"ActionStart", ActionStart, "ActionStart"},
		{"ActionPause", ActionPause, "ActionPause"},
		{"ActionResume", ActionResume, "ActionResume"},
		{"ActionStop", ActionStop, "ActionStop"},
		{"UnknownAction", ActionType(8), "ActionUnknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actionType.String(); got != tt.expectedActionName {
				t.Errorf("ActionType.String() = %v, expected %v", got, tt.expectedActionName)
			}
		})
	}
}

func TestGroupAction(t *testing.T) {

}
