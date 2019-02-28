package utils

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"os"
	"regexp"
	"strconv"
	"sync"

	crypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/internal/utils/contract"
	"github.com/harmony-one/harmony/p2p"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
)

var lock sync.Mutex

// PrivKeyStore is used to persist private key to/from file
type PrivKeyStore struct {
	Key string `json:"key"`
}

func init() {
	bls.Init(bls.BLS12_381)
}

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

// GenKey generates a bls key pair given ip and port.
func GenKey(ip, port string) (*bls.SecretKey, *bls.PublicKey) {
	nodeIDBytes := make([]byte, 32)
	binary.LittleEndian.PutUint32(nodeIDBytes, GetUniqueIDFromIPPort(ip, port))
	privateKey := bls.SecretKey{}
	err := privateKey.SetLittleEndian(nodeIDBytes)
	if err != nil {
		log.Print("failed to set private key", err)
		return nil, nil
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

// LoadPrivateKey parses the key string in base64 format and return PrivKey
func LoadPrivateKey(key string) (p2p_crypto.PrivKey, p2p_crypto.PubKey, error) {
	if key != "" {
		k1, err := p2p_crypto.ConfigDecodeKey(key)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode key: %v", err)
		}
		priKey, err := p2p_crypto.UnmarshalPrivateKey(k1)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal private key: %v", err)
		}
		pubKey := priKey.GetPublic()
		return priKey, pubKey, nil
	}
	return nil, nil, fmt.Errorf("empty key string")
}

// SavePrivateKey convert the PrivKey to base64 format and return string
func SavePrivateKey(key p2p_crypto.PrivKey) (string, error) {
	if key != nil {
		b, err := p2p_crypto.MarshalPrivateKey(key)
		if err != nil {
			return "", fmt.Errorf("failed to marshal private key: %v", err)
		}
		str := p2p_crypto.ConfigEncodeKey(b)
		return str, nil
	}
	return "", fmt.Errorf("key is nil")
}

// SaveKeyToFile save private key to keyfile
func SaveKeyToFile(keyfile string, key p2p_crypto.PrivKey) (err error) {
	str, err := SavePrivateKey(key)
	if err != nil {
		return
	}

	keyStruct := PrivKeyStore{Key: str}

	err = Save(keyfile, &keyStruct)
	return
}

// LoadKeyFromFile load private key from keyfile
// If the private key is not loadable or no file, it will generate
// a new random private key
func LoadKeyFromFile(keyfile string) (key p2p_crypto.PrivKey, pk p2p_crypto.PubKey, err error) {
	var keyStruct PrivKeyStore
	err = Load(keyfile, &keyStruct)
	if err != nil {
		log.Print("No priviate key can be loaded from file", "keyfile", keyfile)
		log.Print("Using random private key")
		key, pk, err = GenKeyP2PRand()
		if err != nil {
			log.Panic("LoadKeyFromFile", "GenKeyP2PRand Error", err)
		}
		err = SaveKeyToFile(keyfile, key)
		if err != nil {
			log.Print("LoadKeyFromFile", "failed to save key to keyfile", err)
		}
		return key, pk, nil
	}
	key, pk, err = LoadPrivateKey(keyStruct.Key)
	return key, pk, err
}

// LoadStakingKeyFromFile load staking private key from keyfile
// If the private key is not loadable or no file, it will generate
// a new random private key
// Currently for deploy_newnode.sh, we hard-coded the first fake account as staking account and
// it is minted in genesis block. See genesis_node.go
func LoadStakingKeyFromFile(keyfile string) *ecdsa.PrivateKey {
	// contract.FakeAccounts[0] gets minted tokens in genesis block of beacon chain.
	if key, err := crypto.HexToECDSA(contract.FakeAccounts[0].Private); err != nil {
		GetLogInstance().Error("Unable to get staking key")
		os.Exit(1)
	}
	if err := crypto.SaveECDSA(keyfile, key); err != nil {
		GetLogInstance().Error("Unable to save the private key", "error", err)
		os.Exit(1)
	}
	// TODO(minhdoan): Enable this back.
	// key, err := crypto.LoadECDSA(keyfile)
	// if err != nil {
	// 	GetLogInstance().Error("no key file. Let's create a staking private key")
	// 	key, err = crypto.GenerateKey()
	// 	if err != nil {
	// 		GetLogInstance().Error("Unable to generate the private key")
	// 		os.Exit(1)
	// 	}
	// 	if err = crypto.SaveECDSA(keyfile, key); err != nil {
	// 		GetLogInstance().Error("Unable to save the private key", "error", err)
	// 		os.Exit(1)
	// 	}
	// }
	return key
}
