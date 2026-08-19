// Command gen materializes the localnet fixture kit on disk (plan WS9
// gen-fixtures.sh): a donor whose head extends past the target on a divergent
// suffix, and two byte-identical Aug-8-style baseline copies. The chains
// carry real BLS certificates from the public dev keys — no secrets.
//
// Usage:
//
//	go run ./internal/recoverydb/fixture/gen --out DIR [--keys .hmy] \
//	  --baseline 18 --target 22 --donor 26
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/internal/recoverydb/anchor"
	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/fixture"
	"github.com/harmony-one/harmony/internal/recoverydb/integrity"
)

func main() {
	out := flag.String("out", "", "output directory (required)")
	keys := flag.String("keys", fixture.RepoKeysDir(), "repo .hmy directory with dev BLS keys")
	baseline := flag.Uint64("baseline", 18, "baseline head height")
	target := flag.Uint64("target", 22, "target height")
	donor := flag.Uint64("donor", 26, "donor head height (past the target)")
	anchorOut := flag.String("anchor", "", "if set, write a localnet anchor manifest (+ .sha256) for the fixture target")
	flag.Parse()
	if *out == "" || *baseline == 0 || *target <= *baseline || *donor <= *target {
		fmt.Fprintln(os.Stderr, "usage: gen --out DIR [--keys .hmy] --baseline N --target M --donor K (0<N<M<K)")
		os.Exit(2)
	}
	if err := run(*out, *keys, *anchorOut, *baseline, *target, *donor); err != nil {
		fmt.Fprintf(os.Stderr, "gen-fixtures: %v\n", err)
		os.Exit(1)
	}
}

func run(out, keys, anchorOut string, baseline, target, donor uint64) error {
	donorDir := filepath.Join(out, "donor", "harmony_db_0")
	baseA := filepath.Join(out, "baseline-a", "harmony_db_0")
	baseB := filepath.Join(out, "baseline-b", "harmony_db_0")

	c, err := fixture.Open(donorDir, keys)
	if err != nil {
		return err
	}
	if err := c.Generate(fixture.Params{Blocks: baseline, TxEvery: 5, DeployContractAt: 6, CreateValidatorAt: 9, DelegateAt: 11}); err != nil {
		return err
	}
	if err := c.Finalize(); err != nil {
		return err
	}
	if err := fixture.CopyDir(donorDir, baseA); err != nil {
		return err
	}
	if err := fixture.CopyDir(donorDir, baseB); err != nil {
		return err
	}
	c, err = fixture.Open(donorDir, keys)
	if err != nil {
		return err
	}
	if err := c.Generate(fixture.Params{Blocks: donor - baseline, TxEvery: 5}); err != nil {
		return err
	}
	if err := c.Finalize(); err != nil {
		return err
	}

	if anchorOut != "" {
		if err := writeAnchor(donorDir, anchorOut, baseline, target); err != nil {
			return err
		}
	}
	fmt.Printf("fixture kit written:\n  donor:      %s (head %d)\n  baseline-a: %s (head %d)\n  baseline-b: %s (head %d)\n  target:     %d\n",
		donorDir, donor, baseA, baseline, baseB, baseline, target)
	if anchorOut != "" {
		fmt.Printf("  anchor:     %s\n", anchorOut)
	}
	return nil
}

func writeAnchor(donorDir, anchorOut string, baseline, target uint64) error {
	db, ro, err := dbopen.OpenSourceDatabase(donorDir)
	if err != nil {
		return err
	}
	defer ro.Close()
	targetHash := rawdb.ReadCanonicalHash(db, target)
	hdr := rawdb.ReadHeader(db, targetHash, target)
	if hdr == nil {
		return fmt.Errorf("donor target header %d missing", target)
	}
	childHash := rawdb.ReadCanonicalHash(db, target+1)
	m := &anchor.Manifest{
		SchemaVersion:        anchor.SchemaVersionV1,
		Network:              "localnet",
		ShardID:              0,
		TargetHeight:         target,
		TargetHash:           targetHash,
		TargetParentHash:     hdr.ParentHash(),
		TargetEpoch:          hdr.Epoch().Uint64(),
		BaselineHeight:       baseline,
		AbandonedChildHeight: target + 1,
		AbandonedChildHash:   childHash,
	}
	if m.TargetHash == (common.Hash{}) || m.AbandonedChildHash == (common.Hash{}) {
		return fmt.Errorf("could not resolve target/child hashes from donor")
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(anchorOut), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(anchorOut, raw, 0o644); err != nil {
		return err
	}
	_, err = integrity.WriteChecksumFile(anchorOut)
	return err
}
