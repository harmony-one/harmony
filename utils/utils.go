package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
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

// TODO(minhdoan): this is probably a hack, probably needs some strong non-collision hash.
func GetUniqueIdFromPeer(peer p2p.Peer) uint16 {
	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		log.Panic("Regex Compilation Failed", "err", err)
	}
	socketId := reg.ReplaceAllString(peer.Ip+peer.Port, "") // A integer Id formed by unique IP/PORT pair
	value, _ := strconv.Atoi(socketId)
	return uint16(value)
}

// RunCmd Runs command `name` with arguments `args`
func RunCmd(name string, args ...string) error {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return errors.New(fmt.Sprintf("file: %v doesn't exist", name))
	}
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return err
	}

	log.Println("Command running", name, args)
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Command finished with error: %v", err)
		} else {
			log.Printf("Command finished successfully")
		}
	}()
	return nil
}
