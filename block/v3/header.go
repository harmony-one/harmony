package v3

import (
	"io"
	"math/big"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/rs/zerolog"

	blockif "github.com/harmony-one/harmony/block/interface"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// Header is the V3 block header.
// V3 block header is exactly the same
// we copy the code instead of embedded v2 header into v3
// when we do type checking in NewBodyForMatchingHeader
// the embedded structure will return v2 header type instead of v3 type
type Header struct {
	fields headerFields
}

// EncodeRLP encodes the header fields into RLP format.
func (h *Header) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &h.fields)
}

// DecodeRLP decodes the given RLP decode stream into the header fields.
func (h *Header) DecodeRLP(s *rlp.Stream) error {
	return s.Decode(&h.fields)
}

// NewHeader creates a new header object.
func NewHeader() *Header {
	return &Header{headerFields{
		Number: new(big.Int),
		Time:   new(big.Int),
		ViewID: new(big.Int),
		Epoch:  new(big.Int),
	}}
}

type headerFields struct {
	ParentHash          common.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase            common.Address `json:"miner"            gencodec:"required"`
	Root                common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash              common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash         common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	OutgoingReceiptHash common.Hash    `json:"outgoingReceiptsRoot"     gencodec:"required"`
	IncomingReceiptHash common.Hash    `json:"incomingReceiptsRoot" gencodec:"required"`
	Bloom               ethtypes.Bloom `json:"logsBloom"        gencodec:"required"`
	Number              *big.Int       `json:"number"           gencodec:"required"`
	GasLimit            uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed             uint64         `json:"gasUsed"          gencodec:"required"`
	Time                *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra               []byte         `json:"extraData"        gencodec:"required"`
	MixDigest           common.Hash    `json:"mixHash"          gencodec:"required"`
	// Additional Fields
	ViewID              *big.Int `json:"viewID"           gencodec:"required"`
	Epoch               *big.Int `json:"epoch"            gencodec:"required"`
	ShardID             uint32   `json:"shardID"          gencodec:"required"`
	LastCommitSignature [96]byte `json:"lastCommitSignature"  gencodec:"required"`
	LastCommitBitmap    []byte   `json:"lastCommitBitmap"     gencodec:"required"` // Contains which validator signed
	Vrf                 []byte   `json:"vrf"`
	Vdf                 []byte   `json:"vdf"`
	ShardState          []byte   `json:"shardState"`
	CrossLinks          []byte   `json:"crossLink"`
	Slashes             []byte   `json:"slashes"`
}

// ParentHash is the header hash of the parent block.  For the genesis block
// which has no parent by definition, this field is zeroed out.
func (h *Header) ParentHash() common.Hash {
	return h.fields.ParentHash
}

// SetParentHash sets the parent hash field.
func (h *Header) SetParentHash(newParentHash common.Hash) {
	h.fields.ParentHash = newParentHash
}

// Coinbase is now the first 20 bytes of the SHA256 hash of the leader's
// public BLS key. This is required for EVM compatibility.
func (h *Header) Coinbase() common.Address {
	return h.fields.Coinbase
}

// SetCoinbase sets the coinbase address field.
func (h *Header) SetCoinbase(newCoinbase common.Address) {
	h.fields.Coinbase = newCoinbase
}

// Root is the state (account) trie root hash.
func (h *Header) Root() common.Hash {
	return h.fields.Root
}

// SetRoot sets the state trie root hash field.
func (h *Header) SetRoot(newRoot common.Hash) {
	h.fields.Root = newRoot
}

// TxHash is the transaction trie root hash.
func (h *Header) TxHash() common.Hash {
	return h.fields.TxHash
}

// SetTxHash sets the transaction trie root hash field.
func (h *Header) SetTxHash(newTxHash common.Hash) {
	h.fields.TxHash = newTxHash
}

// ReceiptHash is the same-shard transaction receipt trie hash.
func (h *Header) ReceiptHash() common.Hash {
	return h.fields.ReceiptHash
}

// SetReceiptHash sets the same-shard transaction receipt trie hash.
func (h *Header) SetReceiptHash(newReceiptHash common.Hash) {
	h.fields.ReceiptHash = newReceiptHash
}

// OutgoingReceiptHash is the egress transaction receipt trie hash.
func (h *Header) OutgoingReceiptHash() common.Hash {
	return h.fields.OutgoingReceiptHash
}

// SetOutgoingReceiptHash sets the egress transaction receipt trie hash.
func (h *Header) SetOutgoingReceiptHash(newOutgoingReceiptHash common.Hash) {
	h.fields.OutgoingReceiptHash = newOutgoingReceiptHash
}

// IncomingReceiptHash is the ingress transaction receipt trie hash.
func (h *Header) IncomingReceiptHash() common.Hash {
	return h.fields.IncomingReceiptHash
}

// SetIncomingReceiptHash sets the ingress transaction receipt trie hash.
func (h *Header) SetIncomingReceiptHash(newIncomingReceiptHash common.Hash) {
	h.fields.IncomingReceiptHash = newIncomingReceiptHash
}

// Bloom is the Bloom filter that indexes accounts and topics logged by smart
// contract transactions (executions) in this block.
func (h *Header) Bloom() ethtypes.Bloom {
	return h.fields.Bloom
}

// SetBloom sets the smart contract log Bloom filter for this block.
func (h *Header) SetBloom(newBloom ethtypes.Bloom) {
	h.fields.Bloom = newBloom
}

// Number is the block number.
//
// The returned instance is a copy; the caller may do anything with it.
func (h *Header) Number() *big.Int {
	return new(big.Int).Set(h.fields.Number)
}

// SetNumber sets the block number.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetNumber(newNumber *big.Int) {
	h.fields.Number = new(big.Int).Set(newNumber)
}

// GasLimit is the gas limit for transactions in this block.
func (h *Header) GasLimit() uint64 {
	return h.fields.GasLimit
}

// SetGasLimit sets the gas limit for transactions in this block.
func (h *Header) SetGasLimit(newGasLimit uint64) {
	h.fields.GasLimit = newGasLimit
}

// GasUsed is the amount of gas used by transactions in this block.
func (h *Header) GasUsed() uint64 {
	return h.fields.GasUsed
}

// SetGasUsed sets the amount of gas used by transactions in this block.
func (h *Header) SetGasUsed(newGasUsed uint64) {
	h.fields.GasUsed = newGasUsed
}

// Time is the UNIX timestamp of this block.
//
// The returned instance is a copy; the caller may do anything with it.
func (h *Header) Time() *big.Int {
	return new(big.Int).Set(h.fields.Time)
}

// SetTime sets the UNIX timestamp of this block.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetTime(newTime *big.Int) {
	h.fields.Time = new(big.Int).Set(newTime)
}

// Extra is the extra data field of this block.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) Extra() []byte {
	return append(h.fields.Extra[:0:0], h.fields.Extra...)
}

// SetExtra sets the extra data field of this block.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetExtra(newExtra []byte) {
	h.fields.Extra = append(newExtra[:0:0], newExtra...)
}

// MixDigest is the mixhash.
//
// This field is a remnant from Ethereum, and Harmony does not use it and always
// zeroes it out.
func (h *Header) MixDigest() common.Hash {
	return h.fields.MixDigest
}

// SetMixDigest sets the mixhash of this block.
func (h *Header) SetMixDigest(newMixDigest common.Hash) {
	h.fields.MixDigest = newMixDigest
}

// ViewID is the ID of the view in which this block was originally proposed.
//
// It normally increases by one for each subsequent block, or by more than one
// if one or more PBFT/FBFT view changes have occurred.
//
// The returned instance is a copy; the caller may do anything with it.
func (h *Header) ViewID() *big.Int {
	return new(big.Int).Set(h.fields.ViewID)
}

// SetViewID sets the view ID in which the block was originally proposed.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetViewID(newViewID *big.Int) {
	h.fields.ViewID = new(big.Int).Set(newViewID)
}

// Epoch is the epoch number of this block.
//
// The returned instance is a copy; the caller may do anything with it.
func (h *Header) Epoch() *big.Int {
	return new(big.Int).Set(h.fields.Epoch)
}

// SetEpoch sets the epoch number of this block.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetEpoch(newEpoch *big.Int) {
	h.fields.Epoch = new(big.Int).Set(newEpoch)
}

// ShardID is the shard ID to which this block belongs.
func (h *Header) ShardID() uint32 {
	return h.fields.ShardID
}

// SetShardID sets the shard ID to which this block belongs.
func (h *Header) SetShardID(newShardID uint32) {
	h.fields.ShardID = newShardID
}

// LastCommitSignature is the FBFT commit group signature for the last block.
func (h *Header) LastCommitSignature() [96]byte {
	return h.fields.LastCommitSignature
}

// SetLastCommitSignature sets the FBFT commit group signature for the last
// block.
func (h *Header) SetLastCommitSignature(newLastCommitSignature [96]byte) {
	h.fields.LastCommitSignature = newLastCommitSignature
}

// LastCommitBitmap is the signatory bitmap of the previous block.  Bit
// positions index into committee member array.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) LastCommitBitmap() []byte {
	return append(h.fields.LastCommitBitmap[:0:0], h.fields.LastCommitBitmap...)
}

// SetLastCommitBitmap sets the signatory bitmap of the previous block.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetLastCommitBitmap(newLastCommitBitmap []byte) {
	h.fields.LastCommitBitmap = append(newLastCommitBitmap[:0:0], newLastCommitBitmap...)
}

// ShardStateHash is the shard state hash.
func (h *Header) ShardStateHash() common.Hash {
	return common.Hash{}
}

// SetShardStateHash sets the shard state hash.
func (h *Header) SetShardStateHash(newShardStateHash common.Hash) {
	h.Logger(utils.Logger()).Warn().
		Str("shardStateHash", newShardStateHash.Hex()).
		Msg("cannot store ShardStateHash in V3 header")
}

// Vrf is the output of the VRF for the epoch.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) Vrf() []byte {
	return append(h.fields.Vrf[:0:0], h.fields.Vrf...)
}

// SetVrf sets the output of the VRF for the epoch.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetVrf(newVrf []byte) {
	h.fields.Vrf = append(newVrf[:0:0], newVrf...)
}

// Vdf is the output of the VDF for the epoch.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) Vdf() []byte {
	return append(h.fields.Vdf[:0:0], h.fields.Vdf...)
}

// SetVdf sets the output of the VDF for the epoch.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetVdf(newVdf []byte) {
	h.fields.Vdf = append(newVdf[:0:0], newVdf...)
}

// ShardState is the RLP-encoded form of shard state (list of committees) for
// the next epoch.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) ShardState() []byte {
	return append(h.fields.ShardState[:0:0], h.fields.ShardState...)
}

// SetShardState sets the RLP-encoded form of shard state
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetShardState(newShardState []byte) {
	h.fields.ShardState = append(newShardState[:0:0], newShardState...)
}

// CrossLinks is the RLP-encoded form of non-beacon block headers chosen to be
// canonical by the beacon committee.  This field is present only on beacon
// chain block headers.
//
// The returned slice is a copy; the caller may do anything with it.
func (h *Header) CrossLinks() []byte {
	return append(h.fields.CrossLinks[:0:0], h.fields.CrossLinks...)
}

// SetCrossLinks sets the RLP-encoded form of non-beacon block headers chosen to
// be canonical by the beacon committee.
//
// It stores a copy; the caller may freely modify the original.
func (h *Header) SetCrossLinks(newCrossLinks []byte) {
	h.fields.CrossLinks = append(newCrossLinks[:0:0], newCrossLinks...)
}

// Slashes ..
func (h *Header) Slashes() []byte {
	return append(h.fields.Slashes[:0:0], h.fields.Slashes...)
}

// SetSlashes ..
func (h *Header) SetSlashes(newSlashes []byte) {
	h.fields.Slashes = append(newSlashes[:0:0], newSlashes...)
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
	return common.StorageSize(unsafe.Sizeof(*h)) +
		common.StorageSize(len(h.Extra())+(h.Number().BitLen()+
			h.Time().BitLen())/8,
		)
}

// Logger returns a sub-logger with block contexts added.
func (h *Header) Logger(logger *zerolog.Logger) *zerolog.Logger {
	nlogger := logger.
		With().
		Str("blockHash", h.Hash().Hex()).
		Uint32("blockShard", h.ShardID()).
		Uint64("blockEpoch", h.Epoch().Uint64()).
		Uint64("blockNumber", h.Number().Uint64()).
		Logger()
	return &nlogger
}

// GetShardState returns the deserialized shard state object.
func (h *Header) GetShardState() (shard.State, error) {
	state, err := shard.DecodeWrapper(h.ShardState())
	if err != nil {
		return shard.State{}, err
	}
	return *state, nil
}

// Copy returns a copy of the given header.
func (h *Header) Copy() blockif.Header {
	cpy := *h
	return &cpy
}
