package matchers

import (
	"fmt"
	"path"
	"strings"
)

// Path is a pathname matcher.
//
// A value matches if it is the same as the matcher pattern or has the matcher
// pattern as a trailing component.  For example,
// a pattern "abc/def" matches "abc/def" itself, "omg/abc/def",
// but not "abc/def/wtf", "abc/omg/def", or "xabc/def".
//
// Both the pattern and the value are sanitized using path.Clean() before use.
//
// The empty pattern "" matches only the empty value "".
type Path string

// Matches returns whether x is the matching pathname itself or ends with the
// matching pathname, inside another directory.
func (p Path) Matches(x interface{}) bool {
	if s, ok := x.(string); ok {
		p1 := path.Clean(string(p))
		s = path.Clean(s)
		return s == p1 || strings.HasSuffix(s, "/"+p1)
	}
	return false
}

// String returns the string representation of this pathname matcher.
func (p Path) String() string {
	return fmt.Sprintf("<path suffix %#v>", path.Clean(string(p)))
}
