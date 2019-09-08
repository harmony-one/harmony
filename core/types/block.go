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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/rs/zerolog"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
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

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	Transactions     []*Transaction
	Uncles           []*block.Header
	IncomingReceipts CXReceiptsProofs
}

// Block represents an entire block in the Ethereum blockchain.
type Block struct {
	header           *block.Header
	uncles           []*block.Header
	transactions     Transactions
	incomingReceipts CXReceiptsProofs

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	// TODO: use it as chain weight (e.g. signatures/stakes)
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// SetLastCommitSig sets the last block's commit group signature.
func (b *Block) SetLastCommitSig(sig []byte, signers []byte) {
	if len(sig) != len(b.header.LastCommitSignature()) {
		utils.Logger().Warn().
			Int("srcLen", len(sig)).
			Int("dstLen", len(b.header.LastCommitSignature())).
			Msg("SetLastCommitSig: sig size mismatch")
	}
	var sig2 [96]byte
	copy(sig2[:], sig)
	b.header.SetLastCommitSignature(sig2)
	b.header.SetLastCommitBitmap(signers)
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header           *block.Header
	Txs              []*Transaction
	Uncles           []*block.Header
	IncomingReceipts CXReceiptsProofs
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs,
// and receipts.
func NewBlock(header *block.Header, txs []*Transaction, receipts []*Receipt, outcxs []*CXReceipt, incxs []*CXReceiptsProof) *Block {
	b := &Block{header: CopyHeader(header)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.header.SetTxHash(EmptyRootHash)
	} else {
		b.header.SetTxHash(DeriveSha(Transactions(txs)))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.header.SetReceiptHash(EmptyRootHash)
	} else {
		b.header.SetReceiptHash(DeriveSha(Receipts(receipts)))
		b.header.SetBloom(CreateBloom(receipts))
	}

	b.header.SetOutgoingReceiptHash(DeriveMultipleShardsSha(CXReceipts(outcxs)))

	if len(incxs) == 0 {
		b.header.SetIncomingReceiptHash(EmptyRootHash)
	} else {
		b.header.SetIncomingReceiptHash(DeriveSha(CXReceiptsProofs(incxs)))
		b.incomingReceipts = make(CXReceiptsProofs, len(incxs))
		copy(b.incomingReceipts, incxs)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewBlockWithHeader(header *block.Header) *Block {
	return &Block{header: CopyHeader(header)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
// TODO ek – no longer necessary
func CopyHeader(h *block.Header) *block.Header {
	cpy := *h
	cpy.Header = cpy.Header.Copy()
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var eb extblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.uncles, b.transactions, b.incomingReceipts = eb.Header, eb.Uncles, eb.Txs, eb.IncomingReceipts
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extblock{
		Header:           b.header,
		Txs:              b.transactions,
		Uncles:           b.uncles,
		IncomingReceipts: b.incomingReceipts,
	})
}

// Uncles return uncles.
func (b *Block) Uncles() []*block.Header {
	return b.uncles
}

// Transactions returns transactions.
func (b *Block) Transactions() Transactions {
	return b.transactions
}

// IncomingReceipts returns verified outgoing receipts
func (b *Block) IncomingReceipts() CXReceiptsProofs {
	return b.incomingReceipts
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
func (b *Block) Number() *big.Int { return b.header.Number() }

// GasLimit returns header gas limit.
func (b *Block) GasLimit() uint64 { return b.header.GasLimit() }

// GasUsed returns header gas used.
func (b *Block) GasUsed() uint64 { return b.header.GasUsed() }

// Time is header time.
func (b *Block) Time() *big.Int { return b.header.Time() }

// NumberU64 is the header number in uint64.
func (b *Block) NumberU64() uint64 { return b.header.Number().Uint64() }

// MixDigest is the header mix digest.
func (b *Block) MixDigest() common.Hash { return b.header.MixDigest() }

// ShardID is the header ShardID
func (b *Block) ShardID() uint32 { return b.header.ShardID() }

// Epoch is the header Epoch
func (b *Block) Epoch() *big.Int { return b.header.Epoch() }

// Bloom returns header bloom.
func (b *Block) Bloom() ethtypes.Bloom { return b.header.Bloom() }

// Coinbase returns header coinbase.
func (b *Block) Coinbase() common.Address { return b.header.Coinbase() }

// Root returns header root.
func (b *Block) Root() common.Hash { return b.header.Root() }

// ParentHash return header parent hash.
func (b *Block) ParentHash() common.Hash { return b.header.ParentHash() }

// TxHash returns header tx hash.
func (b *Block) TxHash() common.Hash { return b.header.TxHash() }

// ReceiptHash returns header receipt hash.
func (b *Block) ReceiptHash() common.Hash { return b.header.ReceiptHash() }

// OutgoingReceiptHash returns header cross shard receipt hash.
func (b *Block) OutgoingReceiptHash() common.Hash { return b.header.OutgoingReceiptHash() }

// Extra returns header extra.
func (b *Block) Extra() []byte { return b.header.Extra() }

// Header returns a copy of Header.
func (b *Block) Header() *block.Header { return CopyHeader(b.header) }

// Body returns the non-header content of the block.
func (b *Block) Body() *Body { return &Body{b.transactions, b.uncles, b.incomingReceipts} }

// Vdf returns header Vdf.
func (b *Block) Vdf() []byte { return b.header.Vdf() }

// Vrf returns header Vrf.
func (b *Block) Vrf() []byte { return b.header.Vrf() }

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
func CalcUncleHash(uncles []*block.Header) common.Hash {
	return hash.FromRLP(uncles)
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *Block) WithSeal(header *block.Header) *Block {
	cpy := *header

	return &Block{
		header:       &cpy,
		transactions: b.transactions,
		uncles:       b.uncles,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, uncles []*block.Header, incomingReceipts CXReceiptsProofs) *Block {
	block := &Block{
		header:           CopyHeader(b.header),
		transactions:     make([]*Transaction, len(transactions)),
		uncles:           make([]*block.Header, len(uncles)),
		incomingReceipts: make([]*CXReceiptsProof, len(incomingReceipts)),
	}
	copy(block.transactions, transactions)
	copy(block.incomingReceipts, incomingReceipts)
	for i := range uncles {
		block.uncles[i] = CopyHeader(uncles[i])
	}
	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) Hash() common.Hash {
	//if hash := b.hash.Load(); hash != nil {
	//	return hash.(common.Hash)
	//}
	// b.Logger(utils.Logger()).Debug().Msg("finalizing and caching block hash")
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
	return b1.header.Number().Cmp(b2.header.Number()) < 0
}

// AddVrf add vrf into block header
func (b *Block) AddVrf(vrf []byte) {
	b.header.SetVrf(vrf)
}

// AddVdf add vdf into block header
func (b *Block) AddVdf(vdf []byte) {
	b.header.SetVdf(vdf)
}

// AddShardState add shardState into block header
func (b *Block) AddShardState(shardState shard.State) error {
	// Make a copy because State.Hash() internally sorts entries.
	// Store the sorted copy.
	shardState = append(shardState[:0:0], shardState...)
	b.header.SetShardStateHash(shardState.Hash())
	data, err := rlp.EncodeToBytes(shardState)
	if err != nil {
		return err
	}
	b.header.SetShardState(data)
	return nil
}

// Logger returns a sub-logger with block contexts added.
func (b *Block) Logger(logger *zerolog.Logger) *zerolog.Logger {
	return b.header.Logger(logger)
}
