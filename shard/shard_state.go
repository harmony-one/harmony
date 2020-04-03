package shard

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/ctxerror"
	"github.com/harmony-one/harmony/numeric"
	"golang.org/x/crypto/sha3"
	"golang.org/x/sync/singleflight"
)

var (
	emptyBLSPubKey = BLSPublicKey{}
	// ErrShardIDNotInSuperCommittee ..
	ErrShardIDNotInSuperCommittee = errors.New("shardID not in super committee")
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

// BLSPublicKey defines the bls public key
type BLSPublicKey [PublicKeySizeInBytes]byte

// BLSSignature defines the bls signature
type BLSSignature [BLSSignatureSizeInBytes]byte

// Slot represents node id (BLS address)
type Slot struct {
	EcdsaAddress common.Address `json:"ecdsa-address"`
	BLSPublicKey BLSPublicKey   `json:"bls-pubkey"`
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

func (l SlotList) String() string {
	blsKeys := make([]string, len(l))
	for i, k := range l {
		blsKeys[i] = k.BLSPublicKey.Hex()
	}
	s, _ := json.Marshal(blsKeys)
	return string(s)
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
	BLSPublicKey BLSPublicKey   `json:"bls-pubkey"`
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
					slot.EcdsaAddress, slot.BLSPublicKey, nil,
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
					slot.EcdsaAddress, slot.BLSPublicKey,
				})
			}
		}

		data, err = rlp.EncodeToBytes(shardStateLegacy)
	}

	return data, err
}

// StakedSlots gives overview of members
// in a subcommittee (aka a shard)
type StakedSlots struct {
	CountStakedValidator int
	CountStakedBLSKey    int
	Addrs                []common.Address
	LookupSet            map[common.Address]struct{}
	TotalEffectiveStaked numeric.Dec
}

// StakedValidators ..
func (c Committee) StakedValidators() *StakedSlots {
	countStakedValidator, countStakedBLSKey := 0, 0
	networkWideSlice, networkWideSet :=
		[]common.Address{}, map[common.Address]struct{}{}
	totalEffectiveStake := numeric.ZeroDec()

	for _, slot := range c.Slots {
		// an external validator,
		// non-nil EffectiveStake is how we known
		if addr := slot.EcdsaAddress; slot.EffectiveStake != nil {
			totalEffectiveStake = totalEffectiveStake.Add(*slot.EffectiveStake)
			countStakedBLSKey++
			if _, seen := networkWideSet[addr]; !seen {
				countStakedValidator++
				networkWideSet[addr] = struct{}{}
				networkWideSlice = append(networkWideSlice, addr)
			}
		}
	}

	return &StakedSlots{
		CountStakedValidator: countStakedValidator,
		CountStakedBLSKey:    countStakedBLSKey,
		Addrs:                networkWideSlice,
		LookupSet:            networkWideSet,
		TotalEffectiveStaked: totalEffectiveStake,
	}
}

// TODO refactor with and update corresponding places
// func (ss *State) StakedValidators() []*StakedSlots {
// 	networkWide := make([]*StakedSlots, len(ss.Shards))
// 	for i := range ss.Shards {
// 		networkWide[i] = ss.Shards[i].StakedValidators()
// 	}
// 	return networkWide
// }

// StakedValidators here is supercommittee wide
func (ss *State) StakedValidators() *StakedSlots {
	countStakedValidator, countStakedBLSKey := 0, 0
	networkWideSlice, networkWideSet :=
		[]common.Address{},
		map[common.Address]struct{}{}

	totalEffectiveStake := numeric.ZeroDec()

	for i := range ss.Shards {
		shard := ss.Shards[i]
		for j := range shard.Slots {

			slot := shard.Slots[j]
			// an external validator,
			// non-nil EffectiveStake is how we known
			if addr := slot.EcdsaAddress; slot.EffectiveStake != nil {
				totalEffectiveStake = totalEffectiveStake.Add(*slot.EffectiveStake)
				countStakedBLSKey++
				if _, seen := networkWideSet[addr]; !seen {
					countStakedValidator++
					networkWideSet[addr] = struct{}{}
					networkWideSlice = append(networkWideSlice, addr)
				}
			}
		}
	}

	return &StakedSlots{
		CountStakedValidator: countStakedValidator,
		CountStakedBLSKey:    countStakedBLSKey,
		Addrs:                networkWideSlice,
		LookupSet:            networkWideSet,
		TotalEffectiveStaked: totalEffectiveStake,
	}
}

// String produces a non-pretty printed JSON string of the SuperCommittee
func (ss *State) String() string {
	s, _ := json.Marshal(ss)
	return string(s)
}

// MarshalJSON ..
func (ss *State) MarshalJSON() ([]byte, error) {
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
			dump[i].NodeList[j].BLSPublicKey = n.BLSPublicKey
			dump[i].NodeList[j].EffectiveStake = n.EffectiveStake
			dump[i].NodeList[j].EcdsaAddress = common2.MustAddressToBech32(n.EcdsaAddress)
		}
	}
	return json.Marshal(dump)
}

// FindCommitteeByID returns the committee configuration for the given shard,
// or nil if the given shard is not found.
func (ss *State) FindCommitteeByID(shardID uint32) (*Committee, error) {
	if ss == nil {
		return nil, ErrShardIDNotInSuperCommittee
	}
	for committee := range ss.Shards {
		if ss.Shards[committee].ShardID == shardID {
			return &ss.Shards[committee], nil
		}
	}
	return nil, ErrShardIDNotInSuperCommittee
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
func (pk BLSPublicKey) Big() *big.Int {
	return new(big.Int).SetBytes(pk[:])
}

// IsEmpty returns whether the bls public key is empty 0 bytes
func (pk BLSPublicKey) IsEmpty() bool {
	return bytes.Equal(pk[:], emptyBLSPubKey[:])
}

// Hex returns the hex string of bls public key
func (pk BLSPublicKey) Hex() string {
	return hex.EncodeToString(pk[:])
}

// MarshalText so that we can use this as JSON printable when used as
// key in a map
func (pk BLSPublicKey) MarshalText() (text []byte, err error) {
	text = make([]byte, BLSSignatureSizeInBytes)
	hex.Encode(text, pk[:])
	return text, nil
}

// FromLibBLSPublicKeyUnsafe could give back nil, use only in cases when
// have invariant that return value won't be nil
func FromLibBLSPublicKeyUnsafe(key *bls.PublicKey) *BLSPublicKey {
	result := &BLSPublicKey{}
	if err := result.FromLibBLSPublicKey(key); err != nil {
		return nil
	}
	return result
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
func CompareNodeID(id1, id2 *Slot) int {
	if c := bytes.Compare(id1.EcdsaAddress[:], id2.EcdsaAddress[:]); c != 0 {
		return c
	}
	if c := CompareBLSPublicKey(id1.BLSPublicKey, id2.BLSPublicKey); c != 0 {
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
func (c *Committee) DeepCopy() Committee {
	r := Committee{}
	r.ShardID = c.ShardID
	r.Slots = c.Slots.DeepCopy()
	return r
}

// Hash ..
func (c *Committee) Hash() common.Hash {
	return hash.FromRLPNew256(c)
}

var (
	blsKeyCache singleflight.Group
)

func lookupBLSPublicKeys(
	c *Committee,
) ([]*bls.PublicKey, error) {
	key := c.Hash().Hex()
	results, err, _ := blsKeyCache.Do(
		key, func() (interface{}, error) {
			slice := make([]*bls.PublicKey, len(c.Slots))
			for j := range c.Slots {
				committerKey := &bls.PublicKey{}
				if err := c.Slots[j].BLSPublicKey.ToLibBLSPublicKey(
					committerKey,
				); err != nil {
					return nil, err
				}
				slice[j] = committerKey
			}
			// Only made once
			go func() {
				time.Sleep(25 * time.Minute)
				blsKeyCache.Forget(key)
			}()
			return slice, nil
		},
	)
	if err != nil {
		return nil, err
	}

	return results.([]*bls.PublicKey), nil
}

// BLSPublicKeys ..
func (c *Committee) BLSPublicKeys() ([]*bls.PublicKey, error) {
	if c == nil {
		return nil, ErrSubCommitteeNil
	}
	return lookupBLSPublicKeys(c)
}

var (
	// ErrValidNotInCommittee ..
	ErrValidNotInCommittee = errors.New("slot signer not this slot's subcommittee")
	// ErrSubCommitteeNil ..
	ErrSubCommitteeNil = errors.New("subcommittee is nil pointer")
	// ErrSuperCommitteeNil ..
	ErrSuperCommitteeNil = errors.New("supercommittee is nil pointer")
)

// AddressForBLSKey ..
func (c *Committee) AddressForBLSKey(key BLSPublicKey) (*common.Address, error) {
	if c == nil {
		return nil, ErrSubCommitteeNil
	}

	for _, slot := range c.Slots {
		if CompareBLSPublicKey(slot.BLSPublicKey, key) == 0 {
			return &slot.EcdsaAddress, nil
		}
	}
	return nil, ErrValidNotInCommittee
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
	if len(nodeList) == 0 {
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
	return append(n.EcdsaAddress[:], n.BLSPublicKey[:]...)
}

func (n Slot) String() string {
	total := "nil"
	if n.EffectiveStake != nil {
		total = n.EffectiveStake.String()
	}
	return "ECDSA: " +
		common2.MustAddressToBech32(n.EcdsaAddress) +
		", BLS: " +
		hex.EncodeToString(n.BLSPublicKey[:]) +
		", EffectiveStake: " +
		total
}
