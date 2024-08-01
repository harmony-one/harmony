package block

import (
	"encoding/json"
	"io"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	blockif "github.com/harmony-one/harmony/block/interface"
	v0 "github.com/harmony-one/harmony/block/v0"
	v1 "github.com/harmony-one/harmony/block/v1"
	v2 "github.com/harmony-one/harmony/block/v2"
	v3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/taggedrlp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type ReadHeader interface {
	// ParentHash is the header hash of the parent block.  For the genesis block
	// which has no parent by definition, this field is zeroed out.
	ParentHash() common.Hash

	// Coinbase is the address of the node that proposed this block and all
	// transactions in it.
	Coinbase() common.Address

	// Root is the state (account) trie root hash.
	Root() common.Hash

	// TxHash is the transaction trie root hash.
	TxHash() common.Hash

	// ReceiptHash is the same-shard transaction receipt trie hash.
	ReceiptHash() common.Hash

	// OutgoingReceiptHash is the egress transaction receipt trie hash.
	OutgoingReceiptHash() common.Hash

	// IncomingReceiptHash is the ingress transaction receipt trie hash.
	IncomingReceiptHash() common.Hash

	// Bloom is the Bloom filter that indexes accounts and topics logged by smart
	// contract transactions (executions) in this block.
	Bloom() types.Bloom

	// Number is the block number.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Number() *big.Int

	// GasLimit is the gas limit for transactions in this block.
	GasLimit() uint64

	// GasUsed is the amount of gas used by transactions in this block.
	GasUsed() uint64

	// Time is the UNIX timestamp of this block.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Time() *big.Int

	// Extra is the extra data field of this block.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Extra() []byte

	// MixDigest is the mixhash.
	//
	// This field is a remnant from Ethereum, and Harmony does not use it and always
	// zeroes it out.
	MixDigest() common.Hash

	// ViewID is the ID of the view in which this block was originally proposed.
	//
	// It normally increases by one for each subsequent block, or by more than one
	// if one or more PBFT/FBFT view changes have occurred.
	//
	// The returned instance is a copy; the caller may do anything with it.
	ViewID() *big.Int

	// Epoch is the epoch number of this block.
	//
	// The returned instance is a copy; the caller may do anything with it.
	Epoch() *big.Int

	// ShardStateHash is the shard state hash.
	ShardStateHash() common.Hash

	// LastCommitBitmap is the signatory bitmap of the previous block.  Bit
	// positions index into committee member array.
	//
	// The returned slice is a copy; the caller may do anything with it.
	LastCommitBitmap() []byte

	// Vrf is the output of the VRF for the epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Vrf() []byte

	// ShardID is the shard ID to which this block belongs.
	ShardID() uint32

	// Vdf is the output of the VDF for the epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	Vdf() []byte

	// CrossLinks is the RLP-encoded form of non-beacon block headers chosen to be
	// canonical by the beacon committee.  This field is present only on beacon
	// chain block headers.
	//
	// The returned slice is a copy; the caller may do anything with it.
	CrossLinks() []byte

	// LastCommitSignature is the FBFT commit group signature for the last block.
	LastCommitSignature() [96]byte

	// ShardState is the RLP-encoded form of shard state (list of committees) for
	// the next epoch.
	//
	// The returned slice is a copy; the caller may do anything with it.
	ShardState() []byte

	// Hash returns the block hash of the header, which is simply the legacy
	// Keccak256 hash of its RLP encoding.
	Hash() common.Hash

	// Size returns the approximate memory used by all internal contents. It is
	// used to approximate and limit the memory consumption of various caches.
	Size() common.StorageSize

	// Logger returns a sub-logger with block contexts added.
	Logger(logger *zerolog.Logger) *zerolog.Logger

	// GetShardState returns the deserialized shard state object.
	// Note that header encoded shard state only exists in the last block of the previous epoch
	GetShardState() (shard.State, error)

	// Copy returns a copy of the header.
	Copy() *Header

	// Slashes is the RLP-encoded form of []slash.Record,
	// The returned slice is a copy; the caller may do anything with it
	Slashes() []byte

	IsLastBlockInEpoch() bool

	NumberU64() uint64
}

var _ ReadHeader = (*Header)(nil)

// Header represents a block header in the Harmony blockchain.
type Header struct {
	blockif.Header
}

// HeaderPair ..
type HeaderPair struct {
	BeaconHeader *Header `json:"beacon-chain-header"`
	ShardHeader  *Header `json:"shard-chain-header"`
}

var (
	// ErrHeaderIsNil ..
	ErrHeaderIsNil = errors.New("cannot encode nil header receiver")
)

// MarshalJSON ..
func (h Header) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ParentHash  common.Hash      `json:"parentHash"`
		UncleHash   common.Hash      `json:"sha3Uncles"`
		Nonce       types.BlockNonce `json:"nonce"`
		Coinbase    common.Address   `json:"miner"`
		Root        common.Hash      `json:"stateRoot"`
		TxHash      common.Hash      `json:"transactionsRoot"`
		ReceiptHash common.Hash      `json:"receiptsRoot"`
		Bloom       types.Bloom      `json:"logsBloom"`
		Difficulty  *hexutil.Big     `json:"difficulty"`
		Number      *hexutil.Big     `json:"number"`
		GasLimit    hexutil.Uint64   `json:"gasLimit"`
		GasUsed     hexutil.Uint64   `json:"gasUsed"`
		Time        *hexutil.Big     `json:"timestamp"`
		Extra       hexutil.Bytes    `json:"extraData"`
		MixDigest   common.Hash      `json:"mixHash"`
		Hash        common.Hash      `json:"hash"`
		// Additional Fields
		ViewID  *big.Int `json:"viewID"`
		Epoch   *big.Int `json:"epoch"`
		ShardID uint32   `json:"shardID"`
	}{
		h.ParentHash(),
		common.Hash{},
		types.BlockNonce{},
		h.Coinbase(),
		h.Root(),
		h.TxHash(),
		h.ReceiptHash(),
		h.Bloom(),
		(*hexutil.Big)(big.NewInt(0)),
		(*hexutil.Big)(h.Number()),
		hexutil.Uint64(h.GasLimit()),
		hexutil.Uint64(h.GasUsed()),
		(*hexutil.Big)(h.Time()),
		h.Extra(),
		h.MixDigest(),
		h.Hash(),
		h.Header.ViewID(),
		h.Header.Epoch(),
		h.Header.ShardID(),
	})
}

// String ..
func (h Header) String() string {
	s, _ := json.Marshal(h)
	return string(s)
}

// EncodeRLP encodes the header using tagged RLP representation.
func (h *Header) EncodeRLP(w io.Writer) error {
	if h == nil {
		return ErrHeaderIsNil
	}
	return HeaderRegistry.Encode(w, h.Header)
}

// DecodeRLP decodes the header using tagged RLP representation.
func (h *Header) DecodeRLP(s *rlp.Stream) error {
	if h == nil {
		return ErrHeaderIsNil
	}
	decoded, err := HeaderRegistry.Decode(s)
	if err != nil {
		return err
	}
	hif, ok := decoded.(blockif.Header)
	if !ok {
		return errors.Errorf(
			"decoded object (type %s) does not implement Header interface",
			taggedrlp.TypeName(reflect.TypeOf(decoded)))
	}
	h.Header = hif
	return nil
}

// Hash returns the block hash of the header.  This uses HeaderRegistry to
// choose and return the right tagged RLP form of the header.
func (h *Header) Hash() common.Hash {
	return hash.FromRLP(h)
}

// NumberU64 returns the block number of the header as a uint64.
func (h *Header) NumberU64() uint64 {
	if h == nil {
		return 0
	}
	return h.Number().Uint64()
}

// Logger returns a sub-logger with block contexts added.
func (h *Header) Logger(logger *zerolog.Logger) *zerolog.Logger {
	nlogger := logger.With().
		Str("blockHash", h.Hash().Hex()).
		Uint32("blockShard", h.ShardID()).
		Uint64("blockEpoch", h.Epoch().Uint64()).
		Uint64("blockNumber", h.Number().Uint64()).
		Logger()
	return &nlogger
}

// With returns a field setter context for the header.
//
// Call a chain of setters on the returned field setter, followed by a call of
// Header method.  Example:
//
//	header := NewHeader(epoch).With().
//		ParentHash(parent.Hash()).
//		ShardID(parent.ShardID()).
//		Number(new(big.Int).Add(parent.Number(), big.NewInt(1)).
//		Header()
func (h *Header) With() HeaderFieldSetter {
	return HeaderFieldSetter{h: h}
}

// IsLastBlockInEpoch returns True if it is the last block of the epoch.
// Note that the last block contains the shard state of the next epoch.
func (h *Header) IsLastBlockInEpoch() bool {
	return len(h.ShardState()) > 0
}

func (h *Header) Copy() *Header {
	return &Header{Header: h.Header.Copy()}
}

// HeaderRegistry is the taggedrlp type registry for versioned headers.
var HeaderRegistry = taggedrlp.NewRegistry()

func init() {
	HeaderRegistry.MustRegister(taggedrlp.LegacyTag, v0.NewHeader())
	HeaderRegistry.MustAddFactory(func() interface{} { return v0.NewHeader() })
	HeaderRegistry.MustRegister("v1", v1.NewHeader())
	HeaderRegistry.MustAddFactory(func() interface{} { return v1.NewHeader() })
	HeaderRegistry.MustRegister("v2", v2.NewHeader())
	HeaderRegistry.MustAddFactory(func() interface{} { return v2.NewHeader() })
	HeaderRegistry.MustRegister("v3", v3.NewHeader())
	HeaderRegistry.MustAddFactory(func() interface{} { return v3.NewHeader() })
}
