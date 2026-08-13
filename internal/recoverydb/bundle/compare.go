package bundle

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/harmony-one/harmony/internal/recoverydb/report"
)

// CompareResult is compare-bundles' outcome (plan WS3, optional command).
type CompareResult struct {
	report.Meta

	Left  string `json:"left"`
	Right string `json:"right"`

	RecordsCompared     uint64 `json:"records_compared"`
	DonorSigDifferences uint64 `json:"donor_sig_differences"` // informational, never fatal
	Identical           bool   `json:"identical"`
	FirstDifference     string `json:"first_difference,omitempty"`
}

// bundleReader streams records across chunk boundaries in manifest order.
type bundleReader struct {
	dir      string
	manifest *Manifest
	chunkIdx int
	r        *bufio.Reader
	f        *os.File
}

func newBundleReader(dir string) (*bundleReader, error) {
	m, _, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}
	if err := m.VerifyChunks(dir); err != nil {
		return nil, err
	}
	return &bundleReader{dir: dir, manifest: m}, nil
}

func (br *bundleReader) next() (*Record, error) {
	for {
		if br.r == nil {
			if br.chunkIdx >= len(br.manifest.Chunks) {
				return nil, nil // clean end of bundle
			}
			f, err := os.Open(filepath.Join(br.dir, br.manifest.Chunks[br.chunkIdx].Name))
			if err != nil {
				return nil, err
			}
			br.f = f
			br.r = bufio.NewReaderSize(f, 1<<20)
		}
		rec, err := ReadFrame(br.r)
		if err == ErrEndOfChunk {
			br.f.Close()
			br.r, br.f = nil, nil
			br.chunkIdx++
			continue
		}
		if err != nil {
			return nil, err
		}
		return rec, nil
	}
}

func (br *bundleReader) close() {
	if br.f != nil {
		br.f.Close()
	}
}

// Compare byte-compares two bundles record by record. Chain differences are
// fatal; donor-local block-sig differences are counted, not fatal (plan WS3).
func Compare(leftDir, rightDir, network string, shardID uint32, toolVersion string) (*CompareResult, error) {
	meta, err := report.NewMeta(ManifestSchemaV1, "compare-bundles", network, shardID, toolVersion, nil)
	if err != nil {
		return nil, err
	}
	res := &CompareResult{Meta: meta, Left: leftDir, Right: rightDir, Identical: true}

	l, err := newBundleReader(leftDir)
	if err != nil {
		return nil, fmt.Errorf("bundle: left: %w", err)
	}
	defer l.close()
	rr, err := newBundleReader(rightDir)
	if err != nil {
		return nil, fmt.Errorf("bundle: right: %w", err)
	}
	defer rr.close()

	if l.manifest.OrderedHashDigest != rr.manifest.OrderedHashDigest {
		res.Identical = false
		res.FirstDifference = fmt.Sprintf("ordered hash digests differ: %s vs %s",
			l.manifest.OrderedHashDigest, rr.manifest.OrderedHashDigest)
	}

	for {
		lrec, err := l.next()
		if err != nil {
			return nil, fmt.Errorf("bundle: left read: %w", err)
		}
		rrec, err := rr.next()
		if err != nil {
			return nil, fmt.Errorf("bundle: right read: %w", err)
		}
		if lrec == nil && rrec == nil {
			break
		}
		if lrec == nil || rrec == nil {
			res.Identical = false
			res.FirstDifference = "bundles have different record counts"
			break
		}
		res.RecordsCompared++
		// Chain content: everything except the donor-local sig.
		lc, rc := *lrec, *rrec
		lds, rds := lc.DonorBlockSig, rc.DonorBlockSig
		lc.DonorBlockSig, rc.DonorBlockSig = nil, nil
		if !recordsEqual(&lc, &rc) {
			res.Identical = false
			res.FirstDifference = fmt.Sprintf("chain difference at height %d", lrec.Height)
			break
		}
		if !bytes.Equal(lds, rds) {
			res.DonorSigDifferences++
		}
	}
	return res, nil
}

func recordsEqual(a, b *Record) bool {
	return a.Version == b.Version && a.Network == b.Network && a.ShardID == b.ShardID &&
		a.Height == b.Height && a.Hash == b.Hash && a.ParentHash == b.ParentHash &&
		a.Epoch == b.Epoch && a.ViewID == b.ViewID && a.StateRoot == b.StateRoot &&
		a.TxRoot == b.TxRoot && a.ReceiptRoot == b.ReceiptRoot &&
		a.OutgoingReceiptRoot == b.OutgoingReceiptRoot && a.IncomingReceiptRoot == b.IncomingReceiptRoot &&
		bytes.Equal(a.BlockWithSigRLP, b.BlockWithSigRLP)
}
