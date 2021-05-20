package rpc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/pkg/errors"
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
