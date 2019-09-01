package types

import (
	"math/big"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/shard"
)

// Header represents a block header in the Harmony blockchain.
type Header struct {
	ParentHash          common.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase            common.Address `json:"miner"            gencodec:"required"`
	Root                common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash              common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash         common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	OutgoingReceiptHash common.Hash    `json:"outgoingReceiptsRoot"     gencodec:"required"`
	IncomingReceiptHash common.Hash    `json:"incomingReceiptsRoot" gencodec:"required"`
	Bloom               types.Bloom    `json:"logsBloom"        gencodec:"required"`
	Number              *big.Int       `json:"number"           gencodec:"required"`
	GasLimit            uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed             uint64         `json:"gasUsed"          gencodec:"required"`
	Time                *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra               []byte         `json:"extraData"        gencodec:"required"`
	MixDigest           common.Hash    `json:"mixHash"          gencodec:"required"`
	// Additional Fields
	ViewID              *big.Int    `json:"viewID"           gencodec:"required"`
	Epoch               *big.Int    `json:"epoch"            gencodec:"required"`
	ShardID             uint32      `json:"shardID"          gencodec:"required"`
	LastCommitSignature [96]byte    `json:"lastCommitSignature"  gencodec:"required"`
	LastCommitBitmap    []byte      `json:"lastCommitBitmap"     gencodec:"required"` // Contains which validator signed
	ShardStateHash      common.Hash `json:"shardStateRoot"`
	Vrf                 []byte      `json:"vrf"`
	Vdf                 []byte      `json:"vdf"`
	ShardState          []byte      `json:"shardState"`
	CrossLinks          []byte      `json:"crossLink"`
}

// field type overrides for gencodec
type headerMarshaling struct {
	Difficulty *hexutil.Big
	Number     *hexutil.Big
	GasLimit   hexutil.Uint64
	GasUsed    hexutil.Uint64
	Time       *hexutil.Big
	Extra      hexutil.Bytes
	Hash       common.Hash `json:"hash"` // adds call to Hash() in MarshalJSON
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	return hash.FromRLP(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	// TODO: update with new fields
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Number.BitLen()+h.Time.BitLen())/8)
}

// Logger returns a sub-logger with block contexts added.
func (h *Header) Logger(logger *zerolog.Logger) *zerolog.Logger {
	nlogger := logger.
		With().
		Str("blockHash", h.Hash().Hex()).
		Uint32("blockShard", h.ShardID).
		Uint64("blockEpoch", h.Epoch.Uint64()).
		Uint64("blockNumber", h.Number.Uint64()).
		Logger()
	return &nlogger
}

// GetShardState returns the deserialized shard state object.
func (h *Header) GetShardState() (shard.ShardState, error) {
	shardState := shard.ShardState{}
	err := rlp.DecodeBytes(h.ShardState, &shardState)
	if err != nil {
		return nil, err
	}
	return shardState, nil
}

