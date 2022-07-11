package tracers

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"

	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/utils"
)

var TestJsonsMock = []byte(`{"14054302":[{"blockNumber":14054302,"blockHash":"0x04d7a0d62d3211151db0dadcaebcb1686c4a3df0e551a00c023c651546293975","transactionHash":"0xce49e42e0fbd37a0cfd08c2da3f1acc371ddbc02c428afa123a43663e57953d7","transactionPosition":0,"subtraces":0,"traceAddress":[0],"type":"suicide","action":{"refundAddress":"0x12e49d93588e0056bd25530c3b1e8aac68f4b70a","balance":"0x0","address":"0x7006c42d6fa41844baa53b0388f9542e634cf55a"},"result":null}],"14833359":[{"blockNumber":14833359,"blockHash":"0x6d6660f3d042a145c7f95c408f28cbf036a18eaf603161c2c00ca3f6041d8b52","transactionHash":"0x9fd0daef346c72d51f7482ddc9a466caf52fa6a116ed13ee0c003e57e632b7c0","transactionPosition":0,"subtraces":0,"traceAddress":[],"type":"create","action":{"from":"0x8520021f89450394244cd4abda4cfe2f1b0ef61c","gas":"0x1017d","init":"0x608060405234801561001057600080fd5b50610149806100206000396000f3fe6080604052600436106100295760003560e01c80630c2ad69c1461002e57806315d55b281461007a575b600080fd5b6100646004803603604081101561004457600080fd5b810190808035906020019092919080359060200190929190505050610091565b6040518082815260200191505060405180910390f35b34801561008657600080fd5b5061008f6100a5565b005b600081838161009c57fe5b04905092915050565b6040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260058152602001807f68656c6c6f00000000000000000000000000000000000000000000000000000081525060200191505060405180910390fdfea26469706673582212202f9958b958267c4ed653e54dc0161cfb9b772209cbe086f4a9ac3d967f22f09564736f6c634300060c0033","value":"0x0"},"result":{"address":"0xf29fcf3a375ce5dd1c58f0e8a584ab5d782cc12b","code":"0x6080604052600436106100295760003560e01c80630c2ad69c1461002e57806315d55b281461007a575b600080fd5b6100646004803603604081101561004457600080fd5b810190808035906020019092919080359060200190929190505050610091565b6040518082815260200191505060405180910390f35b34801561008657600080fd5b5061008f6100a5565b005b600081838161009c57fe5b04905092915050565b6040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260058152602001807f68656c6c6f00000000000000000000000000000000000000000000000000000081525060200191505060405180910390fdfea26469706673582212202f9958b958267c4ed653e54dc0161cfb9b772209cbe086f4a9ac3d967f22f09564736f6c634300060c0033","gasUsed":"0x1017d"}},{"blockNumber":14833359,"blockHash":"0x6d6660f3d042a145c7f95c408f28cbf036a18eaf603161c2c00ca3f6041d8b52","transactionHash":"0xc3b81fa2f6786ffd11a588b9d951a39adb46b6e29abad819b0cb09ee32ea7072","transactionPosition":1,"subtraces":2,"traceAddress":[],"type":"call","action":{"callType":"call","value":"0x0","to":"0x4596817192fbbf0142c576ed3e7cfc0e8f40bbbe","gas":"0x2b71c","from":"0x87946ddc76a4c0a75c8ca1f63dffd0612ae6458c","input":"0x1801fbe5aebcf6e3d785238603dd88bb43cbdfcfeb51c95b570113ee65d2f9271d3b59510000000dcdf493a5e1610e23c037bc4c4e04ab9a6d8fe9d0d462ecd8d45643ac"},"result":{"output":"0x0000000000000000000000000000000000000000000000000000000000000001","gasUsed":"0x13c58"}}]}`)

type JsonCallAction struct {
	CallType string         `json:"callType"`
	Value    string         `json:"value"`
	From     common.Address `json:"from"`
	To       common.Address `json:"to"`
	Gas      string         `json:"gas"`
	Input    string         `json:"input"`
}

type JsonCreateAction struct {
	From  common.Address `json:"from"`
	Value string         `json:"value"`
	Gas   string         `json:"gas"`
	Init  string         `json:"init"`
}

type JsonSuicideAction struct {
	RefundAddress common.Address `json:"refundAddress"`
	Balance       string         `json:"balance"`
	Address       common.Address `json:"address"`
}

type JsonCallOutput struct {
	Output  string `json:"output"`
	GasUsed string `json:"gasUsed"`
}

type JsonCreateOutput struct {
	Address common.Address `json:"address"`
	Code    string         `json:"code"`
	GasUsed string         `json:"gasUsed"`
}

type JsonTrace struct {
	BlockNumber         uint64          `json:"blockNumber"`
	BlockHash           common.Hash     `json:"blockHash"`
	TransactionHash     common.Hash     `json:"transactionHash"`
	TransactionPosition uint            `json:"transactionPosition"`
	Subtraces           uint            `json:"subtraces"`
	TraceAddress        []uint          `json:"traceAddress"`
	Typ                 string          `json:"type"`
	Action              json.RawMessage `json:"action"`
	Result              json.RawMessage `json:"result"`
	Err                 string          `json:"error"`
	Revert              string          `json:"revert"`
}

// this function only used by test
func initFromJson(ts *TraceBlockStorage, bytes []byte) {
	var actionObjs []JsonTrace
	var txs []*TxStorage
	var tx *TxStorage
	json.Unmarshal(bytes, &actionObjs)
	for _, obj := range actionObjs {
		ac := action{}
		if len(obj.Err) > 0 {
			ac.err = errors.New(obj.Err)
			ac.revert = utils.FromHex(obj.Revert)
		}
		if obj.Typ == "call" {
			callAc := &JsonCallAction{}
			err := json.Unmarshal(obj.Action, callAc)
			if err != nil {
				panic(err)
			}
			switch callAc.CallType {
			case "staticcall":
				ac.op = vm.STATICCALL
			case "call":
				ac.op = vm.CALL
			case "callcode":
				ac.op = vm.CALLCODE
			case "delegatecall":
				ac.op = vm.DELEGATECALL
			}
			ac.from = callAc.From
			ac.to = callAc.To
			ac.value, _ = new(big.Int).SetString(callAc.Value[2:], 16)
			ac.gas, _ = strconv.ParseUint(callAc.Gas, 0, 64)
			ac.input = utils.FromHex(callAc.Input)

			if ac.err == nil {
				callOutput := &JsonCallOutput{}
				err = json.Unmarshal(obj.Result, callOutput)
				if err != nil {
					panic(err)
				}
				ac.output = utils.FromHex(callOutput.Output)
				ac.gasUsed, err = strconv.ParseUint(callOutput.GasUsed, 0, 64)
				if err != nil {
					panic(err)
				}
			}
		}
		if obj.Typ == "create" {
			ac.op = vm.CREATE
			callAc := &JsonCreateAction{}
			err := json.Unmarshal(obj.Action, callAc)
			if err != nil {
				panic(err)
			}
			callOutput := &JsonCreateOutput{}
			err = json.Unmarshal(obj.Result, callOutput)
			if err != nil {
				panic(err)
			}
			ac.from = callAc.From
			ac.value, _ = new(big.Int).SetString(callAc.Value[2:], 16)
			ac.gas, _ = strconv.ParseUint(callAc.Gas, 0, 64)
			ac.input = utils.FromHex(callAc.Init)

			if ac.err == nil {
				ac.to = callOutput.Address
				ac.output = utils.FromHex(callOutput.Code)
				ac.gasUsed, _ = strconv.ParseUint(callOutput.GasUsed, 0, 64)
			}
		}
		if obj.Typ == "suicide" {
			ac.op = vm.SELFDESTRUCT
			callAc := &JsonSuicideAction{}
			err := json.Unmarshal(obj.Action, callAc)
			if err != nil {
				panic(err)
			}

			ac.from = callAc.Address
			ac.to = callAc.RefundAddress
			ac.value, _ = new(big.Int).SetString(callAc.Balance[2:], 16)
		}
		ts.Hash = obj.BlockHash
		ts.Number = obj.BlockNumber
		if tx == nil || tx.Hash != obj.TransactionHash {
			tx = &TxStorage{
				Hash: obj.TransactionHash,
			}
			txs = append(txs, tx)
		}
		acStorage := ac.toStorage(ts)
		acStorage.Subtraces = obj.Subtraces
		acStorage.TraceAddress = obj.TraceAddress
		tx.Storages = append(tx.Storages, acStorage)
	}
	for _, tx := range txs {
		b, err := rlp.EncodeToBytes(tx)
		if err != nil {
			panic(err)
		}
		ts.TraceStorages = append(ts.TraceStorages, b)
	}
}

func TestStorage(t *testing.T) {
	testJsons := make(map[string]json.RawMessage)

	var sizeDB []byte
	var memDB [][2][]byte
	//TestJsonsMock, _ := os.ReadFile("/tmp/out.json")
	json.Unmarshal(TestJsonsMock, &testJsons)

	for _, testJson := range testJsons {
		block := &TraceBlockStorage{
			addressIndex: make(map[common.Address]int),
			dataIndex:    make(map[common.Hash]int),
		}
		initFromJson(block, testJson)
		jsonRaw, err := block.ToJson()
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(jsonRaw, testJson) {
			t.Fatal("restroe failed!")
		}
		block.ToDB(func(key, data []byte) {
			for _, kv := range memDB {
				if bytes.Equal(kv[0], key) {
					return
				}
			}
			sizeDB = append(sizeDB, key...)
			newKey := sizeDB[len(sizeDB)-len(key):]
			sizeDB = append(sizeDB, data...)
			newData := sizeDB[len(sizeDB)-len(data):]
			memDB = append(memDB, [2][]byte{newKey, newData})
		})
		newBlock := &TraceBlockStorage{
			Hash:         block.Hash,
			addressIndex: make(map[common.Address]int),
			dataIndex:    make(map[common.Hash]int),
		}

		newBlock.FromDB(func(key []byte) ([]byte, error) {
			for _, kv := range memDB {
				k, v := kv[0], kv[1]
				if bytes.Equal(k, key) {
					return v, nil
				}
			}
			t.Fatalf("key not exist: %x", key)
			return nil, nil
		})
		jsonRaw, err = newBlock.ToJson()
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(jsonRaw, testJson) {
			t.Fatal("restroe failed!")
		}
		//os.WriteFile("/tmp/trace.raw", sizeDB, os.ModePerm)
	}
}

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
