package main

import (
    "bytes"
    "encoding/binary"
    "encoding/gob"
    "log"
    "strconv"
    "strings"
)

// IntToHex converts an int64 to a byte array
func IntToHex(num int64) []byte {
    buff := new(bytes.Buffer)
    err := binary.Write(buff, binary.BigEndian, num)
    if err != nil {
        log.Panic(err)
    }

    return buff.Bytes()
}

// Serialize is to serialize a block into []byte.
func (b *Block) Serialize() []byte {
    var result bytes.Buffer
    encoder := gob.NewEncoder(&result)

    err := encoder.Encode(b)

    return result.Bytes()
}

// DeserializeBlock is to deserialize []byte into a Block.
func DeserializeBlock(d []byte) *Block {
    var block Block

    decoder := gob.NewDecoder(bytes.NewReader(d))
    err := decoder.Decode(&block)

    return &block
}

// Helper library to convert '1,2,3,4' into []int{1,2,3,4}.
func ConvertIntoInts(data string) []int {
    var res = []int{}
    items := strings.Split(data, " ")
    for _, value := range items {
        intValue, err := strconv.Atoi(value)
        checkError(err)
        res = append(res, intValue)
    }
    return res
}

// Helper library to convert '1,2,3,4' into []int{1,2,3,4}.
func ConvertIntoMap(data string) map[string]int {
    var res = map[string]int
    items := strings.Split(data, ",")
    for _, value := range items {
        pair := strings.Split(value, " ")
        if len(pair) == 3 {
            intValue, err := strconv.Atoi(pair[2])
            if err != nil {
                pair[0] = strings.Trim(pair[0])
                res[pair[0]] = intValue
            }
        }
    }
    return res
}
