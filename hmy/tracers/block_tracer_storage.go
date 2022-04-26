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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/crypto/hash"
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
	TraceStorages  [][]byte               // trace data, length equal the number of transaction in a block
	addressIndex   map[common.Address]int // address index in AddressTable
	dataIndex      map[common.Hash]int    // data index in DataKeyTable
}

// get data of index i
func (ts *TraceBlockStorage) getData(i int) []byte {
	return ts.dataValueTable[i]
}

// get address of index i
func (ts *TraceBlockStorage) getAddress(i int) common.Address {
	return ts.AddressTable[i]
}

// store data and assign an index to it. if data existed, just return it's index
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

// store address and assign an index to it. if address existed, just return it's index
func (ts *TraceBlockStorage) indexAddress(address common.Address) int {
	if index, exist := ts.addressIndex[address]; exist {
		return index
	}
	index := len(ts.addressIndex)
	ts.AddressTable = append(ts.AddressTable, address)
	ts.addressIndex[address] = index
	return index
}

// use this key as
func (ts *TraceBlockStorage) KeyDB() []byte {
	return ts.Hash[:]
}

// store TraceBlockStorage to db
func (ts *TraceBlockStorage) ToDB(write func([]byte, []byte)) {
	for index, key := range ts.DataKeyTable {
		write(key[:], ts.dataValueTable[index])
	}
	bytes, _ := rlp.EncodeToBytes(ts)
	write(ts.KeyDB(), bytes)
}

// load TraceBlockStorage from db
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

// return trace result of a tx for giving index
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

// return trace result of a block
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
