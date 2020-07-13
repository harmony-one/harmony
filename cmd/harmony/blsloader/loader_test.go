package blsloader

import (
	"errors"
	"reflect"
	"testing"
)

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
