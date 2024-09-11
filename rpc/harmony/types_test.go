package rpc

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var (
	testAddr1Str  = "one1upj2dzv5ayuqy5x0aclgcr32chqfy32glsdusk"
	testAddr2Str  = "one1k860e6h0sen6ap5fymzwpqtmqlkut2fcus840l"
	testAddr1JStr = fmt.Sprintf(`"%v"`, testAddr1Str)
	testAddr2JStr = fmt.Sprintf(`"%v"`, testAddr2Str)

	testAddr1, _ = internal_common.Bech32ToAddress(testAddr1Str)
	testAddr2, _ = internal_common.Bech32ToAddress(testAddr2Str)
)

func TestAddressOrList_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input string
		exp   AddressOrList
	}{
		{
			input: testAddr1JStr,
			exp: AddressOrList{
				Address:     &testAddr1,
				AddressList: nil,
			},
		},
		{
			input: fmt.Sprintf("[%v, %v]", testAddr1JStr, testAddr2JStr),
			exp: AddressOrList{
				Address:     nil,
				AddressList: []common.Address{testAddr1, testAddr2},
			},
		},
	}

	for _, test := range tests {
		var aol *AddressOrList
		if err := json.Unmarshal([]byte(test.input), &aol); err != nil {
			t.Fatal(err)
		}
		if err := checkAddressOrListEqual(aol, &test.exp); err != nil {
			t.Error(err)
		}
	}
}

func checkAddressOrListEqual(a, b *AddressOrList) error {
	if (a.Address != nil) != (b.Address != nil) {
		return errors.New("address not equal")
	}
	if a.Address != nil && *a.Address != *b.Address {
		return errors.New("address not equal")
	}
	if len(a.AddressList) != len(b.AddressList) {
		return errors.New("address list size not equal")
	}
	for i, addr1 := range a.AddressList {
		addr2 := b.AddressList[i]
		if addr1 != addr2 {
			return errors.New("address list elem not equal")
		}
	}
	return nil
}

func TestDelegation_IntoStructuredResponse(t *testing.T) {
	d := Delegation{
		ValidatorAddress: "one1hwe68yprkhp5sqq5u7sm9uqu8jxz87fd7ffex7",
		DelegatorAddress: "one1c5yja54ksccgmn4njz5w4cqyjwhqatlly7gkm3",
		Amount:           big.NewInt(1000),
		Reward:           big.NewInt(1014),
		Undelegations:    make([]Undelegation, 0),
	}
	rs1, err := NewStructuredResponse(d)
	require.NoError(t, err)

	rs2 := d.IntoStructuredResponse()

	js1, err := json.Marshal(rs1)
	require.NoError(t, err)

	js2, err := json.Marshal(rs2)
	require.NoError(t, err)

	require.JSONEq(t, string(js1), string(js2))
}
