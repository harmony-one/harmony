package db

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/rlp"
)

type Memorybaseddb struct {
	leafLength int64
	nodes      map[int64][]byte
}

func NewMemorybaseddb(leafLength int64, nodes map[int64][]byte) *Memorybaseddb {
	db := Memorybaseddb{nodes: nodes, leafLength: leafLength}
	return &db
}
func (db *Memorybaseddb) Get(index int64) ([]byte, bool) {
	value, ok := db.nodes[index]
	return value, ok
}
func (db *Memorybaseddb) Set(value []byte, index int64) {
	//require correct wordsize
	db.nodes[index] = value
}
func (db *Memorybaseddb) GetLeafLength() int64 {
	return db.leafLength
}
func (db *Memorybaseddb) SetLeafLength(value int64) {
	db.leafLength = value
}

type Thing struct {
	length [][]byte
	nodes  [][][]byte
}

func FromSerialized(input []byte) *Memorybaseddb { //likely a much easier way to do this
	output := []interface{}{}
	err := rlp.DecodeBytes(input, &output)

	if err != nil {
		fmt.Print("RRRRRR ", err)
		panic(errors.New("Problem rlp decoding db node and length data"))
	}
	if len(output) != 2 {
		panic(errors.New("RLP formatting error. Outer tuple should be `[leafLength,nodes]`"))
	}
	leafLengthArr, leafLengthOk := output[0].([]byte)
	if !leafLengthOk {
		panic(errors.New("RLP formatting error. `leafLength` unable to cast to `[]byte`"))
	}
	leafLengthBuffer := make([]byte, 8-len(leafLengthArr))
	leafLengthBuffer = append(leafLengthBuffer, leafLengthArr...)
	leafLength := int64(binary.BigEndian.Uint64(leafLengthBuffer))
	nodesArr, nodesArrayOk := output[1].([]interface{})
	if !nodesArrayOk {
		panic(errors.New("RLP formatting error. `nodesArray` unable to cast to `[][][]byte`"))
	}
	nodes := map[int64][]byte{}

	for _, _keyPair := range nodesArr {
		keyPair, _ := _keyPair.([]interface{})
		unpaddedKeyBuffer := keyPair[0].([]byte)
		keyBuffer := make([]byte, 8-len(unpaddedKeyBuffer))
		keyBuffer = append(keyBuffer, unpaddedKeyBuffer...)
		k := int64(binary.BigEndian.Uint64(keyBuffer))
		nodes[k] = keyPair[1].([]byte)
	}

	db := Memorybaseddb{nodes: nodes, leafLength: leafLength}
	return &db
}
func (db *Memorybaseddb) Serialize() []byte {
	output := []interface{}{}

	leafLengthBytes, err := rlp.EncodeToBytes(uint(db.GetLeafLength()))
	if err != nil {
		panic(errors.New("problem representing leafLength in []byte"))
	}

	nodes := [][][]byte{}
	for nodeIndex, nodeValue := range db.nodes {
		k, _ := rlp.EncodeToBytes(uint(nodeIndex))
		kv := [][]byte{}
		kv = append(kv, k)
		kv = append(kv, nodeValue)
		nodes = append(nodes, kv)
	}

	output = append(output, leafLengthBytes)
	output = append(output, nodes)

	serializedOutput, err := rlp.EncodeToBytes(output)
	if err != nil {
		panic(errors.New("Problem rlp encoding db (nodes and leafLength)"))
	}

	return serializedOutput
}

func (db *Memorybaseddb) Close() {}
