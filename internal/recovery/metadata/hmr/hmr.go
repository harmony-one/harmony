// Package hmr implements the byte-exact HMR1 container (plan §4.5), the
// pre-registered digest definitions, and the timestamp-free canonical
// reference manifest whose SHA-256 is THE reference digest B4/D consumers
// bind.
//
// HMR1 container:
//
//	magic 4B ASCII "HMR1" · format-version u32BE = 1 · anchor-digest 32B =
//	SHA-256 of the anchor config file bytes · record-count u64BE · then
//	records in strictly increasing raw-key order (bytewise), each:
//	key-length u32BE ‖ raw-key ‖ value-length u64BE ‖ raw-value
//
// The decoder rejects duplicate keys, non-monotone order, trailing bytes,
// truncation, and count/header disagreement — each with a distinct error
// class. Encoding a NormalizedSet is byte-reproducible: B4 apply re-derives
// locally, serializes canonically in memory and compares digests; it never
// installs .hmr records.
package hmr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
)

// Magic is the 4-byte container magic.
var Magic = [4]byte{'H', 'M', 'R', '1'}

// FormatVersion is the only supported version.
const FormatVersion uint32 = 1

// Distinct decoder error classes (plan WS3).
var (
	ErrBadMagic      = errors.New("hmr: bad magic")
	ErrBadVersion    = errors.New("hmr: unsupported format version")
	ErrTruncated     = errors.New("hmr: truncated container")
	ErrTrailingBytes = errors.New("hmr: trailing bytes after last record")
	ErrOutOfOrder    = errors.New("hmr: record keys not strictly increasing")
	ErrDuplicateKey  = errors.New("hmr: duplicate record key")
	ErrCountMismatch = errors.New("hmr: record count disagrees with header")
	ErrOversized     = errors.New("hmr: implausible record length")
)

// maxComponent guards decode allocations (1 GiB per component).
const maxComponent = 1 << 30

// Records flattens a NormalizedSet into the container's flat ordered
// record set (sections are assigned by key shape; the container stays a
// flat set). Records with nil values (placeholders from refused runs) are
// rejected.
func Records(set *norm.NormalizedSet) ([]norm.Record, error) {
	out := make([]norm.Record, 0, 3+len(set.DVL)+len(set.Snapshots))
	out = append(out, set.ValidatorList)
	out = append(out, set.DVL...)
	out = append(out, set.Snapshots...)
	out = append(out, set.ShardState)
	out = append(out, set.RewardAccumulator)
	for _, r := range out {
		if len(r.Key) == 0 || r.Value == nil {
			return nil, fmt.Errorf("hmr: refusing to encode incomplete normalized set (key %x has nil value)", r.Key)
		}
	}
	sortByKey(out)
	for i := 1; i < len(out); i++ {
		if bytes.Equal(out[i-1].Key, out[i].Key) {
			return nil, fmt.Errorf("%w: %x", ErrDuplicateKey, out[i].Key)
		}
	}
	return out, nil
}

func sortByKey(rs []norm.Record) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && bytes.Compare(rs[j].Key, rs[j-1].Key) < 0; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// Encode serializes the normalized set with the anchor-config digest bound
// into the header. Deterministic and byte-reproducible.
func Encode(set *norm.NormalizedSet, anchorSHA [32]byte) ([]byte, error) {
	records, err := Records(set)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.Write(Magic[:])
	var v4 [4]byte
	binary.BigEndian.PutUint32(v4[:], FormatVersion)
	buf.Write(v4[:])
	buf.Write(anchorSHA[:])
	var c8 [8]byte
	binary.BigEndian.PutUint64(c8[:], uint64(len(records)))
	buf.Write(c8[:])
	for _, r := range records {
		buf.Write(norm.FrameRecord(r.Key, r.Value))
	}
	return buf.Bytes(), nil
}

// Decoded is the parsed container.
type Decoded struct {
	AnchorSHA [32]byte
	Records   []norm.Record
}

// Decode strictly parses container bytes.
func Decode(data []byte) (*Decoded, error) {
	r := bytes.NewReader(data)
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, ErrTruncated
	}
	if magic != Magic {
		return nil, fmt.Errorf("%w: %x", ErrBadMagic, magic)
	}
	var v4 [4]byte
	if _, err := io.ReadFull(r, v4[:]); err != nil {
		return nil, ErrTruncated
	}
	if version := binary.BigEndian.Uint32(v4[:]); version != FormatVersion {
		return nil, fmt.Errorf("%w: %d", ErrBadVersion, version)
	}
	out := &Decoded{}
	if _, err := io.ReadFull(r, out.AnchorSHA[:]); err != nil {
		return nil, ErrTruncated
	}
	var c8 [8]byte
	if _, err := io.ReadFull(r, c8[:]); err != nil {
		return nil, ErrTruncated
	}
	count := binary.BigEndian.Uint64(c8[:])

	var prev []byte
	for i := uint64(0); i < count; i++ {
		var k4 [4]byte
		if _, err := io.ReadFull(r, k4[:]); err != nil {
			if err == io.EOF {
				// Clean EOF at a record boundary: the header promised more
				// records than the body holds.
				return nil, fmt.Errorf("%w: header %d, body %d", ErrCountMismatch, count, i)
			}
			return nil, ErrTruncated
		}
		klen := binary.BigEndian.Uint32(k4[:])
		if klen == 0 || klen > maxComponent {
			return nil, fmt.Errorf("%w: key length %d", ErrOversized, klen)
		}
		key := make([]byte, klen)
		if _, err := io.ReadFull(r, key); err != nil {
			return nil, ErrTruncated
		}
		var v8 [8]byte
		if _, err := io.ReadFull(r, v8[:]); err != nil {
			return nil, ErrTruncated
		}
		vlen := binary.BigEndian.Uint64(v8[:])
		if vlen > maxComponent {
			return nil, fmt.Errorf("%w: value length %d", ErrOversized, vlen)
		}
		value := make([]byte, vlen)
		if _, err := io.ReadFull(r, value); err != nil {
			return nil, ErrTruncated
		}
		if prev != nil {
			switch c := bytes.Compare(prev, key); {
			case c == 0:
				return nil, fmt.Errorf("%w: %x", ErrDuplicateKey, key)
			case c > 0:
				return nil, fmt.Errorf("%w: %x after %x", ErrOutOfOrder, key, prev)
			}
		}
		prev = key
		out.Records = append(out.Records, norm.Record{Key: key, Value: value})
	}
	// Anything left after the promised record count is malformed: either
	// trailing garbage or more records than the header claims.
	if r.Len() != 0 {
		return nil, ErrTrailingBytes
	}
	return out, nil
}
