package blsgen

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
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

func genBLSKeyWithPassPhrase(dir string, passphrase string) (bls.SecretKey, string, error) {
	privateKey := bls.RandSecretKey()
	publickKey := privateKey.PublicKey()
	fileName := filepath.Join(dir, publickKey.ToHex()+".key")
	// Encrypt with passphrase
	encryptedPrivateKeyStr, err := encrypt(privateKey.ToBytes(), passphrase)
	if err != nil {
		return nil, "", err
	}
	// Write to file.
	err = writeFile(fileName, encryptedPrivateKeyStr)
	return privateKey, fileName, err
}

func TestMultiKeyLoader_loadKeys(t *testing.T) {
	setTestConsole(newTestConsole())

	unitTestDir := filepath.Join(baseTestDir, t.Name())
	os.Remove(unitTestDir)
	os.MkdirAll(unitTestDir, 0700)

	key1, keyFile1, err := genBLSKeyWithPassPhrase(unitTestDir, "pass 1")
	if err != nil {
		t.Fatal(err)
	}
	passFile1 := filepath.Join(unitTestDir, key1.PublicKey().ToHex()+passExt)
	if err := writeFile(passFile1, "pass 1"); err != nil {
		t.Fatal(err)
	}
	key2, keyFile2, err := genBLSKeyWithPassPhrase(unitTestDir, "pass 2")
	if err != nil {
		t.Fatal(err)
	}
	passFile2 := filepath.Join(unitTestDir, key2.PublicKey().ToHex()+passExt)
	if err := writeFile(passFile2, "pass 2"); err != nil {
		t.Fatal(err)
	}

	decrypters := map[string]keyDecrypter{
		basicKeyExt: &passDecrypter{pps: []passProvider{&dynamicPassProvider{}}},
		testExt:     newTestPassDecrypter(),
	}

	loader := &multiKeyLoader{
		keyFiles:      []string{keyFile1, keyFile2},
		decrypters:    decrypters,
		loadedSecrets: make([]bls.SecretKey, 0, 2),
	}
	keys, err := loader.loadKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("unexpected number of keys: %v / 2", len(keys))
	}
	gotPubs := [][]byte{
		keys[0].PublicKey().ToBytes(),
		keys[1].PublicKey().ToBytes(),
	}
	expPubs := [][]byte{
		key1.PublicKey().ToBytes(),
		key2.PublicKey().ToBytes(),
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

	key1, _, err := genBLSKeyWithPassPhrase(unitTestDir, "pass 1")
	if err != nil {
		t.Fatal(err)
	}
	passFile1 := filepath.Join(unitTestDir, key1.PublicKey().ToHex()+passExt)
	if err := writeFile(passFile1, "pass 1"); err != nil {
		t.Fatal(err)
	}
	key2, _, err := genBLSKeyWithPassPhrase(unitTestDir, "pass 2")
	if err != nil {
		t.Fatal(err)
	}
	passFile2 := filepath.Join(unitTestDir, key2.PublicKey().ToHex()+passExt)
	if err := writeFile(passFile2, "pass 2"); err != nil {
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
		key1.PublicKey().ToBytes(),
		key2.PublicKey().ToBytes(),
	}
	expPubs := [][]byte{
		key1.PublicKey().ToBytes(),
		key2.PublicKey().ToBytes(),
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

func (decrypter *testPassDecrypter) decryptFile(keyFile string) (bls.SecretKey, error) {
	return decrypter.pd.decryptFile(keyFile)
}
