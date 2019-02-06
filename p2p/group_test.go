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
