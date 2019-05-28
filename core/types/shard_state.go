package types

import (
	"bytes"
	"encoding/hex"
	"sort"
	"strings"

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

// FindCommitteeByID returns the committee configuration for the given shard,
// or nil if the given shard is not found.
func (ss ShardState) FindCommitteeByID(shardID uint32) *Committee {
	for _, committee := range ss {
		if committee.ShardID == shardID {
			return &committee
		}
	}
	return nil
}

// DeepCopy returns a deep copy of the receiver.
func (ss ShardState) DeepCopy() ShardState {
	var r ShardState
	for _, c := range ss {
		r = append(r, c.DeepCopy())
	}
	return r
}

// CompareShardState compares two ShardState instances.
func CompareShardState(s1, s2 ShardState) int {
	commonLen := len(s1)
	if commonLen > len(s2) {
		commonLen = len(s2)
	}
	for idx := 0; idx < commonLen; idx++ {
		if c := CompareCommittee(&s1[idx], &s2[idx]); c != 0 {
			return c
		}
	}
	switch {
	case len(s1) < len(s2):
		return -1
	case len(s1) > len(s2):
		return +1
	}
	return 0
}

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

// CompareBlsPublicKey compares two BlsPublicKey, lexicographically.
func CompareBlsPublicKey(k1, k2 BlsPublicKey) int {
	return bytes.Compare(k1[:], k2[:])
}

// NodeID represents node id (BLS address).
type NodeID struct {
	EcdsaAddress string
	BlsPublicKey BlsPublicKey
}

// CompareNodeID compares two node IDs.
func CompareNodeID(id1, id2 *NodeID) int {
	if c := strings.Compare(id1.EcdsaAddress, id2.EcdsaAddress); c != 0 {
		return c
	}
	if c := CompareBlsPublicKey(id1.BlsPublicKey, id2.BlsPublicKey); c != 0 {
		return c
	}
	return 0
}

// NodeIDList is a list of NodeIDList.
type NodeIDList []NodeID

// DeepCopy returns a deep copy of the receiver.
func (l NodeIDList) DeepCopy() NodeIDList {
	return append(l[:0:0], l...)
}

// CompareNodeIDList compares two node ID lists.
func CompareNodeIDList(l1, l2 NodeIDList) int {
	commonLen := len(l1)
	if commonLen > len(l2) {
		commonLen = len(l2)
	}
	for idx := 0; idx < commonLen; idx++ {
		if c := CompareNodeID(&l1[idx], &l2[idx]); c != 0 {
			return c
		}
	}
	switch {
	case len(l1) < len(l2):
		return -1
	case len(l1) > len(l2):
		return +1
	}
	return 0
}

// Committee contains the active nodes in one shard
type Committee struct {
	ShardID  uint32
	NodeList NodeIDList
}

// DeepCopy returns a deep copy of the receiver.
func (c Committee) DeepCopy() Committee {
	r := c
	r.NodeList = r.NodeList.DeepCopy()
	return r
}

// CompareCommittee compares two committees and their leader/node list.
func CompareCommittee(c1, c2 *Committee) int {
	switch {
	case c1.ShardID < c2.ShardID:
		return -1
	case c1.ShardID > c2.ShardID:
		return +1
	}
	if c := CompareNodeIDList(c1.NodeList, c2.NodeList); c != 0 {
		return c
	}
	return 0
}

// GetHashFromNodeList will sort the list, then use Keccak256 to hash the list
// notice that the input nodeList will be modified (sorted)
func GetHashFromNodeList(nodeList []NodeID) []byte {
	// in general, nodeList should not be empty
	if nodeList == nil || len(nodeList) == 0 {
		return []byte{}
	}

	sort.Slice(nodeList, func(i, j int) bool {
		return CompareNodeIDByBLSKey(nodeList[i], nodeList[j]) == -1
	})
	d := sha3.NewLegacyKeccak256()
	for i := range nodeList {
		d.Write(nodeList[i].Serialize())
	}
	return d.Sum(nil)
}

// Hash is the root hash of ShardState
func (ss ShardState) Hash() (h common.Hash) {
	// TODO ek – this sorting really doesn't belong here; it should instead
	//  be made an explicit invariant to be maintained and, if needed, checked.
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

// CompareNodeIDByBLSKey compares two nodes by their ID; used to sort node list
func CompareNodeIDByBLSKey(n1 NodeID, n2 NodeID) int {
	return bytes.Compare(n1.BlsPublicKey[:], n2.BlsPublicKey[:])
}

// Serialize serialize NodeID into bytes
func (n NodeID) Serialize() []byte {
	return append(n.BlsPublicKey[:], []byte(n.EcdsaAddress)...)
}

func (n NodeID) String() string {
	return "ECDSA: " + n.EcdsaAddress + ", BLS: " + hex.EncodeToString(n.BlsPublicKey[:])
}
