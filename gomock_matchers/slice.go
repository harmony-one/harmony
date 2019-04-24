package matchers

import (
	"fmt"
	"reflect"
)

// Slice is a gomock matcher that matches elements of an array or
// slice against its own members at the corresponding positions.
// Each member item in a Slice may be a regular item or a gomock
// Matcher instance.
type Slice []interface{}

// Matches returns whether x is a slice with matching elements.
func (sm Slice) Matches(x interface{}) bool {
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Slice, reflect.Array: // OK
	default:
		return false
	}
	l := v.Len()
	if l != len(sm) {
		return false
	}
	for i, m := range sm {
		m1 := toMatcher(m)
		v1 := v.Index(i).Interface()
		if !m1.Matches(v1) {
			return false
		}
	}
	return true
}

// String returns the string representation of this slice matcher.
func (sm Slice) String() string {
	return fmt.Sprint([]interface{}(sm))
}
