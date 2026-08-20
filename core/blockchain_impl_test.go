package core

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	staking "github.com/harmony-one/harmony/staking/types"
)

var errInjectedBatchWrite = errors.New("injected batch write failure")

type failingBatchDatabase struct {
	ethdb.Database
}

func (db failingBatchDatabase) NewBatch() ethdb.Batch {
	return failingBatch{Batch: db.Database.NewBatch()}
}

type failingBatch struct {
	ethdb.Batch
}

func (failingBatch) Write() error {
	return errInjectedBatchWrite
}

func TestRollbackClearsCanonicalMappingAndCache(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, database := getTestEnvironment(*key)
	defer chain.Stop()

	genesis := chain.Genesis()
	makeBlock := func(parent common.Hash, number int64, extra string) *types.Block {
		h := header.With().
			ParentHash(parent).
			Number(big.NewInt(number)).
			Root(genesis.Root()).
			Extra([]byte(extra)).
			Header()
		return types.NewBlockWithHeader(h)
	}

	oldBlock := makeBlock(genesis.Hash(), 1, "old canonical block")
	if err := rawdb.WriteBlock(database, oldBlock); err != nil {
		t.Fatalf("write old block: %v", err)
	}
	if err := chain.WriteHeadBlock(oldBlock); err != nil {
		t.Fatalf("set old canonical head: %v", err)
	}
	if got := chain.GetBlockByNumber(1); got == nil || got.Hash() != oldBlock.Hash() {
		t.Fatalf("failed to warm canonical cache with old block: got %v", got)
	}

	staleBlock2 := makeBlock(oldBlock.Hash(), 2, "stale block above current head")
	if err := rawdb.WriteCanonicalHash(database, staleBlock2.Hash(), 2); err != nil {
		t.Fatalf("write stale canonical mapping: %v", err)
	}
	if got := chain.GetCanonicalHash(2); got != staleBlock2.Hash() {
		t.Fatalf("failed to warm stale canonical cache: got %s want %s", got, staleBlock2.Hash())
	}

	if err := chain.Rollback([]common.Hash{oldBlock.Hash()}); err != nil {
		t.Fatalf("rollback old block: %v", err)
	}
	if got := chain.CurrentBlock().Hash(); got != genesis.Hash() {
		t.Fatalf("current block after rollback: got %s want genesis %s", got, genesis.Hash())
	}
	if got := rawdb.ReadCanonicalHash(database, 1); got != (common.Hash{}) {
		t.Fatalf("persistent canonical mapping survived rollback: got %s", got)
	}
	if got := chain.GetCanonicalHash(1); got != (common.Hash{}) {
		t.Fatalf("cached canonical mapping survived rollback: got %s", got)
	}
	if got := rawdb.ReadCanonicalHash(database, 2); got != (common.Hash{}) {
		t.Fatalf("persistent canonical mapping above rolled-back head survived: got %s", got)
	}
	if got := chain.GetCanonicalHash(2); got != (common.Hash{}) {
		t.Fatalf("cached canonical mapping above rolled-back head survived: got %s", got)
	}

	replacement := makeBlock(genesis.Hash(), 1, "replacement canonical block")
	if err := rawdb.WriteBlock(database, replacement); err != nil {
		t.Fatalf("write replacement block: %v", err)
	}
	if err := chain.WriteHeadBlock(replacement); err != nil {
		t.Fatalf("set replacement canonical head: %v", err)
	}
	if got := chain.GetBlockByNumber(1); got == nil || got.Hash() != replacement.Hash() {
		t.Fatalf("canonical lookup after replacement: got %v want %s", got, replacement.Hash())
	}
}

func TestRollbackBatchFailureLeavesHeadsAndCanonicalMappingUnchanged(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, database := getTestEnvironment(*key)
	defer chain.Stop()

	genesis := chain.Genesis()
	oldBlock := types.NewBlockWithHeader(header.With().
		ParentHash(genesis.Hash()).
		Number(big.NewInt(1)).
		Root(genesis.Root()).
		Extra([]byte("old canonical block")).
		Header())
	if err := rawdb.WriteBlock(database, oldBlock); err != nil {
		t.Fatalf("write old block: %v", err)
	}
	if err := chain.WriteHeadBlock(oldBlock); err != nil {
		t.Fatalf("write old head: %v", err)
	}

	failingDB := failingBatchDatabase{Database: database}
	chain.db = failingDB
	chain.hc.chainDb = failingDB

	err := chain.Rollback([]common.Hash{oldBlock.Hash()})
	if !errors.Is(err, errInjectedBatchWrite) {
		t.Fatalf("rollback error = %v, want %v", err, errInjectedBatchWrite)
	}
	if got := chain.CurrentBlock().Hash(); got != oldBlock.Hash() {
		t.Fatalf("in-memory block head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := chain.CurrentFastBlock().Hash(); got != oldBlock.Hash() {
		t.Fatalf("in-memory fast head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := chain.CurrentHeader().Hash(); got != oldBlock.Hash() {
		t.Fatalf("in-memory header head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := rawdb.ReadHeadBlockHash(database); got != oldBlock.Hash() {
		t.Fatalf("persistent block head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := rawdb.ReadHeadFastBlockHash(database); got != oldBlock.Hash() {
		t.Fatalf("persistent fast head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := rawdb.ReadHeadHeaderHash(database); got != oldBlock.Hash() {
		t.Fatalf("persistent header head changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
	if got := rawdb.ReadCanonicalHash(database, 1); got != oldBlock.Hash() {
		t.Fatalf("canonical mapping changed after failed rollback: got %s want %s", got, oldBlock.Hash())
	}
}

func TestGetCanonicalHashWaitsForCanonicalMutation(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, _, _ := getTestEnvironment(*key)
	defer chain.Stop()

	chain.hc.canonicalMu.Lock()
	done := make(chan common.Hash, 1)
	go func() {
		done <- chain.GetCanonicalHash(0)
	}()

	select {
	case <-done:
		chain.hc.canonicalMu.Unlock()
		t.Fatal("canonical read bypassed mutation lock")
	case <-time.After(25 * time.Millisecond):
	}

	chain.hc.canonicalMu.Unlock()
	select {
	case got := <-done:
		if got != chain.Genesis().Hash() {
			t.Fatalf("canonical read after mutation lock: got %s want %s", got, chain.Genesis().Hash())
		}
	case <-time.After(time.Second):
		t.Fatal("canonical read did not resume after mutation lock")
	}
}

// TestIsSpentIgnoresMutatedMerkleProofIdentity guards against replaying a
// genuine, already-applied CXReceiptsProof by mutating the unauthenticated
// MerkleProof.ShardID/BlockNum while keeping the same signed Header: the
// spent-marker must be keyed off the Header, which cannot be altered without
// invalidating the commit signature, not off MerkleProof fields that
// ValidateCXReceiptsProof only binds to the Header from
// IsCXMerkleProofReplayFixEpoch onward.
func TestIsSpentIgnoresMutatedMerkleProofIdentity(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, database := getTestEnvironment(*key)

	header = header.With().ShardID(1).Number(big.NewInt(42)).Header()

	original := &types.CXReceiptsProof{
		Header: header,
		MerkleProof: &types.CXMerkleProof{
			ShardID:  1,
			BlockNum: big.NewInt(42),
		},
	}

	batch := database.NewBatch()
	if err := chain.WriteCXReceiptsProofSpent(batch, []*types.CXReceiptsProof{original}); err != nil {
		t.Fatalf("WriteCXReceiptsProofSpent failed: %v", err)
	}
	if err := batch.Write(); err != nil {
		t.Fatalf("batch.Write failed: %v", err)
	}

	if !chain.IsSpent(original) {
		t.Fatal("expected original proof to be marked spent")
	}

	replay := &types.CXReceiptsProof{
		Header: header, // same genuine, signed header
		MerkleProof: &types.CXMerkleProof{
			ShardID:  99,               // mutated, unauthenticated
			BlockNum: big.NewInt(9999), // mutated, unauthenticated
		},
	}
	if !chain.IsSpent(replay) {
		t.Fatal("expected replay with mutated MerkleProof identity to be detected as already spent")
	}
}

func TestPrepareStakingMetadata(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	// fake transaction
	tx := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	txs := []*types.Transaction{tx}

	// fake staking transactions
	stx1 := signedCreateValidatorStakingTxn(key)
	stx2 := signedDelegateStakingTxn(key)
	stxs := []*staking.StakingTransaction{stx1, stx2}

	// make a fake block header
	block := types.NewBlock(header, txs, []*types.Receipt{types.NewReceipt([]byte{}, false, 0), types.NewReceipt([]byte{}, false, 0),
		types.NewReceipt([]byte{}, false, 0)}, nil, nil, stxs)
	// run it
	if _, _, err := chain.prepareStakingMetaData(block, []staking.StakeMsg{&staking.Delegate{}}, db); err != nil {
		if err.Error() != "address not present in state" { // when called in test for core/vm
			t.Errorf("Got error %v in prepareStakingMetaData", err)
		}
	} else {
		// when called independently there is no error
	}
}

func signedCreateValidatorStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCreateValidator, sampleCreateValidator(*key)
	}
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedEditValidatorStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveEditValidator, sampleEditValidator(*key)
	}
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedDelegateStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveDelegate, sampleDelegate(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedUndelegateStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveUndelegate, sampleUndelegate(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedCollectRewardsStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCollectRewards, sampleCollectRewards(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}
