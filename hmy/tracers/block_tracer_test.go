package tracers

import (
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/utils"
)

var TestActions = []string{
	`{"callType":"call","value":"0x9f2d9ea5d38f03446","to":"0xf012702a5f0e54015362cbca26a26fc90aa832a3","gas":"0x177d97","from":"0x5a42560d64136caef1a92d4c0829c21385f9f182","input":"0xfb3bdb41000000000000000000000000000000000000000000000001a8a909dfcef4000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000005a42560d64136caef1a92d4c0829c21385f9f1820000000000000000000000000000000000000000000000000000000061cecc5c0000000000000000000000000000000000000000000000000000000000000002000000000000000000000000cf664087a5bb0237a0bad6742852ec6c8d69a27a000000000000000000000000a4e24f10712cec820dd04e35d52f8087a0699d19"}`,
	`{"callType":"staticcall","to":"0x3fe7910191e07503a94237303c8c0497549b0b20","gas":"0x17120d","from":"0xf012702a5f0e54015362cbca26a26fc90aa832a3","input":"0x0902f1ac"}`,
	`{"callType":"delegatecall","to":"0x4a5dca217eea6ec5f3a2f81859f0698aaf7668b9","gas":"0x215170","from":"0x5f753dcdf9b1ad9aabc1346614d1f4746fd6ce5c","input":"0x6352211e000000000000000000000000000000000000000000000000000000000000bd5e"}`,
	`{"from":"0x84eaa517a51445e35a368885da3d30cd9ab2ead4","gas":"0x659087","init":"0x608060405234801561001057600080fd5b5060405161033a38038061033a8339818101604052602081101561003357600080fd5b810190808051906020019092919050505080600081905550506102df8061005b6000396000f3fe6080604052600436106100295760003560e01c80638100626b1461002e5780639eb7a67f1461007c575b600080fd5b61007a6004803603604081101561004457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001909291905050506100d7565b005b34801561008857600080fd5b506100d56004803603604081101561009f57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019092919050505061020a565b005b60008114156100e557610206565b8173ffffffffffffffffffffffffffffffffffffffff16638100626b6002348161010b57fe5b0430600185036040518463ffffffff1660e01b8152600401808373ffffffffffffffffffffffffffffffffffffffff168152602001828152602001925050506000604051808303818588803b15801561016357600080fd5b505af1158015610177573d6000803e3d6000fd5b50505050508173ffffffffffffffffffffffffffffffffffffffff16639eb7a67f30836040518363ffffffff1660e01b8152600401808373ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b1580156101ed57600080fd5b505af1158015610201573d6000803e3d6000fd5b505050505b5050565b6001811115610218576102a5565b8173ffffffffffffffffffffffffffffffffffffffff16639eb7a67f83600184016040518363ffffffff1660e01b8152600401808373ffffffffffffffffffffffffffffffffffffffff16815260200182815260200192505050600060405180830381600087803b15801561028c57600080fd5b505af11580156102a0573d6000803e3d6000fd5b505050505b505056fea2646970667358221220eada8b07d177afc51b40b49ee90fe29274e3a002f2138a9023a11463dd82d4f164736f6c634300060c003300000000000000000000000000000000000000000000000000000000000001c8","value":"0x0"}`,
	`{"refundAddress":"0xf012702a5f0e54015362cbca26a26fc90aa832a3","balance":"0x9f2d9ea5d38f03446","address":"0xf012702a5f0e54015362cbca26a26fc90aa832a3"}`,
}

func unmarshalAction(jsonstr string) (*action, error) {
	actionInterface := make(map[string]string)
	if err := json.Unmarshal([]byte(jsonstr), &actionInterface); err != nil {
		return nil, err
	}
	var ac action
	if callType, exist := actionInterface["callType"]; exist {
		ac.from = common.HexToAddress(actionInterface["from"])
		ac.to = common.HexToAddress(actionInterface["to"])
		ac.gas, _ = strconv.ParseUint(actionInterface["gas"], 0, 64)
		ac.input = utils.FromHex(actionInterface["input"])
		ac.value = big.NewInt(0)
		ac.value.UnmarshalText([]byte(actionInterface["value"]))
		switch strings.ToUpper(callType) {
		case "CALL":
			ac.op = vm.CALL
		case "CALLCODE":
			ac.op = vm.CALLCODE
		case "DELEGATECALL":
			ac.op = vm.DELEGATECALL
		case "STATICCALL":
			ac.op = vm.STATICCALL
		}
		return &ac, nil
	}
	if initCode, exist := actionInterface["init"]; exist {
		ac.op = vm.CREATE
		ac.from = common.HexToAddress(actionInterface["from"])
		ac.value = big.NewInt(0)
		ac.value.UnmarshalText([]byte(actionInterface["value"]))
		ac.gas, _ = strconv.ParseUint(actionInterface["gas"], 0, 64)
		ac.input = utils.FromHex(initCode)
		return &ac, nil
	}
	if refundAddress, exist := actionInterface["refundAddress"]; exist {
		ac.op = vm.SELFDESTRUCT
		ac.from = common.HexToAddress(actionInterface["address"])
		ac.to = common.HexToAddress(refundAddress)
		ac.value = big.NewInt(0)
		ac.value.UnmarshalText([]byte(actionInterface["balance"]))
		return &ac, nil
	}
	return nil, errors.New("invalid action string")
}

func TestActionMarshal(t *testing.T) {
	for _, jsonStr := range TestActions {
		ac, err := unmarshalAction(jsonStr)
		if err != nil {
			t.Error(err)
		}
		_, acJsonStr, _ := ac.toJsonStr()
		if jsonStr != *acJsonStr {
			t.Errorf("expected %s got %s\n", jsonStr, *acJsonStr)
		}
	}
}
