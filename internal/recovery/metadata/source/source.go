// Package source opens the strict read-only source DB and assembles the
// norm.Sources every metadata subcommand consumes: anchor resolution +
// schedule install, DB cross-checks, target-state opening (snaps=nil), the
// best-effort historical state opener, and the canonical header reader.
// It is shared plumbing under the scan / export-reference / audit-branch
// commands (plan §4.1 pipeline).
package source

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/recovery/anchor"
	"github.com/harmony-one/harmony/internal/recovery/dbopen"
	"github.com/harmony-one/harmony/internal/recovery/metadata/norm"
)

// Open is the assembled source.
type Open struct {
	Anchor   *anchor.Resolved
	NormA    norm.Anchor
	DB       *dbopen.DB
	KV       ethdb.KeyValueStore // write-refusing
	ChainDB  ethdb.Database      // rawdb wrapper over KV (no freezer)
	Header   *block.Header       // verified target header
	Sources  norm.Sources
	stateDBs []state.Database
}

// OpenSource runs the shared pipeline: layout gate, strict open, anchor DB
// cross-verification, target resolution (canonical mapping == anchor hash,
// H+hash reverse mapping, header hash recomputation, state.New(root) with
// snaps=nil).
func OpenSource(dbPath string, res *anchor.Resolved, opts dbopen.Options) (*Open, error) {
	if err := dbopen.CheckLayout(dbPath, res.Config.Shard); err != nil {
		return nil, err
	}
	db, err := dbopen.OpenStrictReadOnly(dbPath, opts)
	if err != nil {
		return nil, err
	}
	o := &Open{Anchor: res, DB: db, KV: db.KV()}
	o.ChainDB = rawdb.NewDatabase(o.KV)

	if _, err := anchor.VerifyDB(o.KV, res); err != nil {
		db.Close()
		return nil, err
	}
	// Reverse mapping H+hash -> number must agree.
	if num := rawdb.ReadHeaderNumber(o.ChainDB, res.TargetHash); num == nil || *num != res.Config.TargetHeight {
		db.Close()
		return nil, fmt.Errorf("source: header-number reverse mapping for %s is %v, want %d",
			res.Config.TargetHash, num, res.Config.TargetHeight)
	}
	hdr := rawdb.ReadHeader(o.ChainDB, res.TargetHash, res.Config.TargetHeight)
	if hdr == nil {
		db.Close()
		return nil, fmt.Errorf("source: target header %d %s not decodable", res.Config.TargetHeight, res.Config.TargetHash)
	}
	if got := hdr.Hash(); got != res.TargetHash {
		db.Close()
		return nil, fmt.Errorf("source: target header recomputes to %s, anchor says %s", got.Hex(), res.Config.TargetHash)
	}
	o.Header = hdr

	o.NormA = norm.Anchor{
		Network:            res.Config.Network,
		Shard:              res.Config.Shard,
		TargetHeight:       res.Config.TargetHeight,
		TargetHash:         res.TargetHash,
		TargetRoot:         hdr.Root(),
		Epoch:              res.Config.Epoch,
		EpochFirst:         res.Config.EpochFirstBlock,
		EpochLast:          res.Config.EpochLastBlock,
		SnapshotBase:       res.Config.SnapshotBaseHeight,
		BoundaryHeight:     res.Inplace.BoundaryHeight,
		AbandonedChildHash: res.ChildHash,
		AuditEndHeight:     res.Config.AuditEndHeight,
		ConfigSHA256Hex:    res.ConfigSHAHex(),
	}
	return o, nil
}

// BuildSources opens fresh state handles (fresh iterators/state caches per
// derivation pass — the determinism self-check runs two independent
// passes).
func (o *Open) BuildSources() (norm.Sources, error) {
	sdb := state.NewDatabase(o.ChainDB)
	o.stateDBs = append(o.stateDBs, sdb)
	target, err := state.New(o.Header.Root(), sdb, nil)
	if err != nil {
		return norm.Sources{}, &TargetStateError{Err: err}
	}
	return norm.Sources{
		Raw:     o.KV,
		Target:  target,
		Hist:    &histOpener{o: o, sdb: sdb},
		Headers: &headerReader{o: o},
	}, nil
}

// TargetStateError marks state.New(targetRoot) failure (exit 22).
type TargetStateError struct{ Err error }

func (e *TargetStateError) Error() string {
	return fmt.Sprintf("target state unavailable: %v", e.Err)
}
func (e *TargetStateError) Unwrap() error { return e.Err }

// Close releases the DB handle.
func (o *Open) Close() error { return o.DB.Close() }

type histOpener struct {
	o   *Open
	sdb state.Database
}

// StateAt resolves canonical(height).Root and opens it; unavailability is
// (nil, nil) — structural-only, never an error (plan §4.4 best-effort).
func (h *histOpener) StateAt(height uint64) (*state.DB, error) {
	hash := rawdb.ReadCanonicalHash(h.o.ChainDB, height)
	if hash == (common.Hash{}) {
		return nil, nil
	}
	hdr := rawdb.ReadHeader(h.o.ChainDB, hash, height)
	if hdr == nil {
		return nil, nil
	}
	st, err := state.New(hdr.Root(), h.sdb, nil)
	if err != nil {
		return nil, nil
	}
	return st, nil
}

type headerReader struct{ o *Open }

func (h *headerReader) HeaderByNumber(height uint64) (*block.Header, error) {
	hash := rawdb.ReadCanonicalHash(h.o.ChainDB, height)
	if hash == (common.Hash{}) {
		return nil, fmt.Errorf("no canonical hash at height %d", height)
	}
	hdr := rawdb.ReadHeader(h.o.ChainDB, hash, height)
	if hdr == nil {
		return nil, fmt.Errorf("header %d %s not found", height, hash.Hex())
	}
	return hdr, nil
}
