package params

import (
	"math/big"
	"reflect"
	"testing"
)

func TestCheckCompatible(t *testing.T) {
	type test struct {
		stored, new *ChainConfig
		head        uint64
		wantErr     *ConfigCompatError
	}
	tests := []test{
		{stored: AllProtocolChanges, new: AllProtocolChanges, head: 0, wantErr: nil},
		{stored: AllProtocolChanges, new: AllProtocolChanges, head: 100, wantErr: nil},
		{
			stored:  &ChainConfig{EIP155Block: big.NewInt(10)},
			new:     &ChainConfig{EIP155Block: big.NewInt(20)},
			head:    9,
			wantErr: nil,
		},
		{
			stored: &ChainConfig{S3Block: big.NewInt(30), EIP155Block: big.NewInt(10)},
			new:    &ChainConfig{S3Block: big.NewInt(25), EIP155Block: big.NewInt(20)},
			head:   25,
			wantErr: &ConfigCompatError{
				What:         "EIP155 fork block",
				StoredConfig: big.NewInt(10),
				NewConfig:    big.NewInt(20),
				RewindTo:     9,
			},
		},
	}

	for _, test := range tests {
		err := test.stored.CheckCompatible(test.new, test.head)
		if !reflect.DeepEqual(err, test.wantErr) {
			t.Errorf("error mismatch:\nstored: %v\nnew: %v\nhead: %v\nerr: %v\nwant: %v", test.stored, test.new, test.head, err, test.wantErr)
		}
	}
}
