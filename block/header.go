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
	"github.com/harmony-one/taggedrlp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

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
