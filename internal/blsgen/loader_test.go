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
		return
	}

	fmt.Printf("loaded %v keys\n", len(keys))
	for i, key := range keys {
		fmt.Printf("  key %v: %x\n", i, key.PublicKey().Serialized())
	}
	// Output:
	//
	// loaded 2 keys
	//   key 0: a60cec91fda57196dce57ebe9bc52c6253dc4d9ac7ba6bdaa98b58bf44e4243fe1933e7b36d0d133d7dcd9696508dcf7
	//   key 1: b4bf083468ec18ba39533df66c7f5107e910e90a04cc3e4d1d113009a2ea4da2092a6b3bfaaaaddb862da4d56a1779f6
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
