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
	tests := []struct {
		name                    string
		groupAction             GroupAction
		expectedGroupActionName string
	}{
		{"BeaconStart", GroupAction{Name: GroupID("ABC"), Action: ActionStart}, "414243/ActionStart"},
		{"BeaconPause", GroupAction{Name: GroupID("ABC"), Action: ActionPause}, "414243/ActionPause"},
		{"BeaconResume", GroupAction{Name: GroupID("ABC"), Action: ActionResume}, "414243/ActionResume"},
		{"BeaconStop", GroupAction{Name: GroupID("ABC"), Action: ActionStop}, "414243/ActionStop"},
		{"BeaconUnknown", GroupAction{Name: GroupID("ABC"), Action: ActionType(8)}, "414243/ActionUnknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.groupAction.String(); got != tt.expectedGroupActionName {
				t.Errorf("ActionType.String() = %v, expected %v", got, tt.expectedGroupActionName)
			}
		})
	}

}
