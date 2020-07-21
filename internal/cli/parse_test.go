package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var (
	testStringFlag1 = StringFlag{
		Name:     "string1",
		DefValue: "default",
	}

	testStringFlag2 = StringFlag{
		Name:     "string2",
		DefValue: "default",
	}

	testIntFlag1 = IntFlag{
		Name:     "int1",
		DefValue: 12345,
	}

	testIntFlag2 = IntFlag{
		Name:     "int2",
		DefValue: 12345,
	}

	testBoolFlag1 = BoolFlag{
		Name:     "bool1",
		DefValue: true,
	}

	testBoolFlag2 = BoolFlag{
		Name:     "bool2",
		DefValue: true,
	}

	testStringSliceFlag1 = StringSliceFlag{
		Name:     "string-slice1",
		DefValue: []string{"string1", "string2"},
	}

	testStringSliceFlag2 = StringSliceFlag{
		Name:     "string-slice2",
		DefValue: []string{"string1", "string2"},
	}
)

func TestGetStringFlagValue(t *testing.T) {
	defer tearDown()

	tests := []struct {
		flags     []Flag
		pflags    []Flag
		args      []string
		queryFlag StringFlag

		expExecErr  error
		expParseErr error
		expVal      string
	}{
		{
			flags:     []Flag{testStringFlag1},
			args:      []string{},
			queryFlag: testStringFlag1,
			expVal:    "default",
		},
		{
			flags:     []Flag{testStringFlag1},
			args:      []string{"--string1", "custom"},
			queryFlag: testStringFlag1,
			expVal:    "custom",
		},
		{
			pflags:    []Flag{testStringFlag1},
			args:      []string{"--string1", "custom"},
			queryFlag: testStringFlag1,
			expVal:    "custom",
		},
		{
			flags:      []Flag{testStringFlag1},
			args:       []string{"--string1"},
			expExecErr: errors.New("flag needs an argument"),
		},
		{
			flags:       []Flag{testStringFlag1},
			args:        []string{},
			queryFlag:   testStringFlag2,
			expParseErr: errors.New("flag accessed but not defined"),
		},
	}
	for i, test := range tests {
		var (
			val      string
			parseErr error
		)
		SetParseErrorHandle(func(gotErr error) {
			parseErr = gotErr
		})
		cmd := makeTestCommand(func(cmd *cobra.Command, args []string) {
			val = GetStringFlagValue(cmd, test.queryFlag)
		})
		cmd.SetOut(bytes.NewBuffer(nil))
		if err := RegisterFlags(cmd, test.flags); err != nil {
			t.Fatal(err)
		}
		if err := RegisterPFlags(cmd, test.pflags); err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs(test.args)
		execErr := cmd.Execute()

		if err := assertError(execErr, test.expExecErr); err != nil {
			t.Fatalf("Test %v execution: %v", i, err)
		}
		if execErr != nil || test.expExecErr != nil {
			continue
		}
		if err := assertError(parseErr, test.expParseErr); err != nil {
			t.Fatalf("Test %v parse: %v", i, err)
		}
		if parseErr != nil || test.expParseErr != nil {
			continue
		}
		if val != test.expVal {
			t.Errorf("Test %v: unexpected value %v / %v", i, val, test.expVal)
		}
	}
}

func tearDown() {
	parseErrorHandleFunc = nil
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
