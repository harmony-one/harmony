// Package bundle implements the export-bundle record codec, chunked bundle
// layout, manifest, single-donor export with mechanical preflight, and the
// optional compare-bundles byte comparator (plan WS3).
package bundle

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
)

// RecordVersion1 is the only record version this build reads or writes.
const RecordVersion1 = uint16(1)

// MaxRecordBytes bounds a single frame (defense against corrupt length
// prefixes; blocks are far below this).
const MaxRecordBytes = 128 * 1024 * 1024

// Record is one exported block: the RLP core.BlockWithSig plus the expected
// identity fields the replayer asserts before decode, and the donor's raw
// exact block-sig-N value (informational only; never merged into the chain —
// the certificate inside BlockWithSigRLP comes from the child header).
type Record struct {
	Version    uint16
	Network    string
	ShardID    uint32
	Height     uint64
	Hash       common.Hash
	ParentHash common.Hash
	Epoch      uint64
	ViewID     uint64
	StateRoot  common.Hash

	TxRoot              common.Hash
	ReceiptRoot         common.Hash
	OutgoingReceiptRoot common.Hash
	IncomingReceiptRoot common.Hash

	BlockWithSigRLP []byte
	DonorBlockSig   []byte
}

// NewRecord builds a Record from a block and its child-carried certificate.
func NewRecord(network string, shardID uint32, block *types.Block, sigAndBitmap, donorSig []byte) (*Record, error) {
	bws := core.BlockWithSig{Block: block, CommitSigAndBitmap: sigAndBitmap}
	raw, err := rlp.EncodeToBytes(bws)
	if err != nil {
		return nil, fmt.Errorf("bundle: encode BlockWithSig %d: %w", block.NumberU64(), err)
	}
	h := block.Header()
	return &Record{
		Version:    RecordVersion1,
		Network:    network,
		ShardID:    shardID,
		Height:     block.NumberU64(),
		Hash:       block.Hash(),
		ParentHash: block.ParentHash(),
		Epoch:      block.Epoch().Uint64(),
		ViewID:     h.ViewID().Uint64(),
		StateRoot:  h.Root(),

		TxRoot:              h.TxHash(),
		ReceiptRoot:         h.ReceiptHash(),
		OutgoingReceiptRoot: h.OutgoingReceiptHash(),
		IncomingReceiptRoot: h.IncomingReceiptHash(),

		BlockWithSigRLP: raw,
		DonorBlockSig:   donorSig,
	}, nil
}

// DecodeBlock decodes and re-verifies the embedded block: the decoded
// block's identity fields must equal the record's expected fields, and the
// commit signature must be populated.
func (r *Record) DecodeBlock() (*types.Block, []byte, error) {
	if r.Version != RecordVersion1 {
		return nil, nil, fmt.Errorf("bundle: unsupported record version %d", r.Version)
	}
	block, err := core.RlpDecodeBlockOrBlockWithSig(r.BlockWithSigRLP)
	if err != nil {
		return nil, nil, fmt.Errorf("bundle: decode block at %d: %w", r.Height, err)
	}
	sig := block.GetCurrentCommitSig()
	if len(sig) == 0 {
		return nil, nil, fmt.Errorf("bundle: record %d carries no commit signature", r.Height)
	}
	h := block.Header()
	checks := []struct {
		name      string
		got, want interface{}
	}{
		{"height", block.NumberU64(), r.Height},
		{"hash", block.Hash(), r.Hash},
		{"parent", block.ParentHash(), r.ParentHash},
		{"shard", block.ShardID(), r.ShardID},
		{"epoch", block.Epoch().Uint64(), r.Epoch},
		{"viewID", h.ViewID().Uint64(), r.ViewID},
		{"stateRoot", h.Root(), r.StateRoot},
		{"txRoot", h.TxHash(), r.TxRoot},
		{"receiptRoot", h.ReceiptHash(), r.ReceiptRoot},
		{"outgoingReceiptRoot", h.OutgoingReceiptHash(), r.OutgoingReceiptRoot},
		{"incomingReceiptRoot", h.IncomingReceiptHash(), r.IncomingReceiptRoot},
	}
	for _, c := range checks {
		if fmt.Sprintf("%v", c.got) != fmt.Sprintf("%v", c.want) {
			return nil, nil, fmt.Errorf("bundle: record %d field %s mismatch: block has %v, record expects %v", r.Height, c.name, c.got, c.want)
		}
	}
	return block, sig, nil
}

// WriteFrame writes uvarint(len) ‖ RLP(record).
func WriteFrame(w io.Writer, rec *Record) (int, error) {
	payload, err := rlp.EncodeToBytes(rec)
	if err != nil {
		return 0, fmt.Errorf("bundle: encode record %d: %w", rec.Height, err)
	}
	var lenbuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(lenbuf[:], uint64(len(payload)))
	if _, err := w.Write(lenbuf[:n]); err != nil {
		return 0, fmt.Errorf("bundle: write frame length: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return 0, fmt.Errorf("bundle: write frame payload: %w", err)
	}
	return n + len(payload), nil
}

// ErrEndOfChunk signals a clean end of a chunk file.
var ErrEndOfChunk = errors.New("bundle: end of chunk")

// ReadFrame reads one frame; io.EOF exactly at a frame boundary returns
// ErrEndOfChunk, anything torn is a distinct truncation error.
func ReadFrame(r *bufio.Reader) (*Record, error) {
	length, err := binary.ReadUvarint(r)
	if err == io.EOF {
		return nil, ErrEndOfChunk
	}
	if err != nil {
		return nil, fmt.Errorf("bundle: read frame length: %w", err)
	}
	if length == 0 || length > MaxRecordBytes {
		return nil, fmt.Errorf("bundle: implausible frame length %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("bundle: truncated frame (want %d bytes): %w", length, err)
	}
	var rec Record
	if err := rlp.DecodeBytes(payload, &rec); err != nil {
		return nil, fmt.Errorf("bundle: undecodable record frame: %w", err)
	}
	if rec.Version != RecordVersion1 {
		return nil, fmt.Errorf("bundle: unsupported record version %d", rec.Version)
	}
	return &rec, nil
}
