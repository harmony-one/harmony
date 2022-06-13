/*
This uses a standard file. The entire file is treated as an array where
nodes are stored as byte arrays of length `wordsize` elements and `get/set` operations
are done by reading random accessed data at their `index` multiplied by the wordsize
plus one wordsize because the first slot holds the wordSize and then the leafLength data.
Specifically first 8 bytes are wordsize data (index 0), next 8 bytes are leafLength
data (index 8) then the zeroith leaf element is at index `wordsize`
*/

package db

import (
	"encoding/binary"
	"errors"
	"os"
)

const WORD_SIZE_OFFSET = 0
const LEAF_LENGTH_OFFSET = 8

// The other 8 you see everywhere is just the byte size of an int64

type Filebaseddb struct {
	fd               *os.File
	cachedLeafLength int64
	cachedWordSize   int64
}

func OpenFilebaseddb(path string) *Filebaseddb {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	db := Filebaseddb{fd: file}

	readBuffer := make([]byte, 8)
	wordSize, err := db.fd.ReadAt(readBuffer, WORD_SIZE_OFFSET)

	if wordSize == 0 || err != nil {
		db.Close()
		return nil
	}

	return &db
}

func CreateFilebaseddb(path string, wordSize int64) *Filebaseddb {
	file, err := os.Create(path)
	if err != nil {
		panic(errors.New("Error creating db"))
	}

	db := Filebaseddb{fd: file}

	db.SetWordSize(wordSize)
	db.SetLeafLength(0)

	return &db
}

func (db *Filebaseddb) Get(index int64) ([]byte, bool) {
	wordSize := db.GetWordSize()
	value := make([]byte, wordSize)
	n, err := db.fd.ReadAt(value, wordSize*int64(index+1))

	ok := true
	if err != nil || n == 0 {
		ok = false
	}

	return value, ok

}
func (db *Filebaseddb) Set(value []byte, index int64) {
	wordSize := db.GetWordSize()
	if int64(len(value)) != wordSize || index < 0 {
		panic(errors.New("Argument error to Filebaseddb"))
	}

	n, err := db.fd.WriteAt(value, wordSize*(index+1))

	if err != nil || n == 0 {
		panic(errors.New("Error writing to Filebaseddb"))
	}
}
func (db *Filebaseddb) GetLeafLength() int64 {
	if db.cachedLeafLength == 0 {
		leafLengthBuffer := make([]byte, 8)
		n, err := db.fd.ReadAt(leafLengthBuffer, int64(LEAF_LENGTH_OFFSET))

		if err != nil || n == 0 {
			panic(errors.New("Error reading leafLength of filebaseddb"))
		}
		db.cachedLeafLength = int64(binary.BigEndian.Uint64(leafLengthBuffer))
	}
	return db.cachedLeafLength
}
func (db *Filebaseddb) SetLeafLength(leafLength int64) {
	leafLengthBuffer := make([]byte, db.GetWordSize()-8)
	binary.BigEndian.PutUint64(leafLengthBuffer, uint64(leafLength))

	n, err := db.fd.WriteAt(leafLengthBuffer, int64(LEAF_LENGTH_OFFSET))

	if err != nil || n == 0 {
		panic(errors.New("Error writing to length to Filebaseddb"))
	}

	db.cachedLeafLength = leafLength

	// save/flush only after length data is updated
	// this makes appending 200x slower. Is there a way around it?
	db.fd.Sync()
}

func (db *Filebaseddb) GetWordSize() int64 {
	if db.cachedWordSize == 0 {
		wordSizeBuffer := make([]byte, 8)
		n, err := db.fd.ReadAt(wordSizeBuffer, int64(WORD_SIZE_OFFSET))

		if err != nil || n == 0 {
			panic(errors.New("Error reading wordsize of filebaseddb"))
		}

		db.cachedWordSize = int64(binary.BigEndian.Uint64(wordSizeBuffer))
	}
	return db.cachedWordSize
}
func (db *Filebaseddb) SetWordSize(wordSize int64) {
	wordSizeBuffer := make([]byte, 8)
	binary.BigEndian.PutUint64(wordSizeBuffer, uint64(wordSize))

	n, err := db.fd.WriteAt(wordSizeBuffer, int64(WORD_SIZE_OFFSET))

	if err != nil || n == 0 || wordSize < 16 {
		panic(errors.New("Error initiallizing wordSize to Filebaseddb"))
	}

	db.cachedWordSize = wordSize // cache wordSize property
}

// This function is not recomended to run on a file based db because
// it iterates every single node, and file based dbs have
// the entire dataset as opposed to a proof-based db (sparse dataset)
func (db *Filebaseddb) Serialize() []byte {
	nodes := db.getNodes()
	memDb := NewMemorybaseddb(db.GetLeafLength(), nodes)
	return memDb.Serialize()
}

func (db *Filebaseddb) getNodes() map[int64][]byte {
	db.fd.Sync()
	wordSize := db.GetWordSize()
	stat, err := db.fd.Stat()
	if err != nil {
		panic(errors.New("Error reading filebased db stat"))
	}
	fileSize := stat.Size() //todo: test this
	nodeLength := (fileSize - wordSize) / wordSize
	nodes := map[int64][]byte{}
	for i := int64(0); i < nodeLength; i++ {
		nodeValue, ok := db.Get(i)
		if !ok {
			panic(errors.New("Error reading filebased db node index " + string(i)))
		}
		nodes[nodeLength] = nodeValue
	}
	return nodes
}

func (db *Filebaseddb) Close() {
	db.fd.Close()
}
