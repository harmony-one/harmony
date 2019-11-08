package shard

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"golang.org/x/crypto/sha3"
)

var (
	emptyBLSPubKey = BLSPublicKey{}
)

// StakedValidator ..
type StakedValidator struct {
	// nil means not active, 0 means our node, >= 0 means staked node
	WithDelegationApplied *big.Int `json:"with-delegation-applied,omitempty"`
}

// NodeID represents node id (BLS address)
type NodeID struct {
	ECDSAAddress common.Address   `json:"ecdsa_address"`
	BLSPublicKey BLSPublicKey     `json:"bls_pubkey"`
	Validator    *StakedValidator `json:"staked-validator,omitempty"`
}

// NodeIDList is a list of NodeID
type NodeIDList []NodeID

// Committee contains the active nodes in one shard
type Committee struct {
	ShardID  uint32     `json:"shard_id"`
	NodeList NodeIDList `json:"node_list"`
}

// SuperCommittee is the collection of all committees
type SuperCommittee []Committee

// BLSPublicKey defines the bls public key
type BLSPublicKey [48]byte

// Inventory ..
func (members NodeIDList) Inventory() (result struct {
	BLSPublicKeys         [][48]byte    `json:"bls_pubkey"`
	WithDelegationApplied []numeric.Dec `json:"with-delegation-applied,omitempty"`
}) {
	count := len(members)
	result.BLSPublicKeys = make([][48]byte, count)
	result.WithDelegationApplied = make([]numeric.Dec, count)

	for i := 0; i < count; i++ {
		result.BLSPublicKeys[i] = members[i].BLSPublicKey
		if stake := members[i].Validator; stake != nil {
			result.WithDelegationApplied[i] = numeric.NewDecFromBigInt(stake.WithDelegationApplied)
			// result.CurrentlyActive[i] = stake.Active
		}
	}

	return
}

func (ss SuperCommittee) String() string {
	buf := bytes.Buffer{}
	buf.WriteString("[\n\t")
	committee := ss
	for _, subcommittee := range committee {
		for _, committee := range subcommittee.NodeList {
			buf.WriteString(fmt.Sprintf(
				`{"shard-%d":"%s"},\n`,
				subcommittee.ShardID,
				committee.String(),
			))

		}
	}
	buf.WriteString("]\n")
	return buf.String()
}

// FindCommitteeByID returns the committee configuration for the given shard,
// or nil if the given shard is not found.
func (ss SuperCommittee) FindCommitteeByID(shardID uint32) *Committee {
	for _, committee := range ss {
		if committee.ShardID == shardID {
			return &committee
		}
	}
	return nil
}

// DeepCopy returns a deep copy of the receiver.
func (ss SuperCommittee) DeepCopy() SuperCommittee {
	var r SuperCommittee
	for _, c := range ss {
		r = append(r, c.DeepCopy())
	}
	return r
}

// CompareShardSuperCommittee compares two SuperCommittee instances.
func CompareShardSuperCommittee(s1, s2 SuperCommittee) int {
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

// IsEmpty returns whether the bls public key is empty 0 bytes
func (pk BLSPublicKey) IsEmpty() bool {
	return bytes.Compare(pk[:], emptyBLSPubKey[:]) == 0
}

// Hex returns the hex string of bls public key
func (pk BLSPublicKey) Hex() string {
	return hex.EncodeToString(pk[:])
}

// FromLibBLSPublicKey replaces the key contents with the given key,
func (pk *BLSPublicKey) FromLibBLSPublicKey(key *bls.PublicKey) error {
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
func (pk *BLSPublicKey) ToLibBLSPublicKey(key *bls.PublicKey) error {
	return key.Deserialize(pk[:])
}

// CompareBLSPublicKey compares two BLSPublicKey, lexicographically.
func CompareBLSPublicKey(k1, k2 BLSPublicKey) int {
	return bytes.Compare(k1[:], k2[:])
}

// CompareNodeID compares two node IDs.
func CompareNodeID(id1, id2 *NodeID) int {
	if c := bytes.Compare(id1.ECDSAAddress[:], id2.ECDSAAddress[:]); c != 0 {
		return c
	}
	if c := CompareBLSPublicKey(id1.BLSPublicKey, id2.BLSPublicKey); c != 0 {
		return c
	}
	return 0
}

// DeepCopy returns a deep copy of the receiver.
func (members NodeIDList) DeepCopy() NodeIDList {
	return append(members[:0:0], members...)
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
	for _, nodeID := range nodeList {
		d.Write(nodeID.Serialize())
	}
	return d.Sum(nil)
}

// Hash is the root hash of SuperCommittee
func (ss SuperCommittee) Hash() (h common.Hash) {
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
	return bytes.Compare(n1.BLSPublicKey[:], n2.BLSPublicKey[:])
}

// Serialize serialize NodeID into bytes
func (n NodeID) Serialize() []byte {
	return append(n.ECDSAAddress[:], n.BLSPublicKey[:]...)
}

func (n NodeID) String() string {
	return fmt.Sprintf(
		"ECDSA: %s, BLS: %s",
		common2.MustAddressToBech32(n.ECDSAAddress),
		hex.EncodeToString(n.BLSPublicKey[:]),
	)
}
