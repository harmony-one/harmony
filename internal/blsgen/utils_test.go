package blsgen

import (
	"fmt"
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
		publicKey:   "0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500",
		privateKey:  "78c88c331195591b396e3205830071901a7a79e14fd0ede7f06bfb4c5e9f3473",
		passphrase:  "",
		keyFileData: "1d97f32175d8875f251e15805fd08f0cda794d827cb02d2de7b10d10f36f951d68347bef1e7a3018bd865c6966219cd9c4d20b055c50f8e09a6a3a1666b7c112450f643cc3c175f541fae75da8a843d47993fe89ec85788fd6ea2e98",
	},
	{
		// key with non empty passphrase
		publicKey:   "152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083",
		privateKey:  "c20fa8de733d08e27e3101436d41f6a3207b8bedad7525c6e91a77ae2a49cf56",
		passphrase:  "harmony",
		keyFileData: "194a2d68c37f037f36b28a560402d64ab007f949313b63d9a08f5adb55a061681c70d9119df2d2cdcae5da6e484550c03bad63aae7c1332a3647ce633999ac4ddbb4a40e213c7e88e604784fef40da9d2f28b392c9fb2462f5e51e9c",
	},
}

func writeFile(file string, data string) error {
	dir := filepath.Dir(file)
	os.MkdirAll(dir, 0700)
	return os.WriteFile(file, []byte(data), 0600)
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
