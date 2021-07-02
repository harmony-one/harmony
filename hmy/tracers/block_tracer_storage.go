// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tracers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/utils"
)

type ActionStorage struct {
	Subtraces    uint
	TraceAddress []uint
	TraceData    []byte
}

func (storage *ActionStorage) appendByte(byt byte) {
	storage.TraceData = append(storage.TraceData, byt)
}

func (storage *ActionStorage) appendFixed(data []byte) {
	storage.TraceData = append(storage.TraceData, data...)
}
func (storage *ActionStorage) appendNumber(num *big.Int) {
	bytes, _ := rlp.EncodeToBytes(num)
	storage.appendByte(uint8(len(bytes)))
	storage.appendFixed(bytes)
}

func (storage *ActionStorage) readByte() byte {
	val := storage.TraceData[0]
	storage.TraceData = storage.TraceData[1:]
	return val
}
func (storage *ActionStorage) readFixedData(size uint) []byte {
	fixedData := storage.TraceData[:size]
	storage.TraceData = storage.TraceData[size:]
	return fixedData
}
func (storage *ActionStorage) readNumber() *big.Int {
	size := storage.readByte()
	bytes := storage.readFixedData(uint(size))
	var num big.Int
	rlp.DecodeBytes(bytes, &num)
	return &num
}

type TxStorage struct {
	Hash     common.Hash
	Storages []*ActionStorage
}

type TraceBlockStorage struct {
	Hash           common.Hash
	Number         uint64
	AddressTable   []common.Address       // address table
	DataKeyTable   []common.Hash          // data key table
	dataValueTable [][]byte               // data, store in db, avoid RLPEncode
	TraceStorages  [][]byte               // trace data, lenth equal the number of transaction in a block
	addressIndex   map[common.Address]int // address index in AddressTable
	dataIndex      map[common.Hash]int    // data index in DataKeyTable
}

func (ts *TraceBlockStorage) getData(i int) []byte {
	return ts.dataValueTable[i]
}

func (ts *TraceBlockStorage) getAddress(i int) common.Address {
	return ts.AddressTable[i]
}

func (ts *TraceBlockStorage) indexData(data []byte) int {
	key := hash.Keccak256Hash(data)
	if index, exist := ts.dataIndex[key]; exist {
		return index
	}
	index := len(ts.DataKeyTable)
	ts.DataKeyTable = append(ts.DataKeyTable, key)
	ts.dataValueTable = append(ts.dataValueTable, data)
	ts.dataIndex[key] = index
	return index
}

func (ts *TraceBlockStorage) indexAddress(address common.Address) int {
	if index, exist := ts.addressIndex[address]; exist {
		return index
	}
	index := len(ts.addressIndex)
	ts.AddressTable = append(ts.AddressTable, address)
	ts.addressIndex[address] = index
	return index
}

func (ts *TraceBlockStorage) KeyDB() []byte {
	return ts.Hash[:]
}

func (ts *TraceBlockStorage) ToDB(write func([]byte, []byte)) {
	for index, key := range ts.DataKeyTable {
		write(key[:], ts.dataValueTable[index])
	}
	bytes, _ := rlp.EncodeToBytes(ts)
	write(ts.KeyDB(), bytes)
}

func (ts *TraceBlockStorage) FromDB(read func([]byte) ([]byte, error)) error {
	bytes, err := read(ts.KeyDB())
	if err != nil {
		return err
	}
	err = rlp.DecodeBytes(bytes, ts)
	if err != nil {
		return err
	}
	for _, key := range ts.DataKeyTable {
		data, err := read(key[:])
		if err != nil {
			return err
		}
		ts.dataValueTable = append(ts.dataValueTable, data)
	}
	return nil
}

func (ts *TraceBlockStorage) TxJson(index int) ([]json.RawMessage, error) {
	var results []json.RawMessage
	var txStorage TxStorage
	var err error
	b := ts.TraceStorages[index]
	err = rlp.DecodeBytes(b, &txStorage)
	if err != nil {
		return nil, err
	}

	headPiece := fmt.Sprintf(
		`"blockNumber":%d,"blockHash":"%s","transactionHash":"%s","transactionPosition":%d`,
		ts.Number, ts.Hash.Hex(), txStorage.Hash.Hex(), index,
	)

	for _, acStorage := range txStorage.Storages {
		ac := &action{}
		ac.fromStorage(ts, acStorage)

		typStr, acStr, outStr := ac.toJsonStr()
		if acStr == nil {
			err = errors.New("tracer internal failure")
			return nil, err
		}
		traceStr, _ := json.Marshal(acStorage.TraceAddress)
		bodyPiece := fmt.Sprintf(
			`,"subtraces":%d,"traceAddress":%s,"type":"%s","action":%s`,
			acStorage.Subtraces, string(traceStr), typStr, *acStr,
		)
		var resultPiece string
		if ac.err != nil {
			resultPiece = fmt.Sprintf(`,"error":"Reverted","revert":"0x%x"`, ac.revert)
		} else if outStr != nil {
			resultPiece = fmt.Sprintf(`,"result":%s`, *outStr)
		} else {
			resultPiece = `,"result":null`
		}
		jstr := "{" + headPiece + bodyPiece + resultPiece + "}"
		results = append(results, json.RawMessage(jstr))
	}
	return results, nil
}

func (ts *TraceBlockStorage) ToJson() (json.RawMessage, error) {
	var results []json.RawMessage
	for i := range ts.TraceStorages {
		tx, err := ts.TxJson(i)
		if err != nil {
			return nil, err
		}
		results = append(results, tx...)
	}
	return json.Marshal(results)
}

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

func (ts *TraceBlockStorage) fromJson(bytes []byte) {
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
