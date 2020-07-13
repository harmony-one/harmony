package blsgen

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
)

const (
	testExt = ".test1"
)

func TestNewMultiKeyLoader(t *testing.T) {
	tests := []struct {
		keyFiles   []string
		decrypters []keyDecrypter
		expErr     error
	}{
		{
			keyFiles: []string{
				"test/keyfile1.key",
				"keyfile2.bls",
			},
			decrypters: []keyDecrypter{
				&passDecrypter{},
				&kmsDecrypter{},
			},
			expErr: nil,
		},
		{
			keyFiles: []string{
				"test/keyfile1.key",
				"keyfile2.bls",
			},
			decrypters: []keyDecrypter{
				&passDecrypter{},
			},
			expErr: errors.New("unsupported key extension"),
		},
	}
	for i, test := range tests {
		_, err := newMultiKeyLoader(test.keyFiles, test.decrypters)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
	}
}

func TestMultiKeyLoader_loadKeys(t *testing.T) {
	setTestConsole(newTestConsole())

	unitTestDir := filepath.Join(baseTestDir, t.Name())
	os.Remove(unitTestDir)
	os.MkdirAll(unitTestDir, 0700)

	keyFile1 := filepath.Join(unitTestDir, testKeys[0].publicKey+basicKeyExt)
	if err := writeFile(keyFile1, testKeys[0].keyFileData); err != nil {
		t.Fatal(err)
	}
	keyFile2 := filepath.Join(unitTestDir, testKeys[1].publicKey+testExt)
	if err := writeFile(keyFile2, testKeys[1].keyFileData); err != nil {
		t.Fatal(err)
	}
	passFile1 := filepath.Join(unitTestDir, testKeys[0].publicKey+passExt)
	if err := writeFile(passFile1, testKeys[0].passphrase); err != nil {
		t.Fatal(err)
	}
	decrypters := map[string]keyDecrypter{
		basicKeyExt: &passDecrypter{pps: []passProvider{&dynamicPassProvider{}}},
		testExt:     newTestPassDecrypter(),
	}

	loader := &multiKeyLoader{
		keyFiles:      []string{keyFile1, keyFile2},
		decrypters:    decrypters,
		loadedSecrets: make([]*bls_core.SecretKey, 0, 2),
	}
	keys, err := loader.loadKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("unexpected number of keys: %v / 2", len(keys))
	}
	gotPubs := [][]byte{
		keys[0].Pub.Bytes[:],
		keys[1].Pub.Bytes[:],
	}
	expPubs := [][]byte{
		common.Hex2Bytes(testKeys[0].publicKey),
		common.Hex2Bytes(testKeys[1].publicKey),
	}
	for i := range gotPubs {
		got, exp := gotPubs[i], expPubs[i]
		if !bytes.Equal(got, exp) {
			t.Fatalf("%v pubkey unexpected: %x / %x", i, got, exp)
		}
	}
}

func TestBlsDirLoader(t *testing.T) {
	setTestConsole(newTestConsole())

	unitTestDir := filepath.Join(baseTestDir, t.Name())
	os.Remove(unitTestDir)
	os.MkdirAll(unitTestDir, 0700)

	keyFile1 := filepath.Join(unitTestDir, testKeys[0].publicKey+basicKeyExt)
	if err := writeFile(keyFile1, testKeys[0].keyFileData); err != nil {
		t.Fatal(err)
	}
	keyFile2 := filepath.Join(unitTestDir, testKeys[1].publicKey+testExt)
	if err := writeFile(keyFile2, testKeys[1].keyFileData); err != nil {
		t.Fatal(err)
	}
	passFile1 := filepath.Join(unitTestDir, testKeys[0].publicKey+passExt)
	if err := writeFile(passFile1, testKeys[0].passphrase); err != nil {
		t.Fatal(err)
	}
	// write a file without the given extension
	if err := writeFile(filepath.Join(unitTestDir, "unknown.ext"), "random string"); err != nil {
		t.Fatal(err)
	}
	decrypters := []keyDecrypter{
		&passDecrypter{pps: []passProvider{&dynamicPassProvider{}}},
		newTestPassDecrypter(),
	}

	loader, err := newBlsDirLoader(unitTestDir, decrypters)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := loader.loadKeys()
	if err != nil {
		t.Fatal(err)
	}

	if len(keys) != 2 {
		t.Fatalf("unexpected number of keys: %v / 2", len(keys))
	}
	gotPubs := [][]byte{
		keys[0].Pub.Bytes[:],
		keys[1].Pub.Bytes[:],
	}
	expPubs := [][]byte{
		common.Hex2Bytes(testKeys[0].publicKey),
		common.Hex2Bytes(testKeys[1].publicKey),
	}
	for i := range gotPubs {
		got, exp := gotPubs[i], expPubs[i]
		if !bytes.Equal(got, exp) {
			t.Fatalf("%v pubkey unexpected: %x / %x", i, got, exp)
		}
	}
}

type testPassDecrypter struct {
	pd passDecrypter
}

func newTestPassDecrypter() *testPassDecrypter {
	provider := &testPassProvider{m: map[string]string{
		testKeys[0].publicKey: testKeys[0].passphrase,
		testKeys[1].publicKey: testKeys[1].passphrase,
	}}
	return &testPassDecrypter{
		pd: passDecrypter{
			pps: []passProvider{provider},
		},
	}
}

func (decrypter *testPassDecrypter) extension() string {
	return testExt
}

func (decrypter *testPassDecrypter) decryptFile(keyFile string) (*bls_core.SecretKey, error) {
	return decrypter.pd.decryptFile(keyFile)
}
