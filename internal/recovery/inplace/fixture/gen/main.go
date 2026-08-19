// Command gen materializes the deterministic preflight fixtures into
// testdata/recovery/preflight/ (invoked via
// scripts/recovery/gen-preflight-fixtures.sh). Tests build the same
// fixtures hermetically in temp dirs through the fixture package; the
// materialized copies exist for manual inspection and ad-hoc runs of the
// preflight binary against a known-good database.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
)

func main() {
	out := "testdata/recovery/preflight"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	variants := map[string]fixture.Variant{
		"base":               fixture.VariantBase,
		"bad-account-leaf":   fixture.VariantBadAccountLeaf,
		"bad-storage-leaf":   fixture.VariantBadStorageLeaf,
		"flagged-empty-code": fixture.VariantFlaggedEmptyCode,
		"many-anomalies":     fixture.VariantManyAnomalies,
		"wrapper-unbound":    fixture.VariantWrapperUnbound,
	}
	type summary struct {
		StateRoot  string `json:"state_root"`
		TargetHash string `json:"target_hash"`
		ChildHash  string `json:"child_hash"`
		Height     uint64 `json:"target_height"`
		Network    string `json:"network"`
	}
	all := map[string]summary{}
	for name, v := range variants {
		dir := filepath.Join(out, name, "harmony_db_0")
		if err := os.RemoveAll(dir); err != nil {
			fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			fatal(err)
		}
		m, err := fixture.Build(dir, v)
		if err != nil {
			fatal(fmt.Errorf("build %s: %w", name, err))
		}
		all[name] = summary{
			StateRoot:  m.StateRoot.Hex(),
			TargetHash: m.TargetHash.Hex(),
			ChildHash:  m.ChildHash.Hex(),
			Height:     fixture.TargetHeight,
			Network:    "localnet",
		}
		fmt.Printf("%-20s target %s state %s -> %s\n", name, m.TargetHash.Hex(), m.StateRoot.Hex(), dir)
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "fixtures.json"), append(data, '\n'), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gen-preflight-fixtures:", err)
	os.Exit(1)
}
