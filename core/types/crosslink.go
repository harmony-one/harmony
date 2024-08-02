package types

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
)

// CrossLinkV1 is only used on beacon chain to store the hash links from other shards
// signature and bitmap correspond to |blockNumber|parentHash| byte array
// Capital to enable rlp encoding
// Here we replace header to signatures only, the basic assumption is the committee will not be
// corrupted during one epoch, which is the same as consensus assumption
type CrossLinkV1 struct {
	HashF        common.Hash
	BlockNumberF *big.Int
	ViewIDF      *big.Int
	SignatureF   [96]byte //aggregated signature
	BitmapF      []byte   //corresponding bitmap mask for agg signature
	ShardIDF     uint32   //will be verified with signature on |blockNumber|blockHash| is correct
	EpochF       *big.Int
}

// NewCrossLinkV1 returns a new cross link object
func NewCrossLinkV1(header *block.Header, parentHeader *block.Header) *CrossLinkV1 {
	parentBlockNum := big.NewInt(0)
	parentViewID := big.NewInt(0)
	parentEpoch := big.NewInt(0)
	if parentHeader != nil { // Should always happen, default values to be defensive.
		parentBlockNum = parentHeader.Number()
		parentViewID = parentHeader.ViewID()
		parentEpoch = parentHeader.Epoch()
	}
	return &CrossLinkV1{
		HashF:        header.ParentHash(),
		BlockNumberF: parentBlockNum,
		ViewIDF:      parentViewID,
		SignatureF:   header.LastCommitSignature(),
		BitmapF:      header.LastCommitBitmap(),
		ShardIDF:     header.ShardID(),
		EpochF:       parentEpoch,
	}
}

// ShardID returns shardID
func (cl *CrossLinkV1) ShardID() uint32 {
	return cl.ShardIDF
}

// Number returns blockNum with big.Int format
func (cl *CrossLinkV1) Number() *big.Int {
	return cl.BlockNumberF
}

// ViewID returns viewID with big.Int format
func (cl *CrossLinkV1) ViewID() *big.Int {
	return cl.ViewIDF
}

// Epoch returns epoch with big.Int format
func (cl *CrossLinkV1) Epoch() *big.Int {
	return cl.EpochF
}

// BlockNum returns blockNum
func (cl *CrossLinkV1) BlockNum() uint64 {
	return cl.BlockNumberF.Uint64()
}

// Hash returns hash
func (cl *CrossLinkV1) Hash() common.Hash {
	return cl.HashF
}

// Bitmap returns bitmap
func (cl *CrossLinkV1) Bitmap() []byte {
	return cl.BitmapF
}

// Signature returns aggregated signature
func (cl *CrossLinkV1) Signature() [96]byte {
	return cl.SignatureF
}

// MarshalJSON ..
func (cl *CrossLinkV1) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash        common.Hash `json:"hash"`
		BlockNumber *big.Int    `json:"block-number"`
		ViewID      *big.Int    `json:"view-id"`
		Signature   string      `json:"signature"`
		Bitmap      string      `json:"signature-bitmap"`
		ShardID     uint32      `json:"shard-id"`
		EpochNumber *big.Int    `json:"epoch-number"`
	}{
		cl.HashF,
		cl.BlockNumberF,
		cl.ViewIDF,
		hex.EncodeToString(cl.SignatureF[:]),
		hex.EncodeToString(cl.BitmapF),
		cl.ShardIDF,
		cl.EpochF,
	})
}

// Serialize returns bytes of cross link rlp-encoded content
func (cl *CrossLinkV1) Serialize() []byte {
	bytes, _ := rlp.EncodeToBytes(cl)
	return bytes
}

func (cl *CrossLinkV1) BlockProposer() common.Address {
	return common.Address{}
}

// DeserializeCrossLinkV1 rlp-decode the bytes into cross link object.
func DeserializeCrossLinkV1(bytes []byte) (*CrossLinkV1, error) {
	cl := &CrossLinkV1{}
	err := rlp.DecodeBytes(bytes, cl)
	if err != nil {
		return nil, err
	}
	return cl, err
}

func DeserializeCrossLinks(bytes []byte) (CrossLinks, error) {
	if rs, err := DeserializeCrossLinkV2(bytes); err == nil {
		return CrossLinks{rs}, nil
	}
	rs, err := DeserializeCrossLinkV1(bytes)
	if err != nil {
		return nil, err
	}
	return CrossLinks{rs}, nil
}

func DeserializeCrossLinkV2(bytes []byte) (*CrossLinkV2, error) {
	cl := &CrossLinkV2{}
	err := rlp.DecodeBytes(bytes, cl)
	if err != nil {
		return nil, err
	}
	return cl, err
}

func (cl *CrossLinkV2) Serialize() []byte {
	bytes, _ := rlp.EncodeToBytes(cl)
	return bytes
}

// CrossLinksV1 is a collection of crosslinks
type CrossLinksV1 []CrossLinkV1
type CrossLinks []CrossLink

// Sort crosslinks by shardID and then tie break by blockNum then by viewID
func (cls CrossLinks) Sort() {
	sort.Slice(cls, func(i, j int) bool {
		if s1, s2 := cls[i].ShardID(), cls[j].ShardID(); s1 != s2 {
			return s1 < s2
		}
		if s1, s2 := cls[i].Number(), cls[j].Number(); s1.Cmp(s2) != 0 {
			return s1.Cmp(s2) < 0
		}
		return cls[i].ViewID().Cmp(cls[j].ViewID()) < 0
	})
}

// IsSorted checks whether the cross links are sorted
func (cls CrossLinks) IsSorted() bool {
	return sort.SliceIsSorted(cls, func(i, j int) bool {
		if s1, s2 := cls[i].ShardID(), cls[j].ShardID(); s1 != s2 {
			return s1 < s2
		}
		if s1, s2 := cls[i].Number(), cls[j].Number(); s1.Cmp(s2) != 0 {
			return s1.Cmp(s2) < 0
		}
		return cls[i].ViewID().Cmp(cls[j].ViewID()) < 0
	})
}

type CrossLinkV2 struct {
	CrossLinkV1
	Proposer common.Address
}

func (cl *CrossLinkV2) BlockProposer() common.Address {
	return cl.Proposer
}

type CrossLink interface {
	ShardID() uint32
	ViewID() *big.Int
	Epoch() *big.Int
	Number() *big.Int
	Hash() common.Hash
	Bitmap() []byte
	Signature() [96]byte
	BlockProposer() common.Address
	BlockNum() uint64
	Serialize() []byte
}
