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

// GetUniqueIDFromPeer ...
// TODO(minhdoan): this is probably a hack, probably needs some strong non-collision hash.
func GetUniqueIDFromPeer(peer p2p.Peer) uint32 {
	return GetUniqueIDFromIPPort(peer.IP, peer.Port)
}

// GetUniqueIDFromIPPort --
func GetUniqueIDFromIPPort(ip, port string) uint32 {
	reg, err := regexp.Compile("[^0-9]+")
	if err != nil {
		log.Panic("Regex Compilation Failed", "err", err)
	}
	socketID := reg.ReplaceAllString(ip+port, "") // A integer Id formed by unique IP/PORT pair
	value, _ := strconv.Atoi(socketID)
	return uint32(value)
}

// RunCmd runs command `name` with arguments `args`
func RunCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	stderrBytes := &bytes.Buffer{}
	cmd.Stderr = stderrBytes
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
		return err
	}

	log.Println("Command running", name, args)
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Command finished with error: %v", err)
			log.Printf("Stderr: %v", string(stderrBytes.Bytes()))
		} else {
			log.Printf("Command finished successfully")
		}
	}()
	return nil
}

// GenKey generates a key given ip and port.
func GenKey(ip, port string) (kyber.Scalar, kyber.Point) {
	priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(GetUniqueIDFromIPPort(ip, port))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
	pubKey := pki.GetPublicKeyFromScalar(priKey)

	return priKey, pubKey
}

// AllocateShard uses the number of current nodes and number of shards
// to return the shardNum a new node belongs to, it also tells whether the node is a leader
func AllocateShard(numOfAddedNodes, numOfShards int) (int, bool) {
	if numOfShards == 1 {
		if numOfAddedNodes == 1 {
			return 1, true
		}
		return 1, false
	}
	if numOfAddedNodes > numOfShards {
		shardNum := numOfAddedNodes % numOfShards
		if shardNum == 0 {
			return numOfShards, false
		}
		return shardNum, false
	}
	return numOfAddedNodes, true
}
