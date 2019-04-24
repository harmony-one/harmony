package matchers

import (
	"testing"

	"github.com/golang/mock/gomock"
)

func TestSliceMatcher_Matches(t *testing.T) {
	tests := []struct {
		name string
		sm   Slice
		x    interface{}
		want bool
	}{
		{
			"EmptyEqEmpty",
			Slice{},
			[]interface{}{},
			true,
		},
		{
			"EmptyNeNotEmpty",
			Slice{},
			[]interface{}{1},
			false,
		},
		{
			"NotEmptyNeEmpty",
			Slice{0},
			[]interface{}{},
			false,
		},
		{
			"CompareRawValuesUsingEqualityHappy",
			Slice{1, 2, 3},
			[]interface{}{1, 2, 3},
			true,
		},
		{
			"CompareRawValuesUsingEqualityUnhappy",
			Slice{1, 2, 3},
			[]interface{}{1, 20, 3},
			false,
		},
		{
			"CompareMatcherUsingItsMatchesHappy",
			Slice{gomock.Nil(), gomock.Eq(3)},
			[]interface{}{nil, 3},
			true,
		},
		{
			"CompareMatcherUsingItsMatchesUnhappy",
			Slice{gomock.Nil(), gomock.Eq(3)},
			[]interface{}{0, 3},
			false,
		},
		{
			"NestedHappy",
			Slice{Slice{3}, 30},
			[]interface{}{[]interface{}{3}, 30},
			true,
		},
		{
			"NestedUnhappy",
			Slice{Slice{3}, 30},
			[]interface{}{[]interface{}{300}, 30},
			false,
		},
		{
			"MatchSliceOfMoreSpecificTypes",
			Slice{1, 2, 3},
			[]int{1, 2, 3},
			true,
		},
		{
			"AcceptArraysToo",
			Slice{1, 2, 3},
			[...]int{1, 2, 3},
			true,
		},
		{
			"RejectString",
			Slice{'a', 'b', 'c'},
			"abc",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sm.Matches(tt.x); got != tt.want {
				t.Errorf("Slice.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSliceMatcher_String(t *testing.T) {
	tests := []struct {
		name string
		sm   Slice
		want string
	}{
		{"int", []interface{}{3, 5, 7}, "[3 5 7]"},
		{"string", []interface{}{"omg", "wtf", "bbq"}, "[omg wtf bbq]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sm.String(); got != tt.want {
				t.Errorf("Slice.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
