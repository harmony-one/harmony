package certverify_test

import (
	"math/big"
	"path/filepath"
	"testing"
	"time"

	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/recovery/inplace/anchor"
	"github.com/harmony-one/harmony/internal/recovery/inplace/certverify"
	"github.com/harmony-one/harmony/internal/recovery/inplace/chainread"
	"github.com/harmony-one/harmony/internal/recovery/inplace/fixture"
	"github.com/harmony-one/harmony/internal/recovery/inplace/rodb"
)

func fixtureOutcome(t *testing.T) (*fixture.Manifest, *anchor.Anchor, *rodb.DB, *chainread.Outcome) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "harmony_db_0")
	m, err := fixture.Build(dir, fixture.VariantBase)
	if err != nil {
		t.Fatal(err)
	}
	a, err := anchor.Resolve("localnet", 0, anchor.Overrides{
		TargetHeight: fixture.TargetHeight,
		TargetHash:   m.TargetHash.Hex(),
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := rodb.Open(dir, rodb.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	out, err := chainread.RunChecks(db.KV(&rodb.Latch{}), a, nil)
	if err != nil {
		t.Fatalf("chain checks: %v", err)
	}
	return m, a, db, out
}

// TestVerifyBothSources: the fixture carries both certificate sources with
// identical bytes; verification must pass via the production engine.
func TestVerifyBothSources(t *testing.T) {
	_, a, db, out := fixtureOutcome(t)
	res, err := certverify.Verify(db.KV(&rodb.Latch{}), a, out)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if res.Sources.SatisfiedBy != "exact-key+child-header" {
		t.Fatalf("satisfied by %q", res.Sources.SatisfiedBy)
	}
}

// TestBlockChainImplDifferential: the independent certificate differential.
// A real core.BlockChainImpl is constructed over a scratch COPY of the
// fixture (it may write there - production preflight never constructs one),
// and the engine's verdict through it must agree with the verdict through
// the minimal fail-closed ChainReader, on both the accept and the reject
// side. A shared bug in the minimal reader's committee sourcing would break
// the agreement here.
func TestBlockChainImplDifferential(t *testing.T) {
	m, a, _, out := fixtureOutcome(t)

	scratch := filepath.Join(t.TempDir(), "harmony_db_0")
	if err := fixture.CopyDB(m.Dir, scratch); err != nil {
		t.Fatal(err)
	}
	db, err := rawdb.NewLevelDBDatabase(scratch, 64, 128, "", false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// BlockChainImpl refuses to construct without a genesis block; the
	// recovery fixture deliberately starts at the epoch boundary, so plant
	// a minimal genesis in the scratch copy.
	factory := blockfactory.NewFactory(params.LocalnetChainConfig)
	gh := factory.NewHeader(big.NewInt(0))
	gh.SetNumber(big.NewInt(0))
	gh.SetShardID(0)
	if err := rawdb.WriteHeader(db, gh); err != nil {
		t.Fatal(err)
	}
	if err := rawdb.WriteCanonicalHash(db, gh.Hash(), 0); err != nil {
		t.Fatal(err)
	}
	gbody, err := types.NewBodyForMatchingHeader(gh)
	if err != nil {
		t.Fatal(err)
	}
	if err := rawdb.WriteBody(db, gh.Hash(), 0, gbody); err != nil {
		t.Fatal(err)
	}
	// Point the head block at the target so CurrentHeader matches what the
	// minimal reader serves (the fixture's head is the child, whose header
	// carries no state root).
	if err := rawdb.WriteHeadBlockHash(db, m.TargetHash); err != nil {
		t.Fatal(err)
	}

	// The constructor's leader-rotation init walk resolves each header's
	// coinbase to a committee BLS key - metadata the minimal recovery
	// fixture does not carry, and irrelevant to certificate verification.
	// Push the activation epoch out of reach for the scratch chain only;
	// every field the signature path consults is untouched.
	cfg := *params.LocalnetChainConfig
	cfg.LeaderRotationInternalValidatorsEpoch = big.NewInt(1 << 30)
	cfg.LeaderRotationExternalValidatorsEpoch = big.NewInt(1 << 30)

	engine := chain.NewEngine()
	bc, err := core.NewBlockChain(db, nil, nil, &core.CacheConfig{
		// Disabled skips Stop()'s recent-trie commit sweep, which would
		// dereference the zero roots the fixture's non-target headers carry.
		Disabled:       true,
		TrieCleanLimit: 16,
		TrieDirtyLimit: 16,
		TrieTimeLimit:  time.Minute,
		TriesInMemory:  128,
		SnapshotLimit:  0,
	}, &cfg, engine, vm.Config{})
	if err != nil {
		t.Fatalf("scratch BlockChainImpl: %v", err)
	}
	defer bc.Stop()
	if got := bc.CurrentHeader().Hash(); got != m.TargetHash {
		t.Fatalf("scratch chain current header %s, want target %s", got.Hex(), m.TargetHash.Hex())
	}

	minimal := chainread.NewMinimalChainReader(
		params.LocalnetChainConfig, a.ShardID, out.TargetHeader, a.Epoch, out.ShardState)

	// Accept side: the genuine certificate verifies through both readers.
	sig, bitmap, err := chain.ParseCommitSigAndBitmap(m.CertPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.VerifyHeaderSignature(bc, out.TargetHeader, sig, bitmap); err != nil {
		t.Fatalf("BlockChainImpl path rejects the genuine certificate: %v", err)
	}
	if err := engine.VerifyHeaderSignature(minimal, out.TargetHeader, sig, bitmap); err != nil {
		t.Fatalf("minimal-reader path rejects the genuine certificate: %v", err)
	}

	// Reject side: a tampered aggregate fails through both readers.
	bad := append([]byte(nil), m.CertPayload...)
	bad[10] ^= 0x01
	badSig, badBitmap, err := chain.ParseCommitSigAndBitmap(bad)
	if err != nil {
		t.Fatal(err)
	}
	errFull := engine.VerifyHeaderSignature(bc, out.TargetHeader, badSig, badBitmap)
	errMin := engine.VerifyHeaderSignature(minimal, out.TargetHeader, badSig, badBitmap)
	if errFull == nil || errMin == nil {
		t.Fatalf("tampered certificate accepted: full=%v minimal=%v", errFull, errMin)
	}
}

// TestChainReaderMethodPin: the certificate verification path exercises
// exactly {Config, CurrentHeader, ShardID, ReadShardState} on the minimal
// ChainReader. An upstream internal/chain change that starts calling more
// methods fails here (and fails closed at runtime).
func TestChainReaderMethodPin(t *testing.T) {
	m, a, _, out := fixtureOutcome(t)

	reader := chainread.NewMinimalChainReader(
		params.LocalnetChainConfig, a.ShardID, out.TargetHeader, a.Epoch, out.ShardState)
	engine := chain.NewEngine()
	sig, bitmap, err := chain.ParseCommitSigAndBitmap(m.CertPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.VerifyHeaderSignature(reader, out.TargetHeader, sig, bitmap); err != nil {
		t.Fatalf("verification failed: %v", err)
	}
	called := reader.CalledMethods()
	audited := map[string]bool{
		"Config": true, "CurrentHeader": true, "ShardID": true, "ReadShardState": true,
	}
	for method := range called {
		if !audited[method] {
			t.Fatalf("engine exercised unaudited ChainReader method %s (called set %v)", method, called)
		}
	}
	for _, must := range []string{"Config", "CurrentHeader", "ReadShardState"} {
		if called[must] == 0 {
			t.Fatalf("expected engine to call %s (called set %v)", must, called)
		}
	}
}
