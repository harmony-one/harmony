package utils

import (
	"bytes"
	"encoding/binary"
	"log"
	"regexp"
	"strconv"

	"github.com/simple-rules/harmony-benchmark/p2p"
)

// ConvertFixedDataIntoByteArray converts an empty interface data to a byte array
func ConvertFixedDataIntoByteArray(data interface{}) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func GetUniqueIdFromPeer(peer p2p.Peer) uint16 {
	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		log.Panic("Regex Compilation Failed", "err", err)
	}
	socketId := reg.ReplaceAllString(peer.Ip+peer.Port, "") // A integer Id formed by unique IP/PORT pair
	value, _ := strconv.Atoi(socketId)
	return uint16(value)
}
