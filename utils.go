package main

import (
	"bytes"
	"encoding/binary"
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
