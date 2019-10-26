package block

import (
	"io"
	"reflect"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/taggedrlp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	blockif "github.com/harmony-one/harmony/block/interface"
	v0 "github.com/harmony-one/harmony/block/v0"
	v1 "github.com/harmony-one/harmony/block/v1"
	v2 "github.com/harmony-one/harmony/block/v2"
	v3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/crypto/hash"
)

// Header represents a block header in the Harmony blockchain.
type Header struct {
	blockif.Header
}

// EncodeRLP encodes the header using tagged RLP representation.
func (h *Header) EncodeRLP(w io.Writer) error {
	return HeaderRegistry.Encode(w, h.Header)
}

// DecodeRLP decodes the header using tagged RLP representation.
func (h *Header) DecodeRLP(s *rlp.Stream) error {
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
func (h *Header) Hash() ethcommon.Hash {
	return hash.FromRLP(h)
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
