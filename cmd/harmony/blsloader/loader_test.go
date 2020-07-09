package blsloader

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/multibls"
)

type testKey struct {
	publicKey  string
	privateKey string
	passphrase string
	passFile   string
	path       string
}

// validTestKeys are keys with valid password and valid .pass file
var validTestKeys = []testKey{
	{
		// key with empty passphrase
		publicKey:  "0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500",
		privateKey: "78c88c331195591b396e3205830071901a7a79e14fd0ede7f06bfb4c5e9f3473",
		passphrase: "",
		passFile:   "testData/blskey_passphrase/0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500.pass",
		path:       "testData/blskey_passphrase/0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500.key",
	},
	{
		// key with non empty passphrase
		publicKey:  "152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083",
		privateKey: "c20fa8de733d08e27e3101436d41f6a3207b8bedad7525c6e91a77ae2a49cf56",
		passphrase: "harmony",
		passFile:   "testData/blskey_passphrase/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.pass",
		path:       "testData/blskey_passphrase/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.key",
	},
}

// emptyPassTestKeys are keys with valid password but empty .pass file
var emptyPassTestKeys = []testKey{
	{
		// key with empty passphrase
		publicKey:  "0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500",
		privateKey: "78c88c331195591b396e3205830071901a7a79e14fd0ede7f06bfb4c5e9f3473",
		passphrase: "",
		path:       "testData/blskey_emptypass/0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500.key",
	},
	{
		// key with non empty passphrase
		publicKey:  "152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083",
		privateKey: "c20fa8de733d08e27e3101436d41f6a3207b8bedad7525c6e91a77ae2a49cf56",
		passphrase: "harmony",
		path:       "testData/blskey_emptypass/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.key",
	},
}

// wrongPassTestKeys are keys with wrong pass file and wrong passphrase
var wrongPassTestKeys = []testKey{
	{
		// key with empty passphrase
		publicKey:  "0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500",
		privateKey: "78c88c331195591b396e3205830071901a7a79e14fd0ede7f06bfb4c5e9f3473",
		passphrase: "evil harmony",
		passFile:   "testData/blskey_wrongpass/0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500.pass",
		path:       "testData/blskey_wrongpass/0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500.key",
	},
	{
		// key with non empty passphrase
		publicKey:  "152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083",
		privateKey: "c20fa8de733d08e27e3101436d41f6a3207b8bedad7525c6e91a77ae2a49cf56",
		passphrase: "harmony",
		passFile:   "testData/blskey_wrongpass/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.pass",
		path:       "testData/blskey_wrongpass/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.key",
	},
}

func TestLoadKeys_SingleBls_File(t *testing.T) {
	tests := []struct {
		cfg    Config
		inputs []string

		expOutputs []string
		expPubKeys []string
		expErr     error
	}{
		{
			// load the default pass file with file
			cfg: Config{
				BlsKeyFile:  &validTestKeys[0].path,
				PassSrcType: PassSrcFile,
			},
			inputs: []string{},

			expOutputs: []string{
				fmt.Sprintf("loaded bls key %s\n", validTestKeys[0].publicKey),
			},
			expPubKeys: []string{validTestKeys[0].publicKey},
		},
		{
			// load the default pass file with file
			cfg: Config{
				BlsKeyFile:  &validTestKeys[1].path,
				PassSrcType: PassSrcFile,
			},
			inputs: []string{},

			expOutputs: []string{
				fmt.Sprintf("loaded bls key %s\n", validTestKeys[1].publicKey),
			},
			expPubKeys: []string{validTestKeys[1].publicKey},
		},
		{
			// load key file with prompt
			cfg: Config{
				BlsKeyFile:  &validTestKeys[1].path,
				PassSrcType: PassSrcPrompt,
			},
			inputs: []string{validTestKeys[1].passphrase},

			expOutputs: []string{
				fmt.Sprintf("Enter passphrase for the BLS key file %s:", validTestKeys[1].path),
				fmt.Sprintf("loaded bls key %s\n", validTestKeys[1].publicKey),
			},
			expPubKeys: []string{validTestKeys[1].publicKey},
		},
		{
			// Automatically use pass file
			cfg: Config{
				BlsKeyFile:  &validTestKeys[1].path,
				PassSrcType: PassSrcAuto,
			},
			inputs: []string{},

			expOutputs: []string{
				fmt.Sprintf("loaded bls key %s\n", validTestKeys[1].publicKey),
			},
			expPubKeys: []string{validTestKeys[1].publicKey},
		},
		{
			// Automatically use prompt
			cfg: Config{
				BlsKeyFile:  &emptyPassTestKeys[1].path,
				PassSrcType: PassSrcAuto,
			},
			inputs: []string{emptyPassTestKeys[1].passphrase},

			expOutputs: []string{
				"unable to get passphrase from passphrase file testData/blskey_emptypass/152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083.pass: cannot open passphrase file\n",
				fmt.Sprintf("Enter passphrase for the BLS key file %s:", emptyPassTestKeys[1].path),
				fmt.Sprintf("loaded bls key %s\n", emptyPassTestKeys[1].publicKey),
			},
			expPubKeys: []string{emptyPassTestKeys[1].publicKey},
		},
	}
	for i, test := range tests {
		ts := &testSuite{
			cfg:        test.cfg,
			inputs:     test.inputs,
			expOutputs: test.expOutputs,
			expErr:     test.expErr,
			expPubKeys: test.expPubKeys,
		}
		ts.init()
		ts.process()
		fmt.Println(111)
		if err := ts.checkResult(); err != nil {
			t.Errorf("test %v: %v", i, err)
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

type testSuite struct {
	cfg    Config
	inputs []string

	expOutputs []string
	expPubKeys []string
	expErr     error

	gotKeys    multibls.PrivateKeys
	gotOutputs []string

	timeout time.Duration
	console *testConsole
	gotErr  error // err returned from load key
	errC    chan error
	wg      sync.WaitGroup
}

func (ts *testSuite) init() {
	ts.gotOutputs = make([]string, 0, len(ts.expOutputs))
	ts.console = newTestConsole()
	setTestConsole(ts.console)
	ts.timeout = 1 * time.Second
	ts.errC = make(chan error, 3)
}

func (ts *testSuite) process() {
	ts.wg.Add(3)
	go ts.threadedLoadOutputs()
	go ts.threadedFeedConsoleInputs()
	go ts.threadLoadKeys()

	ts.wg.Wait()
}

func (ts *testSuite) checkResult() error {
	if err := assertError(ts.gotErr, ts.expErr); err != nil {
		return err
	}
	select {
	case err := <-ts.errC:
		return err
	default:
	}
	fmt.Println("got outputs:", ts.gotOutputs)
	fmt.Println("expect outputs:", ts.expOutputs)
	if isClean, msg := ts.console.checkClean(); !isClean {
		return fmt.Errorf("console not clean: %v", msg)
	}
	if ts.expErr != nil {
		return nil
	}
	if err := ts.checkKeys(); err != nil {
		return err
	}
	if err := ts.checkOutputs(); err != nil {
		return err
	}
	return nil
}

func (ts *testSuite) checkOutputs() error {
	if len(ts.gotOutputs) != len(ts.expOutputs) {
		return fmt.Errorf("output size not expected: %v / %v", len(ts.gotOutputs), len(ts.expOutputs))
	}
	for i, gotOutput := range ts.gotOutputs {
		expOutput := ts.expOutputs[i]
		if gotOutput != expOutput {
			return fmt.Errorf("%vth output unexpected: [%v] / [%v]", i, gotOutput, expOutput)
		}
	}
	return nil
}

func (ts *testSuite) checkKeys() error {
	if len(ts.expPubKeys) != len(ts.gotKeys) {
		return fmt.Errorf("loaded key size not expected: %v / %v", len(ts.gotKeys), len(ts.expPubKeys))
	}
	expKeyMap := make(map[bls.SerializedPublicKey]struct{})
	for _, pubKeyStr := range ts.expPubKeys {
		pubKey := pubStrToPubBytes(pubKeyStr)
		if _, exist := expKeyMap[pubKey]; exist {
			return fmt.Errorf("duplicate expect pubkey %x", pubKey)
		}
		expKeyMap[pubKey] = struct{}{}
	}
	gotVisited := make(map[bls.SerializedPublicKey]struct{})
	for _, gotPubWrapper := range ts.gotKeys.GetPublicKeys() {
		pubKey := gotPubWrapper.Bytes
		if _, exist := gotVisited[pubKey]; exist {
			return fmt.Errorf("duplicate got pubkey %x", pubKey)
		}
		if _, exist := expKeyMap[pubKey]; !exist {
			return fmt.Errorf("got pubkey not found in expect %x", pubKey)
		}
	}
	return nil
}

func (ts *testSuite) threadLoadKeys() {
	defer ts.wg.Done()

	ts.gotKeys, ts.gotErr = LoadKeys(ts.cfg)
	return
}

func (ts *testSuite) threadedFeedConsoleInputs() {
	defer ts.wg.Done()

	i := 0
	for i < len(ts.inputs) {
		select {
		case ts.console.In <- ts.inputs[i]:
			i += 1
		case <-time.After(ts.timeout):
			ts.errC <- errors.New("feed inputs timed out")
			return
		}
	}
}

func (ts *testSuite) threadedLoadOutputs() {
	defer ts.wg.Done()
	var (
		i = 0
	)
	for i < len(ts.expOutputs) {
		select {
		case got := <-ts.console.Out:
			ts.gotOutputs = append(ts.gotOutputs, got)
			i++
		case <-time.After(ts.timeout):
			ts.errC <- errors.New("load outputs timed out")
			return
		}
	}
}

func pubStrToPubBytes(str string) bls.SerializedPublicKey {
	b := common.Hex2Bytes(str)
	var pubKey bls.SerializedPublicKey
	copy(pubKey[:], b)
	return pubKey
}
