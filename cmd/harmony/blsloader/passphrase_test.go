package blsloader

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewPassDecrypter(t *testing.T) {
	// setup
	var (
		testDir            = filepath.Join(baseTestDir, t.Name())
		existPassFile      = filepath.Join(testDir, testKeys[0].publicKey+passExt)
		emptyPassFile      = filepath.Join(testDir, testKeys[1].publicKey+passExt)
		invalidExtPassFile = filepath.Join(testDir, testKeys[1].publicKey+".invalid")
	)
	if err := writeFile(existPassFile, testKeys[0].passphrase); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(invalidExtPassFile, testKeys[1].passphrase); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		config        passDecrypterConfig
		expErr        error
		providerTypes []passProvider
	}{
		{
			config: passDecrypterConfig{passSrcType: PassSrcNil},
			expErr: errors.New("unknown PassSrcType"),
		},
		{
			config: passDecrypterConfig{
				passSrcType: PassSrcFile,
				passFile:    &emptyPassFile,
			},
			expErr: errors.New("no such file or directory"),
		},
		{
			config: passDecrypterConfig{
				passSrcType: PassSrcFile,
				passFile:    &existPassFile,
			},
			expErr: nil,
			providerTypes: []passProvider{
				&staticPassProvider{},
			},
		},
		{
			config: passDecrypterConfig{
				passSrcType: PassSrcFile,
				passFile:    &invalidExtPassFile,
			},
			expErr: errors.New("should have extension .pass"),
		},
		{
			config: passDecrypterConfig{passSrcType: PassSrcPrompt},
			expErr: nil,
			providerTypes: []passProvider{
				&promptPassProvider{},
			},
		},
		{
			config: passDecrypterConfig{
				passSrcType:       PassSrcPrompt,
				persistPassphrase: true,
			},
			expErr: nil,
			providerTypes: []passProvider{
				&promptPassProvider{},
			},
		},
		{
			config: passDecrypterConfig{
				passSrcType: PassSrcAuto,
			},
			expErr: nil,
			providerTypes: []passProvider{
				&dynamicPassProvider{},
				&promptPassProvider{},
			},
		},
		{
			config: passDecrypterConfig{
				passSrcType: PassSrcAuto,
				passFile:    &existPassFile,
			},
			expErr: nil,
			providerTypes: []passProvider{
				&staticPassProvider{},
				&promptPassProvider{},
			},
		},
	}
	for i, test := range tests {
		decrypter, err := newPassDecrypter(test.config)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if err != nil {
			continue
		}

		if len(decrypter.pps) != len(test.providerTypes) {
			t.Errorf("Test %v: unexpected provider number %v / %v",
				i, len(decrypter.pps), len(test.providerTypes))
			continue
		}
		for ppIndex, gotPP := range decrypter.pps {
			gotType := reflect.TypeOf(gotPP).Elem()
			expType := reflect.TypeOf(test.providerTypes[ppIndex]).Elem()
			if gotType != expType {
				t.Errorf("Test %v: %v passProvider unexpected type: %v / %v",
					i, ppIndex, gotType, expType)
			}
		}
	}
}

func TestPromptPassProvider_getPassphrase(t *testing.T) {
	unitTestDir := filepath.Join(baseTestDir, t.Name())
	tests := []struct {
		setupFunc     func(rootDir string) error
		keyFile       string
		passphrase    string
		enablePersist bool
		extraInput    []string

		expOutputLen       int
		expErr             error
		newPassFileContent bool
		passFileExist      bool
	}{
		{
			setupFunc:          nil,
			keyFile:            testKeys[1].publicKey + basicKeyExt,
			passphrase:         "new key",
			enablePersist:      false,
			extraInput:         []string{},
			expOutputLen:       1, // prompt for passphrase
			passFileExist:      false,
			newPassFileContent: false,
		},
		{
			// new pass file
			setupFunc:          nil,
			keyFile:            testKeys[1].publicKey + basicKeyExt,
			passphrase:         "new key",
			enablePersist:      true,
			extraInput:         []string{},
			expOutputLen:       1, // prompt for passphrase
			passFileExist:      true,
			newPassFileContent: true,
		},
		{
			// exist pass file, not overwrite
			setupFunc: func(rootDir string) error {
				passFile := filepath.Join(rootDir, testKeys[1].publicKey+passExt)
				return writeFile(passFile, "old key")
			},
			keyFile:            testKeys[1].publicKey + basicKeyExt,
			passphrase:         "new key",
			enablePersist:      true,
			extraInput:         []string{"n"},
			expOutputLen:       2, // prompt for passphrase and ask for overwrite
			passFileExist:      true,
			newPassFileContent: false,
		},
		{
			// exist pass file, do overwrite
			setupFunc: func(rootDir string) error {
				passFile := filepath.Join(rootDir, testKeys[1].publicKey+passExt)
				return writeFile(passFile, "old key")
			},
			keyFile:            testKeys[1].publicKey + basicKeyExt,
			passphrase:         "new key",
			enablePersist:      true,
			extraInput:         []string{"y"},
			expOutputLen:       2, // prompt for passphrase and ask for overwrite
			passFileExist:      true,
			newPassFileContent: true,
		},
	}

	for i, test := range tests {
		tc := newTestConsole()
		setTestConsole(tc)
		tcDir := filepath.Join(unitTestDir, fmt.Sprintf("%v", i))
		os.RemoveAll(tcDir)
		os.MkdirAll(tcDir, 0700)
		if test.setupFunc != nil {
			if err := test.setupFunc(tcDir); err != nil {
				t.Fatal(err)
			}
		}
		tc.In <- test.passphrase
		for _, in := range test.extraInput {
			tc.In <- in
		}

		ppd := &promptPassProvider{enablePersist: test.enablePersist}
		keyFile := filepath.Join(tcDir, test.keyFile)
		passphrase, err := ppd.getPassphrase(keyFile)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if passphrase != test.passphrase {
			t.Errorf("Test %v: got unexpected passphrase: %v / %v", i, passphrase, test.passphrase)
			continue
		}
		for index := 0; index != test.expOutputLen; index++ {
			<-tc.Out
		}
		if isClean, msg := tc.checkClean(); !isClean {
			t.Errorf("Test %v: console not clean: %v", i, msg)
			continue
		}
		passFile := keyFileToPassFileFull(keyFile)
		if !test.passFileExist {
			if _, err := os.Stat(passFile); !os.IsNotExist(err) {
				t.Errorf("Test %v: pass file exist %v", i, passFile)
			}
		} else {
			b, err := ioutil.ReadFile(passFile)
			if err != nil {
				t.Error(err)
				continue
			}
			if test.newPassFileContent && string(b) != test.passphrase {
				t.Errorf("Test %v: unexpected passphrase from persist file: %v/ %v",
					i, string(b), test.passphrase)
			}
			if !test.newPassFileContent && string(b) == test.passphrase {
				t.Errorf("Test %v: passphrase content has changed", i)
			}
		}
	}
}
