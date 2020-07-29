package blsgen

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func ExampleLoadKeys() {
	dir, err := prepareDataForExample()
	if err != nil {
		fmt.Println(err)
		return
	}
	config := Config{
		BlsDir:        &dir,
		PassSrcType:   PassSrcFile,  // not assign PassFile to dynamically use .pass path
		AwsCfgSrcType: AwsCfgSrcNil, // disable loading file with kms
	}

	keys, err := LoadKeys(config)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("loaded %v keys\n", len(keys))
	for i, key := range keys {
		fmt.Printf("  key %v: %x\n", i, key.Pub.Bytes)
	}
	// Output:
	//
	// loaded 2 keys
	//   key 0: 0e969f8b302cf7648bc39652ca7a279a8562b72933a3f7cddac2252583280c7c3495c9ae854f00f6dd19c32fc5a17500
	//   key 1: 152beed46d7a0002ef0f960946008887eedd4775bdf2ed238809aa74e20d31fdca267443615cc6f4ede49d58911ee083
}

func prepareDataForExample() (string, error) {
	unitTestDir := filepath.Join(baseTestDir, "ExampleLoadKeys")
	os.Remove(unitTestDir)
	os.MkdirAll(unitTestDir, 0700)

	if err := writeKeyAndPass(unitTestDir, testKeys[0]); err != nil {
		return "", err
	}
	if err := writeKeyAndPass(unitTestDir, testKeys[1]); err != nil {
		return "", err
	}
	return unitTestDir, nil
}

func writeKeyAndPass(dir string, key testKey) error {
	keyFile := filepath.Join(dir, key.publicKey+basicKeyExt)
	if err := writeFile(keyFile, key.keyFileData); err != nil {
		return fmt.Errorf("cannot write key file data: %v", err)
	}
	passFile := filepath.Join(dir, key.publicKey+passExt)
	if err := writeFile(passFile, key.passphrase); err != nil {
		return fmt.Errorf("cannot write pass file data: %v", err)
	}
	return nil
}

func TestGetKeyDecrypters(t *testing.T) {
	tests := []struct {
		config   Config
		expTypes []keyDecrypter
		expErr   error
	}{
		{
			config: Config{
				PassSrcType:   PassSrcNil,
				AwsCfgSrcType: AwsCfgSrcNil,
			},
			expErr: errors.New("must provide at least one bls key decryption"),
		},
		{
			config: Config{
				PassSrcType:   PassSrcFile,
				AwsCfgSrcType: AwsCfgSrcShared,
			},
			expTypes: []keyDecrypter{
				&passDecrypter{},
				&kmsDecrypter{},
			},
		},
	}
	for i, test := range tests {
		decrypters, err := getKeyDecrypters(test.config)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}
		if len(decrypters) != len(test.expTypes) {
			t.Errorf("Test %v: unexpected decrypter size: %v / %v", i, len(decrypters), len(test.expTypes))
			continue
		}
		for ti := range decrypters {
			gotType := reflect.TypeOf(decrypters[ti]).Elem()
			expType := reflect.TypeOf(test.expTypes[ti]).Elem()
			if gotType != expType {
				t.Errorf("Test %v: %v decrypter type unexpected: %v / %v", i, ti, gotType, expType)
			}
		}
	}
}
