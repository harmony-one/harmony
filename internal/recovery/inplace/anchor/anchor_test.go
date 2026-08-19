package anchor

import (
	"strings"
	"testing"

	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
)

// TestMainnetScheduleProperties pins the derived epoch geometry the plan
// relies on: CalcEpochNumber(92,730,034) = 3002 and EpochLastBlock(3001) =
// 92,700,671 under the compiled mainnet schedule.
func TestMainnetScheduleProperties(t *testing.T) {
	s := shardingconfig.MainnetSchedule
	if got := s.CalcEpochNumber(MainnetTargetHeight); got.Uint64() != 3002 {
		t.Fatalf("CalcEpochNumber(%d) = %s, want 3002", MainnetTargetHeight, got)
	}
	if got := s.EpochLastBlock(3001); got != 92700671 {
		t.Fatalf("EpochLastBlock(3001) = %d, want 92700671", got)
	}
	if got := s.EpochLastBlock(3002); got != 92733439 {
		t.Fatalf("EpochLastBlock(3002) = %d, want 92733439", got)
	}
}

// TestMainnetResolve checks the compiled constants land in the anchor
// (reviewed literals) and the derived fields match the schedule.
func TestMainnetResolve(t *testing.T) {
	a, err := Resolve("mainnet", 0, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if a.TargetHeight != 92730034 {
		t.Fatalf("target height %d", a.TargetHeight)
	}
	if a.TargetHash.Hex() != "0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d" {
		t.Fatalf("target hash %s", a.TargetHash.Hex())
	}
	if a.Epoch.Uint64() != 3002 {
		t.Fatalf("epoch %s", a.Epoch)
	}
	if a.BoundaryHeight != 92700671 {
		t.Fatalf("boundary %d", a.BoundaryHeight)
	}
	if !a.ChainConfig.IsStaking(a.Epoch) {
		t.Fatal("target epoch must be staking-era")
	}
}

// TestMainnetOverridesRefused: the compiled constants are authoritative on
// mainnet.
func TestMainnetOverridesRefused(t *testing.T) {
	if _, err := Resolve("mainnet", 0, Overrides{TargetHeight: 42}); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("height override not refused: %v", err)
	}
	if _, err := Resolve("mainnet", 0, Overrides{TargetHash: "0xdead"}); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("hash override not refused: %v", err)
	}
	if _, err := Resolve("mainnet", 1, Overrides{}); err == nil || !strings.Contains(err.Error(), "shard 0") {
		t.Fatalf("non-zero shard not refused: %v", err)
	}
}

func TestNonMainnetResolve(t *testing.T) {
	hash := "0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d"
	if _, err := Resolve("localnet", 0, Overrides{}); err == nil {
		t.Fatal("localnet without overrides must be refused")
	}
	if _, err := Resolve("localnet", 0, Overrides{TargetHeight: 44, TargetHash: "0x123"}); err == nil {
		t.Fatal("short hash must be refused")
	}
	a, err := Resolve("localnet", 0, Overrides{TargetHeight: 44, TargetHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	if a.Epoch.Uint64() != 3 || a.BoundaryHeight != 36 {
		t.Fatalf("localnet target 44: epoch %s boundary %d, want 3/36", a.Epoch, a.BoundaryHeight)
	}
	if _, err := Resolve("neptune", 0, Overrides{TargetHeight: 44, TargetHash: hash}); err == nil {
		t.Fatal("unknown network must be refused")
	}
	// Pre-staking target refused (localnet epoch 1 < staking epoch 2).
	if _, err := Resolve("localnet", 0, Overrides{TargetHeight: 10, TargetHash: hash}); err == nil || !strings.Contains(err.Error(), "staking") {
		t.Fatalf("pre-staking target not refused: %v", err)
	}
}
