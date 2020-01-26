package shard

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"golang.org/x/crypto/sha3"
)

var (
	emptyBlsPubKey = BlsPublicKey{}
)

// PublicKeySizeInBytes ..
const (
	PublicKeySizeInBytes    = 48
	BLSSignatureSizeInBytes = 96
)

// State is the collection of all committees
type State struct {
	Epoch  *big.Int    `json:"epoch"`
	Shards []Committee `json:"shards"`
}

// BlsPublicKey defines the bls public key
type BlsPublicKey [PublicKeySizeInBytes]byte

// BLSSignature defines the bls signature
type BLSSignature [BLSSignatureSizeInBytes]byte

// Slot represents node id (BLS address)
type Slot struct {
	EcdsaAddress common.Address `json:"ecdsa-address"`
	BlsPublicKey BlsPublicKey   `json:"bls-pubkey"`
	// nil means our node, 0 means not active, > 0 means staked node
	EffectiveStake *numeric.Dec `json:"effective-stake" rlp:"nil"`
}

// SlotList is a list of Slot.
type SlotList []Slot

// Committee contains the active nodes in one shard
type Committee struct {
	ShardID uint32   `json:"shard-id"`
	Slots   SlotList `json:"subcommittee"`
}

/* Legacy
These are the pre-staking used data-structures, needed to maintain
compatibilty for RLP decode/encode
*/

// StateLegacy ..
type StateLegacy []CommitteeLegacy

// SlotLegacy represents node id (BLS address)
type SlotLegacy struct {
	EcdsaAddress common.Address `json:"ecdsa-address"`
	BlsPublicKey BlsPublicKey   `json:"bls-pubkey"`
}

// SlotListLegacy is a list of SlotList.
type SlotListLegacy []SlotLegacy

// CommitteeLegacy contains the active nodes in one shard
type CommitteeLegacy struct {
	ShardID uint32         `json:"shard-id"`
	Slots   SlotListLegacy `json:"subcommittee"`
}

// DecodeWrapper ..
func DecodeWrapper(shardState []byte) (*State, error) {
	oldSS := StateLegacy{}
	newSS := State{}
	var (
		err1 error
		err2 error
	)
	err1 = rlp.DecodeBytes(shardState, &newSS)
	if err1 == nil {
		return &newSS, nil
	}
	err2 = rlp.DecodeBytes(shardState, &oldSS)
	if err2 == nil {
		newSS := State{}
		newSS.Shards = make([]Committee, len(oldSS))
		for i := range oldSS {
			newSS.Shards[i] = Committee{ShardID: oldSS[i].ShardID, Slots: SlotList{}}
			for _, slot := range oldSS[i].Slots {
				newSS.Shards[i].Slots = append(newSS.Shards[i].Slots, Slot{
					slot.EcdsaAddress, slot.BlsPublicKey, nil,
				})
			}
		}
		newSS.Epoch = nil // Make sure for legacy state, the epoch is nil
		return &newSS, nil
	}
	return nil, err2
}

// EncodeWrapper ..
func EncodeWrapper(shardState State, isStaking bool) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if isStaking {
		data, err = rlp.EncodeToBytes(shardState)
	} else {
		shardStateLegacy := make(StateLegacy, len(shardState.Shards))
		for i := range shardState.Shards {
			shardStateLegacy[i] = CommitteeLegacy{
				ShardID: shardState.Shards[i].ShardID, Slots: SlotListLegacy{},
			}
			for _, slot := range shardState.Shards[i].Slots {
				shardStateLegacy[i].Slots = append(shardStateLegacy[i].Slots, SlotLegacy{
					slot.EcdsaAddress, slot.BlsPublicKey,
				})
			}
		}

		data, err = rlp.EncodeToBytes(shardStateLegacy)
	}

	return data, err
}

// ExternalValidators returns only the staking era,
// external validators aka non-harmony nodes
func (ss *State) ExternalValidators() []common.Address {
	processed := make(map[common.Address]struct{})
	for i := range ss.Shards {
		shard := ss.Shards[i]
		for j := range shard.Slots {
			slot := shard.Slots[j]
			if slot.EffectiveStake != nil { // For external validator
				_, ok := processed[slot.EcdsaAddress]
				if !ok {
					processed[slot.EcdsaAddress] = struct{}{}

				}
			}
		}
	}
	slice, i := make([]common.Address, len(processed)), 0
	for key := range processed {
		slice[i] = key
		i++
	}
	return slice
}

// JSON produces a non-pretty printed JSON string of the SuperCommittee
func (ss *State) JSON() string {
	type t struct {
		Slot
		EcdsaAddress string `json:"ecdsa-address"`
	}
	type v struct {
		Committee
		Count    int `json:"member-count"`
		NodeList []t `json:"subcommittee"`
	}
	dump := make([]v, len(ss.Shards))
	for i := range ss.Shards {
		c := len(ss.Shards[i].Slots)
		dump[i].ShardID = ss.Shards[i].ShardID
		dump[i].NodeList = make([]t, c)
		dump[i].Count = c
		for j := range ss.Shards[i].Slots {
			n := ss.Shards[i].Slots[j]
			dump[i].NodeList[j].BlsPublicKey = n.BlsPublicKey
			dump[i].NodeList[j].EffectiveStake = n.EffectiveStake
			dump[i].NodeList[j].EcdsaAddress = common2.MustAddressToBech32(n.EcdsaAddress)
		}
	}
	buf, _ := json.Marshal(dump)
	return string(buf)
}

// FindCommitteeByID returns the committee configuration for the given shard,
// or nil if the given shard is not found.
func (ss *State) FindCommitteeByID(shardID uint32) *Committee {
	if ss == nil {
		return nil
	}
	for committee := range ss.Shards {
		if ss.Shards[committee].ShardID == shardID {
			return &ss.Shards[committee]
		}
	}
	return nil
}

// DeepCopy returns a deep copy of the receiver.
func (ss *State) DeepCopy() *State {
	var r State
	if ss.Epoch != nil {
		r.Epoch = big.NewInt(0).Set(ss.Epoch)
	}
	for _, c := range ss.Shards {
		r.Shards = append(r.Shards, c.DeepCopy())
	}
	return &r
}

// Big ..
func (pk BlsPublicKey) Big() *big.Int {
	return new(big.Int).SetBytes(pk[:])
}

// IsEmpty returns whether the bls public key is empty 0 bytes
func (pk BlsPublicKey) IsEmpty() bool {
	return bytes.Compare(pk[:], emptyBlsPubKey[:]) == 0
}

// Hex returns the hex string of bls public key
func (pk BlsPublicKey) Hex() string {
	return hex.EncodeToString(pk[:])
}

// MarshalJSON ..
func (pk BlsPublicKey) MarshalJSON() ([]byte, error) {
	buf := bytes.Buffer{}
	buf.WriteString(`"`)
	buf.WriteString(pk.Hex())
	buf.WriteString(`"`)
	return buf.Bytes(), nil
}

// FromLibBLSPublicKeyUnsafe could give back nil, use only in cases when
// have invariant that return value won't be nil
func FromLibBLSPublicKeyUnsafe(key *bls.PublicKey) *BlsPublicKey {
	result := &BlsPublicKey{}
	if err := result.FromLibBLSPublicKey(key); err != nil {
		return nil
	}
	return result
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

// CompareNodeID compares two node IDs.
func CompareNodeID(id1, id2 *Slot) int {
	if c := bytes.Compare(id1.EcdsaAddress[:], id2.EcdsaAddress[:]); c != 0 {
		return c
	}
	if c := CompareBlsPublicKey(id1.BlsPublicKey, id2.BlsPublicKey); c != 0 {
		return c
	}
	return 0
}

// DeepCopy returns a deep copy of the receiver.
func (l SlotList) DeepCopy() SlotList {
	return append(l[:0:0], l...)
}

// CompareNodeIDList compares two node ID lists.
func CompareNodeIDList(l1, l2 SlotList) int {
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
	r.Slots = c.Slots.DeepCopy()
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
	if c := CompareNodeIDList(c1.Slots, c2.Slots); c != 0 {
		return c
	}
	return 0
}

// GetHashFromNodeList will sort the list, then use Keccak256 to hash the list
// NOTE: do not modify the underlining content for hash
func GetHashFromNodeList(nodeList []Slot) []byte {
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

// Hash is the root hash of State
func (ss *State) Hash() (h common.Hash) {
	// TODO ek – this sorting really doesn't belong here; it should instead
	//  be made an explicit invariant to be maintained and, if needed, checked.
	copy := ss.DeepCopy()
	sort.Slice(copy.Shards, func(i, j int) bool {
		return copy.Shards[i].ShardID < copy.Shards[j].ShardID
	})
	d := sha3.NewLegacyKeccak256()
	for i := range copy.Shards {
		hash := GetHashFromNodeList(copy.Shards[i].Slots)
		d.Write(hash)
	}
	d.Sum(h[:0])
	return h
}

// Serialize serialize Slot into bytes
func (n Slot) Serialize() []byte {
	return append(n.EcdsaAddress[:], n.BlsPublicKey[:]...)
}

func (n Slot) String() string {
	total := "nil"
	if n.EffectiveStake != nil {
		total = n.EffectiveStake.String()
	}
	return "ECDSA: " + common2.MustAddressToBech32(n.EcdsaAddress) + ", BLS: " + hex.EncodeToString(n.BlsPublicKey[:]) + ", EffectiveStake: " + total
}
