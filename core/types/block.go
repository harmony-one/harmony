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
	"fmt"
	"io"
	"math/big"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	v0 "github.com/harmony-one/harmony/block/v0"
	v1 "github.com/harmony-one/harmony/block/v1"
	v2 "github.com/harmony-one/harmony/block/v2"
	v3 "github.com/harmony-one/harmony/block/v3"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/taggedrlp"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

const (
	blockV0 = "" // same as taggedrlp.LegacyTag
	blockV1 = "v1"
	blockV2 = "v2"
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

// BodyInterface is a simple accessor interface for block body.
type BodyInterface interface {
	// Transactions returns a deep copy the list of transactions in this block.
	Transactions() []*Transaction

	// StakingTransactions returns a deep copy of staking transactions
	StakingTransactions() []*staking.StakingTransaction

	// TransactionAt returns the transaction at the given index in this block.
	// It returns nil if index is out of bounds.
	TransactionAt(index int) *Transaction

	// StakingTransactionAt returns the staking transaction at the given index in this block.
	// It returns nil if index is out of bounds.
	StakingTransactionAt(index int) *staking.StakingTransaction

	// CXReceiptAt returns the CXReceipt given index (calculated from IncomingReceipts)
	// It returns nil if index is out of bounds
	CXReceiptAt(index int) *CXReceipt

	// SetTransactions sets the list of transactions with a deep copy of the
	// given list.
	SetTransactions(newTransactions []*Transaction)

	// SetStakingTransactions sets the list of staking transactions with a deep copy of the
	// given list.
	SetStakingTransactions(newStakingTransactions []*staking.StakingTransaction)

	// Uncles returns a deep copy of the list of uncle headers of this block.
	Uncles() []*block.Header

	// SetUncles sets the list of uncle headers with a deep copy of the given
	// list.
	SetUncles(newUncle []*block.Header)

	// IncomingReceipts returns a deep copy of the list of incoming cross-shard
	// transaction receipts of this block.
	IncomingReceipts() CXReceiptsProofs

	// SetIncomingReceipts sets the list of incoming cross-shard transaction
	// receipts of this block with a dep copy of the given list.
	SetIncomingReceipts(newIncomingReceipts CXReceiptsProofs)
}

// Body is a simple (mutable, non-safe) data container for storing and moving
// a block's data contents (transactions and uncles) together.
type Body struct {
	BodyInterface
}

// NewBodyForMatchingHeader returns a new block body struct whose implementation
// matches the version of the given field.
//
// TODO ek – this is a stopgap, and works only while there is a N:1 mapping
//
//	between header and body versions.  Replace usage with factory.
func NewBodyForMatchingHeader(h *block.Header) (*Body, error) {
	var bi BodyInterface
	switch h.Header.(type) {
	case *v3.Header:
		bi = new(BodyV2)
	case *v2.Header, *v1.Header:
		bi = new(BodyV1)
	case *v0.Header:
		bi = new(BodyV0)
	default:
		return nil, errors.Errorf("unsupported header type %s",
			taggedrlp.TypeName(reflect.TypeOf(h)))
	}
	return &Body{bi}, nil
}

// NewTestBody creates a new, empty body object for epoch 0 using the test
// factory.  Use for unit tests.
func NewTestBody() *Body {
	block := new(Block)
	block.header = blockfactory.NewTestHeader()
	body, err := NewBodyForMatchingHeader(blockfactory.NewTestHeader())
	if err != nil {
		panic(err)
	}
	return body
}

// With returns a field setter context for the receiver.
func (b *Body) With() BodyFieldSetter {
	return BodyFieldSetter{b}
}

// EncodeRLP RLP-encodes the block body onto the given writer.  It uses tagged RLP
// encoding for non-Genesis body formats.
func (b *Body) EncodeRLP(w io.Writer) error {
	return BodyRegistry.Encode(w, b.BodyInterface)
}

// DecodeRLP decodes a block body out of the given RLP stream into the receiver.
// It uses tagged RLP encoding for non-Genesis body formats.
func (b *Body) DecodeRLP(s *rlp.Stream) error {
	decoded, err := BodyRegistry.Decode(s)
	if err != nil {
		return err
	}
	bif, ok := decoded.(BodyInterface)
	if !ok {
		return errors.Errorf(
			"decoded body (type %s) does not implement BodyInterface",
			taggedrlp.TypeName(reflect.TypeOf(decoded)))
	}
	b.BodyInterface = bif
	return nil
}

// BodyRegistry is the tagged RLP registry for block body types.
var BodyRegistry = taggedrlp.NewRegistry()

func init() {
	BodyRegistry.MustRegister(taggedrlp.LegacyTag, new(BodyV0))
	BodyRegistry.MustRegister(blockV1, new(BodyV1))
	BodyRegistry.MustRegister(blockV2, new(BodyV2))
}

// Block represents an entire block in the Harmony blockchain.
type Block struct {
	header              *block.Header
	uncles              []*block.Header
	transactions        Transactions
	stakingTransactions staking.StakingTransactions
	incomingReceipts    CXReceiptsProofs

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

	commitLock sync.Mutex
	// Commit Signatures/Bitmap
	commitSigAndBitmap []byte
}

func (b *Block) String() string {
	m := b.Header()
	return fmt.Sprintf(
		"[ViewID:%d Num:%d BlockHash:%s]",
		m.ViewID(),
		m.Number(),
		m.Hash().Hex(),
	)

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

// SetCurrentCommitSig sets the commit group signature that signed on this block.
func (b *Block) SetCurrentCommitSig(sigAndBitmap []byte) {
	if len(sigAndBitmap) <= 96 {
		utils.Logger().Warn().
			Int("srcLen", len(sigAndBitmap)).
			Int("dstLen", len(b.header.LastCommitSignature())).
			Msg("SetCurrentCommitSig: sig size mismatch")
	}
	b.commitLock.Lock()
	b.commitSigAndBitmap = sigAndBitmap
	b.commitLock.Unlock()
}

// GetCurrentCommitSig get the commit group signature that signed on this block.
func (b *Block) GetCurrentCommitSig() []byte {
	b.commitLock.Lock()
	defer b.commitLock.Unlock()
	return b.commitSigAndBitmap
}

// DeprecatedTd is an old relic for extracting the TD of a block. It is in the
// code solely to facilitate upgrading the database from the old format to the
// new, after which it should be deleted. Do not use!
func (b *Block) DeprecatedTd() *big.Int {
	return b.td
}

// "external" block encoding. used for eth protocol, etc.
type extblock struct {
	Header *block.Header
	Txs    []*Transaction
	Uncles []*block.Header
}

// CX-ready extblock
type extblockV1 struct {
	Header           *block.Header
	Txs              []*Transaction
	Uncles           []*block.Header
	IncomingReceipts CXReceiptsProofs
}

// includes staking transaction
type extblockV2 struct {
	Header           *block.Header
	Txs              []*Transaction
	Stks             []*staking.StakingTransaction
	Uncles           []*block.Header
	IncomingReceipts CXReceiptsProofs
}

var onceBlockReg sync.Once
var extblockReg *taggedrlp.Registry

func blockRegistry() *taggedrlp.Registry {
	onceBlockReg.Do(func() {
		extblockReg = taggedrlp.NewRegistry()
		extblockReg.MustRegister(taggedrlp.LegacyTag, &extblock{})
		extblockReg.MustRegister(blockV1, &extblockV1{})
		extblockReg.MustRegister(blockV2, &extblockV2{})
	})

	return extblockReg
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs,
// and receipts.
func NewBlock(
	header *block.Header, txs []*Transaction,
	receipts []*Receipt, outcxs []*CXReceipt, incxs []*CXReceiptsProof,
	stks []*staking.StakingTransaction) *Block {

	b := &Block{header: CopyHeader(header)}

	if len(receipts) != len(txs)+len(stks) {
		utils.Logger().Error().
			Int("receiptsLen", len(receipts)).
			Int("txnsLen", len(txs)).
			Int("stakingTxnsLen", len(stks)).
			Msg("Length of receipts doesn't match length of transactions")
		return nil
	}

	// Put transactions into block
	if len(txs) == 0 && len(stks) == 0 {
		b.header.SetTxHash(EmptyRootHash)
	} else {
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)

		b.stakingTransactions = make(staking.StakingTransactions, len(stks))
		copy(b.stakingTransactions, stks)

		b.header.SetTxHash(DeriveSha(
			Transactions(txs),
			staking.StakingTransactions(stks),
		))
	}

	// Put receipts into block
	if len(receipts) == 0 {
		b.header.SetReceiptHash(EmptyRootHash)
	} else {
		b.header.SetReceiptHash(DeriveSha(Receipts(receipts)))
		b.header.SetBloom(CreateBloom(receipts))
	}

	// Put cross-shard receipts (ingres/egress) into block
	b.header.SetOutgoingReceiptHash(CXReceipts(outcxs).ComputeMerkleRoot())
	if len(incxs) == 0 {
		b.header.SetIncomingReceiptHash(EmptyRootHash)
	} else {
		b.header.SetIncomingReceiptHash(DeriveSha(CXReceiptsProofs(incxs)))
		b.incomingReceipts = make(CXReceiptsProofs, len(incxs))
		copy(b.incomingReceipts, incxs)
	}

	// Great! Block is finally finalized.
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
func CopyHeader(h *block.Header) *block.Header {
	if h == nil {
		return nil
	}
	cpy := *h
	cpy.Header = cpy.Header.Copy()
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	registry := blockRegistry()
	eb, err := registry.Decode(s)
	if err != nil {
		return err
	}
	switch eb := eb.(type) {
	case *extblockV2:
		b.header, b.uncles, b.transactions, b.incomingReceipts, b.stakingTransactions = eb.Header, eb.Uncles, eb.Txs, eb.IncomingReceipts, eb.Stks
	case *extblockV1:
		b.header, b.uncles, b.transactions, b.incomingReceipts = eb.Header, eb.Uncles, eb.Txs, eb.IncomingReceipts
	case *extblock:
		b.header, b.uncles, b.transactions, b.incomingReceipts = eb.Header, eb.Uncles, eb.Txs, nil
	default:
		return errors.Errorf("unknown extblock type %s", taggedrlp.TypeName(reflect.TypeOf(eb)))
	}
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *Block) EncodeRLP(w io.Writer) error {
	var eb interface{}

	switch h := b.header.Header.(type) {
	case *v3.Header:
		eb = extblockV2{b.header, b.transactions, b.stakingTransactions, b.uncles, b.incomingReceipts}
	case *v2.Header, *v1.Header:
		eb = extblockV1{b.header, b.transactions, b.uncles, b.incomingReceipts}
	case *v0.Header:
		if len(b.incomingReceipts) > 0 {
			return errors.New("incomingReceipts unsupported in v0 block")
		}
		eb = extblock{b.header, b.transactions, b.uncles}
	default:
		return errors.Errorf("unsupported block header type %s",
			taggedrlp.TypeName(reflect.TypeOf(h)))
	}

	extblockReg := blockRegistry()
	return extblockReg.Encode(w, eb)
}

// Uncles return uncles.
func (b *Block) Uncles() []*block.Header {
	return b.uncles
}

// IsLastBlockInEpoch returns if its the last block of the epoch.
func (b *Block) IsLastBlockInEpoch() bool {
	return b.header.IsLastBlockInEpoch()
}

// Transactions returns transactions.
func (b *Block) Transactions() Transactions {
	return b.transactions
}

// StakingTransactions returns stakingTransactions.
func (b *Block) StakingTransactions() staking.StakingTransactions {
	return b.stakingTransactions
}

// IncomingReceipts returns verified outgoing receipts
func (b *Block) IncomingReceipts() CXReceiptsProofs {
	return b.incomingReceipts
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
func (b *Block) Body() *Body {
	body, err := NewBodyForMatchingHeader(b.header)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("cannot create block Body struct")
		return nil
	}
	return body.With().
		Transactions(b.transactions).
		StakingTransactions(b.stakingTransactions).
		Uncles(b.uncles).
		IncomingReceipts(b.incomingReceipts).
		Body()
}

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

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(transactions []*Transaction, stakingTxns []*staking.StakingTransaction, uncles []*block.Header, incomingReceipts CXReceiptsProofs) *Block {
	block := &Block{
		header:              CopyHeader(b.header),
		transactions:        make([]*Transaction, len(transactions)),
		stakingTransactions: make([]*staking.StakingTransaction, len(stakingTxns)),
		uncles:              make([]*block.Header, len(uncles)),
		incomingReceipts:    make([]*CXReceiptsProof, len(incomingReceipts)),
	}
	copy(block.transactions, transactions)
	copy(block.stakingTransactions, stakingTxns)
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

// Logger returns a sub-logger with block contexts added.
func (b *Block) Logger(logger *zerolog.Logger) *zerolog.Logger {
	return b.header.Logger(logger)
}
