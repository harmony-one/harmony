package blsgen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPrompt = yesNoPrompt

func init() {
	// Move the test data to temp directory
	os.RemoveAll(baseTestDir)
	os.MkdirAll(baseTestDir, 0777)
}

var baseTestDir = filepath.Join(".testdata")

type testKey struct {
	publicKey   string
	privateKey  string
	passphrase  string
	keyFileData string
}

// testKeys are keys with valid passphrase and valid .pass file
var testKeys = []testKey{
	{
		// key with empty passphrase
		publicKey:   "b4bf083468ec18ba39533df66c7f5107e910e90a04cc3e4d1d113009a2ea4da2092a6b3bfaaaaddb862da4d56a1779f6",
		privateKey:  "560601040a43a7e4f9c988e132c469740da186af1902e511726b207d21ecd988",
		passphrase:  "xxx",
		keyFileData: "d73dc36b3e3684aa5ac745caf60faa6d2ab9cb7b725ac4d67c0834bd6855b1e64766dd61214760654498f58e6952fef5f31a76d8ed6b43050bbd46bc",
	},
	{
		// key with non empty passphrase
		publicKey:   "a60cec91fda57196dce57ebe9bc52c6253dc4d9ac7ba6bdaa98b58bf44e4243fe1933e7b36d0d133d7dcd9696508dcf7",
		privateKey:  "399674fa048ef9ab26eaffc5a8ad7a038ba978c3c1653520d5d2239764943271",
		passphrase:  "yyy",
		keyFileData: "6ba6471b4181799dffa9039e78acd62d826104721efaa83ce4cef263398f3b48e792463d5a191b94e918b83920101a000142bbbfe09271cdd31fac34",
	},
}

func writeFile(file string, data string) error {
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0700)
	return ioutil.WriteFile(file, []byte(data), 0600)
}

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
		gotOutputs := drainCh(tc.Out)
		if len(gotOutputs) != test.lenOutputs {
			t.Errorf("unexpected output size: %v / %v", len(gotOutputs), test.lenOutputs)
		}
		if clean, msg := tc.checkClean(); !clean {
			t.Errorf("Test %v: console unclean with message [%v]", i, msg)
		}
	}
}

func drainCh(c chan string) []string {
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
