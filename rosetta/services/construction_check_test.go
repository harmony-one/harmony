package services

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/pkg/errors"
)

func TestConstructMetadataOptions(t *testing.T) {
	toShard := uint32(1)
	fromShard := uint32(0)
	refTxMedata := &TransactionMetadata{
		ToShardID:   &toShard,
		FromShardID: &fromShard,
	}
	refGasPrice := float64(10)

	cases := []struct {
		Metadata    ConstructMetadataOptions
		ExpectError bool
	}{
		{
			Metadata: ConstructMetadataOptions{
				TransactionMetadata: refTxMedata,
				GasPriceMultiplier:  nil,
			},
			ExpectError: false,
		},
		{
			Metadata: ConstructMetadataOptions{
				TransactionMetadata: refTxMedata,
				GasPriceMultiplier:  &refGasPrice,
			},
			ExpectError: false,
		},
		{
			Metadata: ConstructMetadataOptions{
				TransactionMetadata: nil,
				GasPriceMultiplier:  &refGasPrice,
			},
			ExpectError: true,
		},
		{
			Metadata: ConstructMetadataOptions{
				TransactionMetadata: nil,
				GasPriceMultiplier:  nil,
			},
			ExpectError: true,
		},
	}

	for i, test := range cases {
		mapString, err := types.MarshalMap(test.Metadata)
		if err != nil {
			t.Error(err)
			continue
		}
		ref := ConstructMetadataOptions{}
		err = ref.UnmarshalFromInterface(mapString)
		if test.ExpectError && err == nil {
			t.Errorf("expected error for test %v", i)
			continue
		} else if !test.ExpectError && err != nil {
			t.Error(errors.WithMessage(err, fmt.Sprintf("error for test %v", i)))
			continue
		}
		if !test.ExpectError && types.Hash(test.Metadata) != types.Hash(ref) {
			t.Errorf("unmarshalled metadata doesn't match ref metadata for test %v", i)
		}
	}

	ref := ConstructMetadataOptions{}
	if err := ref.UnmarshalFromInterface(false); err == nil {
		t.Error("expect error")
	}
}

func TestConstructMetadata(t *testing.T) {
	toShard := uint32(1)
	fromShard := uint32(0)
	refTxMedata := &TransactionMetadata{
		ToShardID:   &toShard,
		FromShardID: &fromShard,
	}

	cases := []struct {
		Metadata    ConstructMetadata
		ExpectError bool
	}{
		{
			Metadata: ConstructMetadata{
				Nonce:       0,
				GasLimit:    12000,
				GasPrice:    big.NewInt(1e3),
				Transaction: refTxMedata,
			},
			ExpectError: false,
		},
		{
			Metadata: ConstructMetadata{
				Nonce:       0,
				GasLimit:    12000,
				GasPrice:    big.NewInt(1e3),
				Transaction: nil,
			},
			ExpectError: true,
		},
		{
			Metadata: ConstructMetadata{
				Nonce:       0,
				GasLimit:    12000,
				GasPrice:    nil,
				Transaction: refTxMedata,
			},
			ExpectError: true,
		},
		{
			Metadata: ConstructMetadata{
				Nonce:       0,
				GasLimit:    12000,
				GasPrice:    nil,
				Transaction: nil,
			},
			ExpectError: true,
		},
	}

	for i, test := range cases {
		mapString, err := types.MarshalMap(test.Metadata)
		if err != nil {
			t.Error(err)
			continue
		}

		ref := ConstructMetadata{}
		err = ref.UnmarshalFromInterface(mapString)
		if test.ExpectError && err == nil {
			t.Errorf("expected error for test %v", i)
			continue
		} else if !test.ExpectError && err != nil {
			t.Error(errors.WithMessage(err, fmt.Sprintf("error for test %v", i)))
			continue
		}
		if !test.ExpectError && types.Hash(test.Metadata) != types.Hash(ref) {
			t.Errorf("unmarshalled metadata doesn't match ref metadata for test %v", i)
		}
	}

	ref := ConstructMetadata{}
	if err := ref.UnmarshalFromInterface(false); err == nil {
		t.Error("expect error")
	}
}

func TestGetSuggestedFeeAndPrice(t *testing.T) {
	refEstGasUsed := big.NewInt(1000000)

	cases := []struct {
		GasMul         float64
		EstGasUsed     *big.Int
		RefGasPrice    *big.Int
		RefAmountValue *big.Int
	}{
		{
			GasMul:         -1.111111,
			EstGasUsed:     refEstGasUsed,
			RefGasPrice:    big.NewInt(DefaultGasPrice),
			RefAmountValue: new(big.Int).Mul(big.NewInt(DefaultGasPrice), refEstGasUsed),
		},
		{
			GasMul:         0,
			EstGasUsed:     refEstGasUsed,
			RefGasPrice:    big.NewInt(DefaultGasPrice),
			RefAmountValue: new(big.Int).Mul(big.NewInt(DefaultGasPrice), refEstGasUsed),
		},
		{
			GasMul:         1,
			EstGasUsed:     refEstGasUsed,
			RefGasPrice:    big.NewInt(DefaultGasPrice),
			RefAmountValue: new(big.Int).Mul(big.NewInt(DefaultGasPrice), refEstGasUsed),
		},
		{
			GasMul:         1.5,
			EstGasUsed:     refEstGasUsed,
			RefGasPrice:    big.NewInt(DefaultGasPrice * 1.5),
			RefAmountValue: new(big.Int).Mul(big.NewInt(DefaultGasPrice*1.5), refEstGasUsed),
		},
		{
			GasMul:         2,
			EstGasUsed:     refEstGasUsed,
			RefGasPrice:    big.NewInt(DefaultGasPrice * 2),
			RefAmountValue: new(big.Int).Mul(big.NewInt(DefaultGasPrice*2), refEstGasUsed),
		},
	}

	for i, test := range cases {
		refAmounts, refPrice := getSuggestedFeeAndPrice(test.GasMul, test.EstGasUsed)
		if len(refAmounts) != 1 {
			t.Errorf("expect exactly 1 amount for case %v", i)
			continue
		}
		refAmountValue, err := types.AmountValue(refAmounts[0])
		if err != nil {
			t.Error(err)
			continue
		}
		if refAmountValue.Cmp(test.RefAmountValue) != 0 {
			t.Errorf("refrence amount value %v != got value %v for case %v",
				refAmountValue, test.RefAmountValue, i,
			)
		}
		if refPrice.Cmp(test.RefGasPrice) != 0 {
			t.Errorf("refrence gas price %v != got gas price %v for case %v",
				refPrice, test.RefGasPrice, i,
			)
		}
	}

}
