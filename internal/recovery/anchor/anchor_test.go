package anchor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	inplaceanchor "github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
)

// localnetConfig builds a schedule-consistent localnet anchor config for a
// staking-era target, computing the geometry from the real schedule so the
// cross-checks pass.
func localnetConfig(t *testing.T) Config {
	t.Helper()
	shardingconfig.InitLocalnetConfig(16, 16)
	sched := shardingconfig.LocalnetSchedule
	// Pick a target in epoch 3 (localnet staking epoch is 2), mid-epoch.
	epoch := uint64(3)
	first := sched.EpochLastBlock(epoch-1) + 1
	last := sched.EpochLastBlock(epoch)
	target := first + 2
	if sched.CalcEpochNumber(target).Uint64() != epoch {
		t.Fatalf("target %d resolves to epoch %s, want %d", target, sched.CalcEpochNumber(target), epoch)
	}
	return Config{
		Schema:             Schema,
		Network:            "localnet",
		Shard:              0,
		TargetHeight:       target,
		TargetHash:         "0x1111111111111111111111111111111111111111111111111111111111111111",
		AbandonedChildHash: "0x2222222222222222222222222222222222222222222222222222222222222222",
		Epoch:              epoch,
		EpochFirstBlock:    first,
		EpochLastBlock:     last,
		SnapshotBaseHeight: sched.EpochLastBlock(epoch-1) - 1,
		AuditEndHeight:     last + 100,
		KnownBadBlocks:     []uint64{target + 2},
	}
}

func writeConfig(t *testing.T, c Config) string {
	t.Helper()
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "recovery-anchor.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveLocalnetRoundTrip(t *testing.T) {
	c := localnetConfig(t)
	p := writeConfig(t, c)
	r, err := Resolve(p)
	if err != nil {
		t.Fatalf("resolve valid localnet config: %v", err)
	}
	if r.Config.TargetHeight != c.TargetHeight {
		t.Fatalf("target height %d, want %d", r.Config.TargetHeight, c.TargetHeight)
	}
	if r.ConfigSHAHex() == "" {
		t.Fatal("config sha not recorded")
	}
}

func TestResolveRejectsUnknownField(t *testing.T) {
	c := localnetConfig(t)
	raw, _ := json.Marshal(c)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	m["surprise"] = 1
	raw2, _ := json.Marshal(m)
	p := filepath.Join(t.TempDir(), "a.json")
	_ = os.WriteFile(p, raw2, 0o644)
	if _, err := Resolve(p); err == nil {
		t.Fatal("unknown field must be rejected (strict parse)")
	}
}

func TestResolveRejectsScheduleMismatch(t *testing.T) {
	c := localnetConfig(t)
	c.EpochLastBlock += 1 // wrong
	p := writeConfig(t, c)
	if _, err := Resolve(p); err == nil {
		t.Fatal("schedule mismatch must be rejected")
	}
}

func TestResolveRejectsWrongSnapshotBase(t *testing.T) {
	c := localnetConfig(t)
	c.SnapshotBaseHeight += 3
	p := writeConfig(t, c)
	if _, err := Resolve(p); err == nil {
		t.Fatal("wrong snapshot base must be rejected")
	}
}

func TestResolveRejectsAuditEndBelowTarget(t *testing.T) {
	c := localnetConfig(t)
	c.AuditEndHeight = c.TargetHeight
	p := writeConfig(t, c)
	if _, err := Resolve(p); err == nil {
		t.Fatal("audit_end_height <= target must be rejected")
	}
}

func TestResolveRejectsBadSchema(t *testing.T) {
	c := localnetConfig(t)
	c.Schema = "wrong"
	p := writeConfig(t, c)
	if _, err := Resolve(p); err == nil {
		t.Fatal("wrong schema must be rejected")
	}
}

// TestMainnetDriftPin pins the shipped anchor config against the compiled
// importable symbols (plan WS1 drift test). The shipped file lives next to
// the release docs; the pin re-verifies its exact constants.
func TestMainnetDriftPin(t *testing.T) {
	// The compiled anchor is authoritative; the shipped mainnet config must
	// echo it exactly. We assert the compiled symbols themselves here so a
	// change to either side trips the pin.
	if inplaceanchor.MainnetTargetHeight != 92730034 {
		t.Fatalf("compiled MainnetTargetHeight drifted: %d", inplaceanchor.MainnetTargetHeight)
	}
	if inplaceanchor.MainnetTargetHashHex != "0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d" {
		t.Fatalf("compiled MainnetTargetHashHex drifted: %s", inplaceanchor.MainnetTargetHashHex)
	}
}
