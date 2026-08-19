package bundle

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
	"github.com/harmony-one/harmony/internal/recoverydb/keys"
	"github.com/harmony-one/harmony/internal/recoverydb/report"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

// DefaultChunkBytes is the default chunk size (512 MiB, plan WS3).
const DefaultChunkBytes = int64(512 * 1024 * 1024)

// ExportConfig parameterizes export and preflight.
type ExportConfig struct {
	Network     string
	ShardID     uint32
	ChainConfig *params.ChainConfig

	FromHeight      uint64
	ToHeight        uint64
	CertChildHeight uint64
	BaselineHeight  uint64
	BaselineHash    common.Hash

	Anchor *anchor.Manifest // optional; adds pinned-hash assertions

	OutputDir   string
	ChunkBytes  int64
	Donor       string
	ToolVersion string
	Inputs      []integrity.InputRef
}

// PreflightReport is the --report-only output (plan WS3 donor preflight).
type PreflightReport struct {
	report.Meta

	Donor           string `json:"donor"`
	FromHeight      uint64 `json:"from_height"`
	ToHeight        uint64 `json:"to_height"`
	CertChildHeight uint64 `json:"cert_child_height"`

	CanonicalPresent bool     `json:"canonical_present"`
	HeadersPresent   bool     `json:"headers_present"`
	BodiesPresent    bool     `json:"bodies_present"`
	CertsPresent     bool     `json:"certificates_present"`
	CertChildPresent bool     `json:"cert_child_present"`
	ChainWalkOK      bool     `json:"chain_walk_ok"`
	TargetHashOK     bool     `json:"target_hash_ok"`
	TargetHash       string   `json:"target_hash"`
	Gaps             []string `json:"gaps,omitempty"` // first examples, block-accurate
	GapCount         uint64   `json:"gap_count"`

	Passed bool `json:"passed"`
}

func (cfg *ExportConfig) validate() error {
	if cfg.FromHeight > cfg.ToHeight {
		return fmt.Errorf("bundle: --from-height %d above --to-height %d", cfg.FromHeight, cfg.ToHeight)
	}
	if cfg.CertChildHeight != cfg.ToHeight+1 {
		return fmt.Errorf("bundle: --certificate-child-height %d must equal --to-height+1 (%d)", cfg.CertChildHeight, cfg.ToHeight+1)
	}
	if cfg.FromHeight != cfg.BaselineHeight+1 {
		return fmt.Errorf("bundle: --from-height %d disagrees with baseline manifest head %d (+1)", cfg.FromHeight, cfg.BaselineHeight)
	}
	if cfg.Anchor != nil {
		if cfg.ToHeight != cfg.Anchor.TargetHeight {
			return fmt.Errorf("bundle: --to-height %d disagrees with anchor target %d", cfg.ToHeight, cfg.Anchor.TargetHeight)
		}
	}
	if cfg.ChunkBytes <= 0 {
		cfg.ChunkBytes = DefaultChunkBytes
	}
	return nil
}

// Preflight runs the mechanical donor survey: presence over the whole range
// [from, certChild] and the parent-hash chain walk terminating at the pinned
// target hash (plan WS3). A gapped donor is refused by Export.
func Preflight(db ethdb.Database, cfg ExportConfig) (*PreflightReport, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	meta, err := report.NewMeta(ManifestSchemaV1, "export-bundle --report-only", cfg.Network, cfg.ShardID, cfg.ToolVersion, cfg.Inputs)
	if err != nil {
		return nil, err
	}
	rep := &PreflightReport{
		Meta:  meta,
		Donor: cfg.Donor, FromHeight: cfg.FromHeight, ToHeight: cfg.ToHeight, CertChildHeight: cfg.CertChildHeight,
		CanonicalPresent: true, HeadersPresent: true, BodiesPresent: true, CertsPresent: true,
	}
	addGap := func(format string, args ...interface{}) {
		rep.GapCount++
		if len(rep.Gaps) < 20 {
			rep.Gaps = append(rep.Gaps, fmt.Sprintf(format, args...))
		}
	}

	for n := cfg.FromHeight; n <= cfg.CertChildHeight; n++ {
		ch := rawdb.ReadCanonicalHash(db, n)
		if ch == (common.Hash{}) {
			rep.CanonicalPresent = false
			addGap("canonical hash missing at %d", n)
			continue
		}
		hdr := rawdb.ReadHeader(db, ch, n)
		if hdr == nil {
			rep.HeadersPresent = false
			addGap("header missing at %d", n)
			continue
		}
		if n <= cfg.ToHeight {
			if body := rawdb.ReadBody(db, ch, n); body == nil {
				rep.BodiesPresent = false
				addGap("body missing at %d", n)
			}
		}
		// Certificate shape (round 13 finding 13): each header at height
		// n > from carries the quorum certificate for block n-1 in its
		// last-commit fields; a zero signature or empty bitmap means Export
		// would fail later, so refuse the donor mechanically here.
		if n > cfg.FromHeight {
			if sig := hdr.LastCommitSignature(); sig == ([96]byte{}) {
				rep.CertsPresent = false
				addGap("zero last-commit signature in header %d (certificate for block %d)", n, n-1)
			}
			if len(hdr.LastCommitBitmap()) == 0 {
				rep.CertsPresent = false
				addGap("empty last-commit bitmap in header %d (certificate for block %d)", n, n-1)
			}
		}
	}

	childHash := rawdb.ReadCanonicalHash(db, cfg.CertChildHeight)
	childHdr := rawdb.ReadHeader(db, childHash, cfg.CertChildHeight)
	rep.CertChildPresent = childHdr != nil

	// Hash-chain walk from the certificate child down to from-height.
	rep.ChainWalkOK = true
	if childHdr != nil {
		cur := childHdr
		for n := cfg.CertChildHeight; n > cfg.FromHeight; n-- {
			parent := cur.ParentHash()
			ch := rawdb.ReadCanonicalHash(db, n-1)
			if ch != parent {
				rep.ChainWalkOK = false
				addGap("hash chain break at %d: parent %s, canonical %s", n-1, parent.Hex(), ch.Hex())
				break
			}
			hdr := rawdb.ReadHeader(db, ch, n-1)
			if hdr == nil {
				rep.ChainWalkOK = false
				addGap("header missing during chain walk at %d", n-1)
				break
			}
			cur = hdr
		}
		targetHash := childHdr.ParentHash()
		rep.TargetHash = targetHash.Hex()
		rep.TargetHashOK = true
		if cfg.Anchor != nil && targetHash != cfg.Anchor.TargetHash {
			rep.TargetHashOK = false
			addGap("entry at %d is %s, pinned target hash is %s", cfg.ToHeight, targetHash.Hex(), cfg.Anchor.TargetHash.Hex())
		}
	}

	rep.Passed = rep.CanonicalPresent && rep.HeadersPresent && rep.BodiesPresent &&
		rep.CertsPresent && rep.CertChildPresent && rep.ChainWalkOK && rep.TargetHashOK &&
		rep.GapCount == 0
	return rep, nil
}

// Export runs the full single-donor export (plan WS3): per-block certificate
// extraction from child headers, BLS verification against the donor's own
// committee records, chunked frames, sidecar, manifest.
func Export(db ethdb.Database, cfg ExportConfig) (*Manifest, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	pre, err := Preflight(db, cfg)
	if err != nil {
		return nil, err
	}
	if !pre.Passed {
		return nil, fmt.Errorf("bundle: donor preflight failed (%d gaps; first: %v)", pre.GapCount, pre.Gaps)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("bundle: create output dir: %w", err)
	}
	entries, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		return nil, fmt.Errorf("bundle: output dir %s is not empty", cfg.OutputDir)
	}

	cv := verify.NewCertVerifier(db, cfg.ChainConfig, cfg.ShardID)
	orderedH := report.NewHasher("bundle.orderedHashes")

	var (
		chunks    []ChunkInfo
		chunkIdx  int
		chunkFile *os.File
		chunkW    *bufio.Writer
		chunkInfo ChunkInfo
	)
	openChunk := func() error {
		chunkInfo = ChunkInfo{Name: ChunkName(chunkIdx), FirstHeight: 0}
		f, err := os.OpenFile(filepath.Join(cfg.OutputDir, chunkInfo.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("bundle: create chunk: %w", err)
		}
		chunkFile = f
		chunkW = bufio.NewWriterSize(f, 1<<20)
		return nil
	}
	closeChunk := func() error {
		if chunkFile == nil {
			return nil
		}
		if err := chunkW.Flush(); err != nil {
			return fmt.Errorf("bundle: flush chunk: %w", err)
		}
		if err := chunkFile.Sync(); err != nil {
			return fmt.Errorf("bundle: fsync chunk: %w", err)
		}
		if err := chunkFile.Close(); err != nil {
			return fmt.Errorf("bundle: close chunk: %w", err)
		}
		sum, err := integrity.FileSHA256(filepath.Join(cfg.OutputDir, chunkInfo.Name))
		if err != nil {
			return err
		}
		chunkInfo.SHA256 = sum
		chunks = append(chunks, chunkInfo)
		chunkFile, chunkW = nil, nil
		chunkIdx++
		return nil
	}
	if err := openChunk(); err != nil {
		return nil, err
	}

	for n := cfg.FromHeight; n <= cfg.ToHeight; n++ {
		ch := rawdb.ReadCanonicalHash(db, n)
		block := rawdb.ReadBlock(db, ch, n)
		if block == nil {
			return nil, fmt.Errorf("bundle: block %d unreadable despite preflight", n)
		}
		childHash := rawdb.ReadCanonicalHash(db, n+1)
		childHdr := rawdb.ReadHeader(db, childHash, n+1)
		if childHdr == nil {
			return nil, fmt.Errorf("bundle: child header %d unreadable despite preflight", n+1)
		}
		if childHdr.ParentHash() != ch {
			return nil, fmt.Errorf("bundle: child %d does not extend block %d", n+1, n)
		}
		sig := childHdr.LastCommitSignature()
		bitmap := childHdr.LastCommitBitmap()
		// Verify the certificate against the donor's committee before
		// writing (plan WS3).
		if err := cv.VerifyHeaderCert(block.Header(), sig, bitmap); err != nil {
			return nil, fmt.Errorf("bundle: block %d: %w", n, err)
		}
		sigAndBitmap := append(sig[:], bitmap...)

		// Donor's exact raw block-sig-N, informational only.
		var donorSig []byte
		if val, err := db.Get(keys.BlockSigKey(n)); err == nil {
			donorSig = val
		}

		rec, err := NewRecord(cfg.Network, cfg.ShardID, block, sigAndBitmap, donorSig)
		if err != nil {
			return nil, err
		}
		written, err := WriteFrame(chunkW, rec)
		if err != nil {
			return nil, err
		}
		orderedH.Add(block.Hash().Bytes())
		if chunkInfo.Records == 0 {
			chunkInfo.FirstHeight = n
		}
		chunkInfo.Records++
		chunkInfo.LastHeight = n
		chunkInfo.Bytes += uint64(written)
		if int64(chunkInfo.Bytes) >= cfg.ChunkBytes && n < cfg.ToHeight {
			if err := closeChunk(); err != nil {
				return nil, err
			}
			if err := openChunk(); err != nil {
				return nil, err
			}
		}
	}
	if err := closeChunk(); err != nil {
		return nil, err
	}

	// Sidecar: the raw RLP header of the certificate child.
	childHash := rawdb.ReadCanonicalHash(db, cfg.CertChildHeight)
	childHdr := rawdb.ReadHeader(db, childHash, cfg.CertChildHeight)
	if childHdr == nil {
		return nil, fmt.Errorf("bundle: certificate child %d vanished", cfg.CertChildHeight)
	}
	if cfg.Anchor != nil {
		if childHdr.ParentHash() != cfg.Anchor.TargetHash {
			return nil, fmt.Errorf("bundle: sidecar parent hash %s != pinned target %s", childHdr.ParentHash().Hex(), cfg.Anchor.TargetHash.Hex())
		}
		if childHdr.Hash() != cfg.Anchor.AbandonedChildHash {
			return nil, fmt.Errorf("bundle: sidecar header hash %s != ABANDONED_CHILD_HASH %s", childHdr.Hash().Hex(), cfg.Anchor.AbandonedChildHash.Hex())
		}
	}
	rawHdr, err := rlp.EncodeToBytes(childHdr)
	if err != nil {
		return nil, fmt.Errorf("bundle: encode sidecar header: %w", err)
	}
	sidecarPath := SidecarPath(cfg.OutputDir)
	if err := os.WriteFile(sidecarPath, rawHdr, 0o644); err != nil {
		return nil, fmt.Errorf("bundle: write sidecar: %w", err)
	}
	sidecarSum, err := integrity.FileSHA256(sidecarPath)
	if err != nil {
		return nil, err
	}
	childSig := childHdr.LastCommitSignature()
	childSigAndBitmap := append(childSig[:], childHdr.LastCommitBitmap()...)

	meta, err := report.NewMeta(ManifestSchemaV1, "export-bundle", cfg.Network, cfg.ShardID, cfg.ToolVersion, cfg.Inputs)
	if err != nil {
		return nil, err
	}
	m := &Manifest{
		Meta:              meta,
		BaselineHeight:    cfg.BaselineHeight,
		BaselineHash:      cfg.BaselineHash.Hex(),
		FromHeight:        cfg.FromHeight,
		ToHeight:          cfg.ToHeight,
		TargetHash:        childHdr.ParentHash().Hex(),
		RecordCount:       cfg.ToHeight - cfg.FromHeight + 1,
		OrderedHashDigest: orderedH.Digest().SHA256,
		Chunks:            chunks,
		Sidecar: SidecarInfo{
			Name:        filepath.Base(sidecarPath),
			SHA256:      sidecarSum,
			ChildHeight: cfg.CertChildHeight,
			ChildHash:   childHdr.Hash().Hex(),
			ParentHash:  childHdr.ParentHash().Hex(),
			SigSHA256:   integrity.BytesSHA256(childSigAndBitmap),
		},
		Donor: cfg.Donor,
	}

	// Directory SHA256SUMS over chunks + sidecar; manifest.json carries its
	// own sibling .sha256 (no self-reference).
	sums := []integrity.SumsEntry{{SHA256: sidecarSum, Name: m.Sidecar.Name}}
	for _, c := range chunks {
		sums = append(sums, integrity.SumsEntry{SHA256: c.SHA256, Name: c.Name})
	}
	if err := integrity.WriteSums(SumsPath(cfg.OutputDir), sums); err != nil {
		return nil, err
	}
	if _, err := report.WriteJSON(ManifestPath(cfg.OutputDir), m); err != nil {
		return nil, err
	}
	if err := report.FsyncWalk(report.OSFS, cfg.OutputDir); err != nil {
		return nil, err
	}
	return m, nil
}
