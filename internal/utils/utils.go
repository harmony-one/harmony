package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"os"
	"regexp"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	bls2 "github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	p2p_crypto "github.com/libp2p/go-libp2p-crypto"
)

var lock sync.Mutex
var privateNets []*net.IPNet

// PrivKeyStore is used to persist private key to/from file
type PrivKeyStore struct {
	Key string `json:"key"`
}

func init() {
	bls.Init(bls.BLS12_381)

	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
	} {
		_, block, _ := net.ParseCIDR(cidr)
		privateNets = append(privateNets, block)
	}
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
		Logger().Error().
			AnErr("err", err).
			Msg("Failed to convert fixed data into byte array")
	}
	return buff.Bytes()
}

// GetUniqueIDFromIPPort --
func GetUniqueIDFromIPPort(ip, port string) uint32 {
	reg, _ := regexp.Compile("[^0-9]+")
	socketID := reg.ReplaceAllString(ip+port, "") // A integer Id formed by unique IP/PORT pair
	value, _ := strconv.Atoi(socketID)
	return uint32(value)
}

// GetAddressFromBLSPubKey return the address object from bls pub key.
func GetAddressFromBLSPubKey(pubKey *bls.PublicKey) common.Address {
	addr := common.Address{}
	addrBytes := pubKey.GetAddress()
	addr.SetBytes(addrBytes[:])
	return addr
}

// GetAddressFromBLSPubKeyBytes return the address object from bls pub key.
func GetAddressFromBLSPubKeyBytes(pubKeyBytes []byte) common.Address {
	pubKey, err := bls2.BytesToBLSPublicKey(pubKeyBytes[:])
	addr := common.Address{}
	if err == nil {
		addrBytes := pubKey.GetAddress()
		addr.SetBytes(addrBytes[:])
	} else {
		Logger().Err(err).Msg("Failed to get address of bls key")
	}
	return addr
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
		Logger().Info().
			Str("keyfile", keyfile).
			Msg("No private key can be loaded from file")
		Logger().Info().Msg("Using random private key")
		key, pk, err = GenKeyP2PRand()
		if err != nil {
			Logger().Error().
				AnErr("GenKeyP2PRand Error", err).
				Msg("LoadedKeyFromFile")
			panic(err)
		}
		err = SaveKeyToFile(keyfile, key)
		if err != nil {
			Logger().Error().
				AnErr("keyfile", err).
				Msg("failed to save key to keyfile")
		}
		return key, pk, nil
	}
	key, pk, err = LoadPrivateKey(keyStruct.Key)
	return key, pk, err
}

// IsPrivateIP checks if an IP address is private or not
func IsPrivateIP(ip net.IP) bool {
	for _, block := range privateNets {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// GetPortFromDiff returns the port from base and the diff.
func GetPortFromDiff(port string, diff int) string {
	if portNum, err := strconv.Atoi(port); err == nil {
		return fmt.Sprintf("%d", portNum-diff)
	}
	GetLogInstance().Error("error on parsing port.")
	return ""
}

// GetPendingCXKey creates pending CXReceiptsProof key given shardID and blockNum
// it is to avoid adding duplicated CXReceiptsProof from the same source shard
func GetPendingCXKey(shardID uint32, blockNum uint64) string {
	key := strconv.FormatUint(uint64(shardID), 10) + "-" + strconv.FormatUint(blockNum, 10)
	return key
}

// AppendIfMissing appends an item if it's missing in the slice, returns appended slice and true
// Otherwise, return the original slice and false
func AppendIfMissing(slice []common.Address, addr common.Address) ([]common.Address, bool) {
	for _, ele := range slice {
		if ele == addr {
			return slice, false
		}
	}
	return append(slice, addr), true
}

// PrintError prints the given error in the extended format (%+v) onto stderr.
func PrintError(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
}

// FatalError prints the given error in the extended format (%+v) onto stderr,
// then exits with status 1.
func FatalError(err error) {
	PrintError(err)
	os.Exit(1)
}

// PrintErrMsg prints the given error wrapped with the given message in the
// extended format (%+v) onto stderr.
func PrintErrMsg(err error, format string, args ...interface{}) {
	PrintError(errors.WithMessagef(err, format, args...))
}

// FatalErrMsg prints the given error wrapped with the given message in the
// extended format (%+v) onto stderr, then exits with status 1.
func FatalErrMsg(err error, format string, args ...interface{}) {
	FatalError(errors.WithMessagef(err, format, args...))
}
