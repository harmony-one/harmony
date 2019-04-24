package matchers

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Struct is a struct member matcher.
type Struct map[string]interface{}

// Matches returns whether all specified members match.
func (sm Struct) Matches(x interface{}) bool {
	v := reflect.ValueOf(x)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return false
	}
	for n, m := range sm {
		m1 := toMatcher(m)
		f := v.FieldByName(n)
		if f == (reflect.Value{}) {
			return false
		}
		f1 := f.Interface()
		if !m1.Matches(f1) {
			return false
		}
	}
	return true
}

func (sm Struct) String() string {
	var fields sort.StringSlice
	for name := range sm {
		fields = append(fields, name)
	}
	sort.Sort(fields)
	for i, name := range fields {
		value := sm[name]
		var vs string
		if _, ok := value.(fmt.Stringer); ok {
			vs = fmt.Sprintf("%s", value)
		} else {
			vs = fmt.Sprintf("%v", value)
		}
		fields[i] = fmt.Sprintf("%s=%s", name, vs)
	}
	return "<struct " + strings.Join(fields, " ") + ">"
}
