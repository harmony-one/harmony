package shard

import (
	"bytes"
	"encoding/hex"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"golang.org/x/crypto/sha3"

	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
)

// EpochShardState is the shard state of an epoch
type EpochShardState struct {
	Epoch      uint64
	ShardState State
}

// State is the collection of all committees
type State []Committee

// FindCommitteeByID returns the committee configuration for the given shard,
// or nil if the given shard is not found.
func (ss State) FindCommitteeByID(shardID uint32) *Committee {
	for _, committee := range ss {
		if committee.ShardID == shardID {
			return &committee
		}
	}
	return nil
}

// DeepCopy returns a deep copy of the receiver.
func (ss State) DeepCopy() State {
	var r State
	for _, c := range ss {
		r = append(r, c.DeepCopy())
	}
	return r
}

// CompareShardState compares two State instances.
func CompareShardState(s1, s2 State) int {
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
type BlsPublicKey [48]byte

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
	EcdsaAddress common.Address `json:"ecdsa_address"`
	BlsPublicKey BlsPublicKey   `json:"bls_pubkey"`
}

// CompareNodeID compares two node IDs.
func CompareNodeID(id1, id2 *NodeID) int {
	if c := bytes.Compare(id1.EcdsaAddress[:], id2.EcdsaAddress[:]); c != 0 {
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
	ShardID  uint32     `json:"shard_id"`
	NodeList NodeIDList `json:"node_list"`
}

// DeepCopy returns a deep copy of the receiver.
func (c Committee) DeepCopy() Committee {
	r := Committee{}
	r.ShardID = c.ShardID
	r.NodeList = c.NodeList.DeepCopy()
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
// NOTE: do not modify the underlining content for hash
func GetHashFromNodeList(nodeList []NodeID) []byte {
	// in general, nodeList should not be empty
	if nodeList == nil || len(nodeList) == 0 {
		return []byte{}
	}

	d := sha3.NewLegacyKeccak256()
	for i := range nodeList {
		d.Write(nodeList[i].Serialize())
	}
	return d.Sum(nil)
}

// Hash is the root hash of State
func (ss State) Hash() (h common.Hash) {
	// TODO ek – this sorting really doesn't belong here; it should instead
	//  be made an explicit invariant to be maintained and, if needed, checked.
	copy := ss.DeepCopy()
	sort.Slice(copy, func(i, j int) bool {
		return copy[i].ShardID < copy[j].ShardID
	})
	d := sha3.NewLegacyKeccak256()
	for i := range copy {
		hash := GetHashFromNodeList(copy[i].NodeList)
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
	return append(n.EcdsaAddress[:], n.BlsPublicKey[:]...)
}

func (n NodeID) String() string {
	return "ECDSA: " + common2.MustAddressToBech32(n.EcdsaAddress) + ", BLS: " + hex.EncodeToString(n.BlsPublicKey[:])
}
