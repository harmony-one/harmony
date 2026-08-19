package fixture

import (
	"path/filepath"
	"testing"

	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/shard"
)

// TestGenerateTwinChain proves the generator builds a replay-grade localnet
// chain spanning an epoch boundary with pre- and post-target staking.
func TestGenerateTwinChain(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	c, err := Open(dir, RepoKeysDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// localnet 16/16: epoch boundaries at 4,20,36,52,... (block1=5). A
	// target at 30 (epoch 2, staking) with the 36/37 boundary in the
	// post-target range gives a transition to audit.
	spec := Spec{
		Blocks:                44,
		CreateValidatorAt:     10,
		DelegateAt:            14,
		PostCreateValidatorAt: 40,
		PostDelegateAt:        42,
	}
	if err := c.Generate(spec); err != nil {
		t.Fatalf("generate: %v", err)
	}
	head := c.Bc.CurrentBlock()
	if head.NumberU64() != 44 {
		t.Fatalf("head at %d, want 44", head.NumberU64())
	}
	// The pre-target validator must be in the validator list.
	list, err := rawdb.ReadValidatorList(c.DB)
	if err != nil {
		t.Fatalf("read validator list: %v", err)
	}
	found := false
	for _, a := range list {
		if a == c.ValidatorAddr {
			found = true
		}
	}
	if !found {
		t.Fatalf("pre-target validator %s not in list %v", c.ValidatorAddr.Hex(), list)
	}
	t.Logf("validator list has %d entries at head epoch %s", len(list), head.Epoch())
	epochAt := func(n uint64) uint64 { return shard.Schedule.CalcEpochNumber(n).Uint64() }
	t.Logf("epoch(30)=%d epoch(36)=%d epoch(37)=%d epoch(44)=%d",
		epochAt(30), epochAt(36), epochAt(37), epochAt(44))
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}
}
