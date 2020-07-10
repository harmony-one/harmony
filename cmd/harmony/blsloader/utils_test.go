package blsloader

import (
	"fmt"
	"strings"
	"testing"
)

const testPrompt = yesNoPrompt

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		inputs     []string
		lenOutputs int
		expRes     bool
		expErr     error
	}{
		{
			inputs:     []string{"yes"},
			lenOutputs: 1,
			expRes:     true,
		},
		{
			inputs:     []string{"YES\n"},
			lenOutputs: 1,
			expRes:     true,
		},
		{
			inputs:     []string{"y"},
			lenOutputs: 1,
			expRes:     true,
		},
		{
			inputs:     []string{"Y"},
			lenOutputs: 1,
			expRes:     true,
		},
		{
			inputs:     []string{"\tY"},
			lenOutputs: 1,
			expRes:     true,
		},
		{
			inputs:     []string{"No"},
			lenOutputs: 1,
			expRes:     false,
		},
		{
			inputs:     []string{"\tn"},
			lenOutputs: 1,
			expRes:     false,
		},
		{
			inputs:     []string{"invalid input", "y"},
			lenOutputs: 2,
			expRes:     true,
		},
	}
	for i, test := range tests {
		tc := newTestConsole()
		setTestConsole(tc)
		for _, input := range test.inputs {
			tc.In <- input
		}

		got, err := promptYesNo(testPrompt)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		} else if assErr != nil {
			continue
		}

		// check results
		if got != test.expRes {
			t.Errorf("Test %v: result unexpected %v / %v", i, got, test.expRes)
		}
		gotOutputs := drainOutCh(tc.Out)
		if len(gotOutputs) != test.lenOutputs {
			t.Errorf("unexpected output size: %v / %v", len(gotOutputs), test.lenOutputs)
		}
		if clean, msg := tc.checkClean(); !clean {
			t.Errorf("Test %v: console unclean with message [%v]", i, msg)
		}
	}
}

func drainOutCh(c chan string) []string {
	var res []string
	for {
		select {
		case gotOut := <-c:
			res = append(res, gotOut)
		default:
			return res
		}
	}
}

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	if (got == nil) || (expect == nil) {
		return nil
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	return nil
}
