package matchers

import "testing"

func TestPathMatcher_Matches(t *testing.T) {
	tests := []struct {
		name string
		p    Path
		x    interface{}
		want bool
	}{
		{"EmptyMatchesEmpty", "", "", true},
		{"EmptyDoesNotMatchNonEmpty", "", "a", false},
		{"EmptyDoesNotMatchNonEmptyEvenWithTrailingSlash", "", "a/", false},
		{"NonEmptyDoesNotMatchEmpty", "a", "", false},
		{"ExactIsOK", "abc/def", "abc/def", true},
		{"SuffixIsOK", "abc/def", "omg/abc/def", true},
		{"SubstringIsNotOK", "abc/def", "omg/abc/def/wtf", false},
		{"PrefixIsNotOK", "abc/def", "abc/def/wtf", false},
		{"InterveningElementIsNotOK", "abc/def", "abc/bbq/def", false},
		{"GeneralNonMatch", "abc/def", "omg/wtf", false},
		{"UncleanPattern", "abc//def/", "abc/def", true},
		{"UncleanArg", "abc/def", "abc//def", true},
		{"NonStringArg", "a", 'a', false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Matches(tt.x); got != tt.want {
				t.Errorf("Path.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathMatcher_String(t *testing.T) {
	tests := []struct {
		name string
		p    Path
		want string
	}{
		{"General", "abc/def", "<path suffix \"abc/def\">"},
		{"Unclean", "abc//def/", "<path suffix \"abc/def\">"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.String(); got != tt.want {
				t.Errorf("Path.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
