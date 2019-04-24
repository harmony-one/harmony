package matchers

import (
	"testing"

	"github.com/golang/mock/gomock"
)

type stringable int

func (s stringable) String() string {
	return "omg"
}

func TestStructMatcher_Matches(t *testing.T) {
	type value struct {
		A int
		B string
	}
	tests := []struct {
		name string
		sm   Struct
		x    interface{}
		want bool
	}{
		{
			"EmptyMatchesEmpty",
			Struct{},
			value{},
			true,
		},
		{
			"EmptyMatchesAny",
			Struct{},
			value{A: 3, B: "omg"},
			true,
		},
		{
			"EmptyStillDoesNotMatchNonStruct",
			Struct{},
			0,
			false,
		},
		{
			"RegularFieldValuesUseEq1",
			Struct{"A": 3, "B": "omg"},
			value{A: 3, B: "omg"},
			true,
		},
		{
			"RegularFieldValuesUseEq2",
			Struct{"A": 3, "B": "omg"},
			value{A: 4, B: "omg"},
			false,
		},
		{
			"MatchersAreUsedVerbatim1",
			Struct{"A": gomock.Not(3), "B": gomock.Eq("omg")},
			value{A: 4, B: "omg"},
			true,
		},
		{
			"MatchersAreUsedVerbatim2",
			Struct{"A": gomock.Not(3), "B": gomock.Eq("omg")},
			value{A: 3, B: "omg"},
			false,
		},
		{
			"UnspecifiedFieldsAreIgnored",
			Struct{"A": 3},
			value{A: 3, B: "omg"},
			true,
		},
		{
			"MissingFieldsReturnFailure",
			Struct{"NOTFOUND": 3},
			value{A: 3},
			false,
		},
		{
			"DerefsPointer",
			Struct{"A": 3},
			&value{A: 3},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sm.Matches(tt.x); got != tt.want {
				t.Errorf("Struct.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStructMatcher_String(t *testing.T) {
	tests := []struct {
		name string
		sm   Struct
		want string
	}{
		{"UsesStringer", Struct{"A": stringable(0)}, "<struct A=omg>"},
		{"ReprIfNotStringable", Struct{"A": nil}, "<struct A=<nil>>"},
		{"SortsByKey", Struct{"B": 3, "A": 4}, "<struct A=4 B=3>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sm.String(); got != tt.want {
				t.Errorf("Struct.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
