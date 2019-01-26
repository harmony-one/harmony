package utils

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dedis/kyber"
	eth_crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"
	"github.com/harmony-one/harmony/p2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
)

var (
	lock     sync.Mutex
	key, err = ecdsa.GenerateKey(eth_crypto.S256(), strings.NewReader("Beaconchain Dummy Address"))
	//BeaconChainAddress is available across all packages. Make it constant.
	BeaconChainAddress = eth_crypto.PubkeyToAddress(key.PublicKey)
)

// Unmarshal is a function that unmarshals the data from the
// reader into the specified value.
func Unmarshal(r io.Reader, v interface{}) error {
	return json.NewDecoder(r).Decode(v)
}

// Marshal is a function that marshals the object into an
// io.Reader.
func Marshal(v interface{}) (io.Reader, error) {
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

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

// GenKey generates a key given ip and port.
func GenKey(ip, port string) (kyber.Scalar, kyber.Point) {
	priKey := crypto.Ed25519Curve.Scalar().SetInt64(int64(GetUniqueIDFromIPPort(ip, port))) // TODO: figure out why using a random hash value doesn't work for private key (schnorr)
	pubKey := pki.GetPublicKeyFromScalar(priKey)

	return priKey, pubKey
}

// GenKeyBLS generates a bls key pair given ip and port.
func GenKeyBLS(ip, port string) (*bls.SecretKey, *bls.PublicKey) {
	nodeIDBytes := make([]byte, 32)
	binary.LittleEndian.PutUint32(nodeIDBytes, GetUniqueIDFromIPPort(ip, port))
	privateKey := bls.SecretKey{}
	err := privateKey.SetLittleEndian(nodeIDBytes)
	if err != nil {
		log.Print("failed to set private key", err)
	}
	priKey := &privateKey
	pubKey := privateKey.GetPublicKey()

	return priKey, pubKey
}

// GenKeyP2P generates a pair of RSA keys used in libp2p host
func GenKeyP2P(ip, port string) (p2p_crypto.PrivKey, p2p_crypto.PubKey, error) {
	r := mrand.New(mrand.NewSource(int64(GetUniqueIDFromIPPort(ip, port))))
	return p2p_crypto.GenerateKeyPairWithReader(p2p_crypto.RSA, 2048, r)
}

// GenKeyP2PRand generates a pair of RSA keys used in libp2p host, using random seed
func GenKeyP2PRand() (p2p_crypto.PrivKey, p2p_crypto.PubKey, error) {
	return p2p_crypto.GenerateKeyPair(p2p_crypto.RSA, 2048)
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

// Save saves a representation of v to the file at path.
func Save(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := Marshal(v)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, r)
	return err
}

// Load loads the file at path into v.
func Load(path string, v interface{}) error {
	lock.Lock()
	defer lock.Unlock()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
	}
	defer f.Close()
	return Unmarshal(f, v)
}
