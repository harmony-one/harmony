package pprof

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

func TestUnpackProfilesIntoMap(t *testing.T) {
	tests := []struct {
		input  *Config
		expMap map[string]Profile
		expErr error
	}{
		{
			input:  &Config{},
			expMap: nil,
		},
		{
			input: &Config{
				ProfileNames: []string{"cpu", "cpu"},
			},
			expMap: map[string]Profile{
				"cpu": {
					Name:       "cpu",
					Interval:   0,
					Debug:      0,
					ProfileRef: pprof.Lookup("cpu"),
				},
			},
		},
		{
			input: &Config{
				ProfileNames: []string{"test"},
			},
			expMap: nil,
			expErr: errors.New("pprof profile does not exist: test"),
		},
		{
			input: &Config{
				ProfileNames:       []string{"cpu", "heap"},
				ProfileIntervals:   []int{0, 60},
				ProfileDebugValues: []int{1},
			},
			expMap: map[string]Profile{
				"cpu": {
					Name:       "cpu",
					Interval:   0,
					Debug:      1,
					ProfileRef: pprof.Lookup("cpu"),
				},
				"heap": {
					Name:       "heap",
					Interval:   60,
					Debug:      1,
					ProfileRef: pprof.Lookup("heap"),
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

func TestStart(t *testing.T) {
	input := &Config{
		Enabled:          true,
		Folder:           tempTestDir(),
		ProfileNames:     []string{"cpu"},
		ProfileIntervals: []int{1},
	}
	defer os.RemoveAll(input.Folder)
	s := NewService(*input)
	err := s.Start()
	if assErr := assertError(err, nil); assErr != nil {
		t.Fatal(assErr)
	}
	time.Sleep(1 * time.Second)
}

func tempTestDir() string {
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, fmt.Sprintf("pprof-service-test-%d-%d", os.Getpid(), rand.Int()))
	os.RemoveAll(testDir)
	return testDir
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
