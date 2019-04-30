package types

import (
	"bytes"
	"encoding/hex"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"golang.org/x/crypto/sha3"

	"github.com/harmony-one/harmony/internal/ctxerror"
)

// EpochShardState is the shard state of an epoch
type EpochShardState struct {
	Epoch      uint64
	ShardState ShardState
}

// ShardState is the collection of all committees
type ShardState []Committee

// BlsPublicKey defines the bls public key
type BlsPublicKey [96]byte

// Hex returns the hex string of bls public key
func (pk BlsPublicKey) Hex() string {
	return hex.EncodeToString(pk[:])
}

// FromLibBLSPublicKey replaces the key contents with the given key,
func (pk *BlsPublicKey) FromLibBLSPublicKey(key *bls.PublicKey) error {
	bytes := key.Serialize()
	if len(bytes) != len(pk) {
		return ctxerror.New("BLS public key size mismatch",
			"expected", len(pk),
			"actual", len(bytes))
	}
	copy(pk[:], bytes)
	return nil
}

// ToLibBLSPublicKey copies the key contents into the given key.
func (pk *BlsPublicKey) ToLibBLSPublicKey(key *bls.PublicKey) error {
	return key.Deserialize(pk[:])
}

// NodeID represents node id (BLS address).
type NodeID struct {
	EcdsaAddress string
	BlsPublicKey BlsPublicKey
}

// Committee contains the active nodes in one shard
type Committee struct {
	ShardID  uint32
	Leader   NodeID
	NodeList []NodeID
}

// GetHashFromNodeList will sort the list, then use Keccak256 to hash the list
// notice that the input nodeList will be modified (sorted)
func GetHashFromNodeList(nodeList []NodeID) []byte {
	// in general, nodeList should not be empty
	if nodeList == nil || len(nodeList) == 0 {
		return []byte{}
	}

	sort.Slice(nodeList, func(i, j int) bool {
		return CompareNodeID(nodeList[i], nodeList[j]) == -1
	})
	d := sha3.NewLegacyKeccak256()
	for i := range nodeList {
		d.Write(nodeList[i].Serialize())
	}
	return d.Sum(nil)
}

// Hash is the root hash of ShardState
func (ss ShardState) Hash() (h common.Hash) {
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].ShardID < ss[j].ShardID
	})
	d := sha3.NewLegacyKeccak256()
	for i := range ss {
		hash := GetHashFromNodeList(ss[i].NodeList)
		d.Write(hash)
	}
	d.Sum(h[:0])
	return h
}

// CompareNodeID compares two nodes by their ID; used to sort node list
func CompareNodeID(n1 NodeID, n2 NodeID) int {
	return bytes.Compare(n1.BlsPublicKey[:], n2.BlsPublicKey[:])
}

// Serialize serialize NodeID into bytes
func (n NodeID) Serialize() []byte {
	return append(n.BlsPublicKey[:], []byte(n.EcdsaAddress)...)
}
