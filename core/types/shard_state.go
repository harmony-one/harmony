package types

import (
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

type NodeRole byte

const (
	Leader NodeRole = iota
	Validator
)

type NodeInfo struct {
	NodeID string
	Role   NodeRole
}

// ShardState is the collection of all committees
type ShardState []Committee

// Committee contains the active nodes in one shard
type Committee struct {
	ShardID  uint32
	NodeList []NodeInfo
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
	if n1 < n2 {
		return -1
	}
	if n1 > n2 {
		return 1
	}
	return 0
}

// Serialize serialize NodeID into bytes
func (n NodeID) Serialize() []byte {
	return []byte(n)
}
