package blsgen

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/bls"
)

func TestNewPassDecrypter(t *testing.T) {
	// setup
	var (
		testDir       = filepath.Join(baseTestDir, t.Name())
		existPassFile = filepath.Join(testDir, testKeys[0].publicKey+passExt)
		emptyPassFile = filepath.Join(testDir, testKeys[1].publicKey+passExt)
	)
	if err := writeFile(existPassFile, testKeys[0].passphrase); err != nil {
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

func TestPassDecrypter_decryptFile(t *testing.T) {
	setTestConsole(newTestConsole())
	unitTestDir := filepath.Join(baseTestDir, t.Name())
	tests := []struct {
		setupFunc func(rootDir string) error
		providers []passProvider
		keyFile   string

		expPublicKey string
		expErr       error
	}{
		{
			// 1. Two providers, one return err and one return the correct passphrase.
			setupFunc: func(rootDir string) error {
				keyFile := filepath.Join(rootDir, testKeys[1].publicKey+basicKeyExt)
				return writeFile(keyFile, testKeys[1].keyFileData)
			},
			providers: []passProvider{
				&errPassProvider{},
				makeTestPassProvider(),
			},
			keyFile:      testKeys[1].publicKey + basicKeyExt,
			expPublicKey: testKeys[1].publicKey,
		},
		{
			// 2. Only error provider. Return the decryption error
			setupFunc: func(rootDir string) error {
				keyFile := filepath.Join(rootDir, testKeys[1].publicKey+basicKeyExt)
				return writeFile(keyFile, testKeys[1].keyFileData)
			},
			providers: []passProvider{&errPassProvider{}},
			keyFile:   testKeys[1].publicKey + basicKeyExt,
			expErr:    errors.New("failed to load bls key"),
		},
	}
	for i, test := range tests {
		tcDir := filepath.Join(unitTestDir, fmt.Sprintf("%v", i))
		os.RemoveAll(tcDir)
		os.MkdirAll(tcDir, 0700)
		if test.setupFunc != nil {
			if err := test.setupFunc(tcDir); err != nil {
				t.Fatal(err)
			}
		}
		keyFile := filepath.Join(tcDir, test.keyFile)

		decrypter := &passDecrypter{pps: test.providers}
		secret, err := decrypter.decryptFile(keyFile)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		gotPub := bls.FromLibBLSPublicKeyUnsafe(secret.GetPublicKey())[:]
		if expPub := common.Hex2Bytes(test.expPublicKey); !bytes.Equal(gotPub, expPub) {
			t.Errorf("Test %v: unexpected public key %v / %v", i, gotPub, expPub)
		}
	}
}

type testPassProvider struct {
	m map[string]string
}

func makeTestPassProvider() *testPassProvider {
	return &testPassProvider{
		m: map[string]string{
			testKeys[0].publicKey: testKeys[0].passphrase,
			testKeys[1].publicKey: testKeys[1].passphrase,
		},
	}
}

func (provider *testPassProvider) getPassphrase(keyFile string) (string, error) {
	basename := filepath.Base(keyFile)
	publicKey := strings.TrimSuffix(basename, filepath.Ext(basename))
	pass, exist := provider.m[publicKey]
	if !exist {
		return "", errors.New("passphrase not exist")
	}
	return pass, nil
}

type errPassProvider struct{}

func (provider *errPassProvider) getPassphrase(keyFile string) (string, error) {
	return "", errors.New("error intended")
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
			// exist pass file, do not overwrite
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
			b, err := os.ReadFile(passFile)
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

func TestDynamicPassProvider_getPassPhrase(t *testing.T) {
	unitTestDir := filepath.Join(baseTestDir, t.Name())

	tests := []struct {
		setupFunc func(rootDir string) error
		keyFile   string
		expPass   string
		expErr    error
	}{
		{
			setupFunc: func(rootDir string) error {
				passFile := filepath.Join(rootDir, testKeys[1].publicKey+passExt)
				return writeFile(passFile, "passphrase\n")
			},
			keyFile: testKeys[1].publicKey + basicKeyExt,
			expPass: "passphrase",
		},
		{
			keyFile: testKeys[1].publicKey + basicKeyExt,
			expErr:  errors.New("no such file"),
		},
	}
	for i, test := range tests {
		tcDir := filepath.Join(unitTestDir, fmt.Sprintf("%v", i))
		os.RemoveAll(tcDir)
		os.MkdirAll(tcDir, 0700)

		if test.setupFunc != nil {
			if err := test.setupFunc(tcDir); err != nil {
				t.Fatal(err)
			}
		}
		provider := &dynamicPassProvider{}
		keyFile := filepath.Join(tcDir, test.keyFile)
		got, err := provider.getPassphrase(keyFile)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
			continue
		}
		if err != nil {
			continue
		}
		if got != test.expPass {
			t.Errorf("Test %v: unexpected passphrase: %v / %v", i, got, test.expPass)
		}
	}
}
