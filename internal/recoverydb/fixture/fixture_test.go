package fixture

import (
	"testing"

	"github.com/harmony-one/harmony/internal/recoverydb/dbopen"
	"github.com/harmony-one/harmony/internal/recoverydb/verify"
)

// TestGenerateSmoke proves the in-process localnet producer creates a
// replay-grade chain with real BLS certificates (plan WS8 fixture kit).
func TestGenerateSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("fixture generation is not short")
	}
	dir := t.TempDir() + "/harmony_db_0"
	c, err := Open(dir, RepoKeysDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := c.Generate(Params{Blocks: 20, TxEvery: 4}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	head := c.Bc.CurrentBlock()
	if head.NumberU64() != 20 {
		t.Fatalf("head at %d, want 20", head.NumberU64())
	}
	if len(head.GetCurrentCommitSig()) == 0 {
		t.Fatalf("head has no commit certificate")
	}
	if err := c.Finalize(); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	// Reopen strictly read-only, verify the head certificate with the
	// standalone verifier, and walk the state.
	db, ro, err := dbopen.OpenSourceDatabase(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer ro.Close()
	cfg := c.Bc.Config()
	cv := verify.NewCertVerifier(db, cfg, 0)
	sigVal, err := db.Get(blockSigKeyForTest(20))
	if err != nil {
		t.Fatalf("read block-sig-20: %v", err)
	}
	if err := cv.VerifyCommitSigBytes(head.Header(), sigVal); err != nil {
		t.Fatalf("verify head certificate: %v", err)
	}
	walk, err := verify.WalkState(db, head.Root(), verify.StateWalkOptions{CheckPreimages: true, RequirePreimages: true})
	if err != nil {
		t.Fatalf("state walk: %v", err)
	}
	if walk.AccountCount == 0 {
		t.Fatalf("state walk found no accounts")
	}
	t.Logf("fixture OK: %d accounts, %d slots, %d codes", walk.AccountCount, walk.StorageSlotCount, walk.UniqueCodeCount)
}

func blockSigKeyForTest(n uint64) []byte {
	return append([]byte("block-sig-"), []byte{0, 0, 0, 0, 0, 0, 0, byte(n)}...)
}
