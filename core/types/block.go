// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package types contains data types related to Ethereum consensus.
package types

import (
	"encoding/binary"
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"

	"github.com/harmony-one/harmony/internal/utils"
)

// Constants for block.
var (
	EmptyRootHash  = DeriveSha(Transactions{})
	EmptyUncleHash = CalcUncleHash(nil)
)

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

// Header represents a block header in the Ethereum blockchain.
type Header struct {
	ParentHash  common.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase    common.Address `json:"miner"            gencodec:"required"`
	Root        common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash      common.Hash    `json:"transactionsRoot" gencodec:"required"`
	ReceiptHash common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	Bloom       Bloom          `json:"logsBloom"        gencodec:"required"`
	Difficulty  *big.Int       `json:"difficulty"       gencodec:"required"`
	Number      *big.Int       `json:"number"           gencodec:"required"`
	GasLimit    uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed     uint64         `json:"gasUsed"          gencodec:"required"`
	Time        *big.Int       `json:"timestamp"        gencodec:"required"`
	Extra       []byte         `json:"extraData"        gencodec:"required"`
	MixDigest   common.Hash    `json:"mixHash"          gencodec:"required"`
	Nonce       BlockNonce     `json:"nonce"            gencodec:"required"`
	// Additional Fields
	ShardID          uint32      `json:"shardID"          gencodec:"required"`
	PrepareSignature [48]byte    `json:"prepareSignature" gencodec:"required"`
	PrepareBitmap    []byte      `json:"prepareBitmap"    gencodec:"required"` // Contains which validator signed
	CommitSignature  [48]byte    `json:"commitSignature"  gencodec:"required"`
	CommitBitmap     []byte      `json:"commitBitmap"     gencodec:"required"` // Contains which validator signed
	RandPreimage     [32]byte    `json:"randPreimage"`
	RandSeed         [32]byte    `json:"randSeed"`
	ShardStateHash   common.Hash `json:"shardStateRoot"`
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
	return rlpHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *Header) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen())/8)
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions []*Transaction
	Uncles       []*Header
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	header       *Header
	uncles       []*Header
	transactions Transactions

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// SetPrepareSig sets the block's prepare group signature.
func (b *Block) SetPrepareSig(sig []byte, signers []byte) {
	if len(sig) != len(b.header.PrepareSignature) {
		utils.GetLogInstance().Warn("SetPrepareSig: sig size mismatch",
			"srcLen", len(sig),
			"dstLen", len(b.header.PrepareSignature))
	}
	copy(b.header.PrepareSignature[:], sig[:])
	b.header.PrepareBitmap = append(signers[:0:0], signers...)
}

// SetCommitSig sets the block's commit group signature.
func (b *Block) SetCommitSig(sig []byte, signers []byte) {
	if len(sig) != len(b.header.CommitSignature) {
		utils.GetLogInstance().Warn("SetCommitSig: sig size mismatch",
			"srcLen", len(sig),
			"dstLen", len(b.header.CommitSignature))
	}
	copy(b.header.CommitSignature[:], sig[:])
	b.header.CommitBitmap = append(signers[:0:0], signers...)
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}

// StorageBlock defines the RLP encoding of a Block stored in the
// state database. The StorageBlock encoding contains fields that
// would otherwise need to be recomputed.
// [deprecated by eth/63]
type StorageBlock Block

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
}

// [deprecated by eth/63]
// "storage" block encoding. used for database.
type storageblock struct {
	Header *Header
	Txs    []*Transaction
	Uncles []*Header
	TD     *big.Int
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs,
// and receipts.
func NewBlock(header *Header, txs []*Transaction, receipts []*Receipt) *Block {
	b := &Block{header: CopyHeader(header), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.TxHash = EmptyRootHash
	} else {
		b.header.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.ReceiptHash = EmptyRootHash
	} else {
		b.header.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions = eb.Header, eb.Uncles, eb.Txs
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header: b.header,
		Txs:    b.transactions,
		Uncles: b.uncles,
	})
}

// DecodeRLP decodes RLP
// [deprecated by eth/63]
func (b *StorageBlock) DecodeRLP(s *rlp.Stream) error {
	var sb storageblock
	if err := s.Decode(&sb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.td = sb.Header, sb.Uncles, sb.Txs, sb.TD
	return nil
}

// Uncles return uncles.
func (b *Block) Uncles() []*Header {
	return b.uncles
}

// Transactions returns transactions.
func (b *Block) Transactions() Transactions {
	return b.transactions
}

// Transaction returns Transaction.
func (b *Block) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}

// Number returns header number.
func (b *Block) Number() *big.Int { return new(big.Int).Set(b.header.Number) }

// GasLimit returns header gas limit.
func (b *Block) GasLimit() uint64 { return b.header.GasLimit }

// GasUsed returns header gas used.
func (b *Block) GasUsed() uint64 { return b.header.GasUsed }

// Difficulty is the header difficulty.
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }

// Time is header time.
func (b *Block) Time() *big.Int { return new(big.Int).Set(b.header.Time) }

// NumberU64 is the header number in uint64.
func (b *Block) NumberU64() uint64 { return b.header.Number.Uint64() }

// MixDigest is the header mix digest.
func (b *Block) MixDigest() common.Hash { return b.header.MixDigest }

// Nonce is the header nonce.
func (b *Block) Nonce() uint64 { return binary.BigEndian.Uint64(b.header.Nonce[:]) }

// ShardID is the header ShardID
func (b *Block) ShardID() uint32 { return b.header.ShardID }

// Bloom returns header bloom.
func (b *Block) Bloom() Bloom { return b.header.Bloom }

// Coinbase returns header coinbase.
func (b *Block) Coinbase() common.Address { return b.header.Coinbase }

// Root returns header root.
func (b *Block) Root() common.Hash { return b.header.Root }

// ParentHash return header parent hash.
func (b *Block) ParentHash() common.Hash { return b.header.ParentHash }

// TxHash returns header tx hash.
func (b *Block) TxHash() common.Hash { return b.header.TxHash }

// ReceiptHash returns header receipt hash.
func (b *Block) ReceiptHash() common.Hash { return b.header.ReceiptHash }

// Extra returns header extra.
func (b *Block) Extra() []byte { return common.CopyBytes(b.header.Extra) }

// Header returns a copy of Header.
func (b *Block) Header() *Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body { return &Body{b.transactions, b.uncles} }

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *Block) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// CalcUncleHash returns rlp hash of uncles.
func CalcUncleHash(uncles []*Header) common.Hash {
	return rlpHash(uncles)
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *Header) *Block {
	cpy := *header

	return &Block{
		header:       &cpy,
		transactions: b.transactions,
		uncles:       b.uncles,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, uncles []*Header) *Block {
	block := &Block{
		header:       CopyHeader(b.header),
		transactions: make([]*Transaction, len(transactions)),
		uncles:       make([]*Header, len(uncles)),
	}
	copy(block.transactions, transactions)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}

// Blocks is an array of Block.
type Blocks []*Block

// BlockBy is the func type.
type BlockBy func(b1, b2 *Block) bool

// Sort sorts blocks.
func (blockBy BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     blockBy,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 *Block) bool
}

// Len returns len of the blocks.
func (s blockSorter) Len() int {
	return len(s.blocks)
}

// Swap swaps block i and block j.
func (s blockSorter) Swap(i, j int) {
	s.blocks[i], s.blocks[j] = s.blocks[j], s.blocks[i]
}

// Less checks if block i is less than block j.
func (s blockSorter) Less(i, j int) bool {
	return s.by(s.blocks[i], s.blocks[j])
}

// Number checks if block b1 is less than block b2.
func Number(b1, b2 *Block) bool {
	return b1.header.Number.Cmp(b2.header.Number) < 0
}

// AddRandSeed add random seed into block header
func (b *Block) AddRandSeed(randSeed [32]byte) {
	b.header.RandSeed = randSeed
}

// AddRandPreimage add randomness preimage into block header
func (b *Block) AddRandPreimage(pRnd [32]byte) {
	b.header.RandPreimage = pRnd
}

// AddShardStateHash add shardStateHash into block header
func (b *Block) AddShardStateHash(shardStateHash common.Hash) {
	b.header.ShardStateHash = shardStateHash
}
