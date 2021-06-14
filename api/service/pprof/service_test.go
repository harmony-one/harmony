package pprof

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestUnpackProfilesIntoMap(t *testing.T) {
	tests := []struct {
		input  *Config
		expMap map[string]Profile
		expErr error
	}{
		{
			input:  &Config{},
			expMap: make(map[string]Profile),
		},
		{
			input: &Config{
				ProfileNames: []string{"test", "test"},
			},
			expMap: nil,
			expErr: errors.New("Pprof profile names contains duplicate: test"),
		},
		{
			input: &Config{
				ProfileNames: []string{"test"},
			},
			expMap: map[string]Profile{
				"test": {
					Name: "test",
				},
			},
		},
		{
			input: &Config{
				ProfileNames:       []string{"test1", "test2"},
				ProfileIntervals:   []int{0, 60},
				ProfileDebugValues: []int{1},
			},
			expMap: map[string]Profile{
				"test1": {
					Name:     "test1",
					Interval: 0,
					Debug:    1,
				},
				"test2": {
					Name:     "test2",
					Interval: 60,
					Debug:    1,
				},
			},
		},
	}
	for i, test := range tests {
		actual, err := test.input.unpackProfilesIntoMap()
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, assErr)
		}
		if !reflect.DeepEqual(actual, test.expMap) {
			t.Errorf("Test %v: unexpected map\n\t%+v\n\t%+v", i, actual, test.expMap)
		}
	}
}

func assertError(gotErr, expErr error) error {
	if (gotErr == nil) != (expErr == nil) {
		return fmt.Errorf("error unexpected [%v] / [%v]", gotErr, expErr)
	}
	if gotErr == nil {
		return nil
	}
	if !strings.Contains(gotErr.Error(), expErr.Error()) {
		return fmt.Errorf("error unexpected [%v] / [%v]", gotErr, expErr)
	}
	return nil
}
