package utils

import (
	"bytes"
	"encoding/binary"
	"log"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/dedis/kyber"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
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

func GetUniqueIdFromIpPort(ip, port string) uint16 {
	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		log.Panic("Regex Compilation Failed", "err", err)
	}
	socketId := reg.ReplaceAllString(ip+port, "") // A integer Id formed by unique IP/PORT pair
	value, _ := strconv.Atoi(socketId)
	return uint16(value)
}

// RunCmd Runs command `name` with arguments `args`
func RunCmd(name string, args ...string) error {
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

//Takes ip and port to generate a public and private key
func GenKey(ip, port string) (kyber.Scalar, kyber.Point) {
	priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(GetUniqueIdFromIpPort(ip, port))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
	pubKey := pki.GetPublicKeyFromScalar(priKey)

	return priKey, pubKey
}

func AllocateShard(numnode, numshards int) (int, bool) {
	if numnode <= numshards {
		return numnode, true
	} else {
		shardnum := numnode % numshards
		if shardnum == 0 {
			return numshards, false
		} else {
			return shardnum, false
		}

	}
}
