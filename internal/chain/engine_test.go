package chain

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/trie"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/engine"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	types2 "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

var (
	bigOne        = big.NewInt(1e18)
	tenKOnes      = new(big.Int).Mul(big.NewInt(10000), bigOne)
	twentyKOnes   = new(big.Int).Mul(big.NewInt(20000), bigOne)
	fourtyKOnes   = new(big.Int).Mul(big.NewInt(40000), bigOne)
	thousandKOnes = new(big.Int).Mul(big.NewInt(1000000), bigOne)
)

const (
	// validator creation parameters
	doubleSignShardID     = 0
	doubleSignEpoch       = 4
	doubleSignBlockNumber = 37
	doubleSignViewID      = 38

	creationHeight  = 33
	lastEpochInComm = 5
	currentEpoch    = 5

	numShard        = 2
	numNodePerShard = 5

	offenderShard      = doubleSignShardID
	offenderShardIndex = 0
)

var (
	doubleSignBlock1 = makeBlockForTest(doubleSignEpoch, 0)
	doubleSignBlock2 = makeBlockForTest(doubleSignEpoch, 1)
)

var (
	keyPairs = genKeyPairs(25)

	offIndex = offenderShard*numNodePerShard + offenderShardIndex
	offAddr  = makeTestAddress(offIndex)
	offKey   = keyPairs[offIndex]
	offPub   = offKey.Pub()

	leaderAddr = makeTestAddress("leader")
)

// Tests that slashing works on the engine level. Since all slashing is
// thoroughly unit tested on `double-sign_test.go`, it just makes sure that
// slashing is applied to the state.
func TestApplySlashing(t *testing.T) {
	chain := makeFakeBlockChain()
	state := makeTestStateDB()
	header := makeFakeHeader()
	current := makeDefaultValidatorWrapper()
	slashes := slash.Records{makeSlashRecord()}

	if err := state.UpdateValidatorWrapper(current.Address, current); err != nil {
		t.Error(err)
	}
	if _, err := state.Commit(true); err != nil {
		t.Error(err)
	}

	// Inital Leader's balance: 0
	// Initial Validator's self-delegation: FourtyKOnes
	if err := applySlashes(chain, header, state, slashes); err != nil {
		t.Error(err)
	}

	expDelAmountAfterSlash := twentyKOnes
	expRewardToBeneficiary := tenKOnes

	if current.Delegations[0].Amount.Cmp(expDelAmountAfterSlash) != 0 {
		t.Errorf("Slashing was not applied properly to validator: %v/%v", expDelAmountAfterSlash, current.Delegations[0].Amount)
	}

	beneficiaryBalanceAfterSlash := state.GetBalance(leaderAddr)
	if beneficiaryBalanceAfterSlash.Cmp(expRewardToBeneficiary) != 0 {
		t.Errorf("Slashing reward was not added properly to beneficiary: %v/%v", expRewardToBeneficiary, beneficiaryBalanceAfterSlash)
	}
}

func TestVerifyShardStateRejectsEmptyShardState(t *testing.T) {
	chain := makeFakeBlockChain()
	chain.superCommittee = shard.State{}
	header := makeFakeHeader()
	header.SetEpoch(big.NewInt(currentEpoch))

	encodedEmptyState, err := shard.EncodeWrapper(shard.State{}, false)
	if err != nil {
		t.Fatal(err)
	}
	header.SetShardState(encodedEmptyState)

	err = NewEngine().VerifyShardState(chain, chain, header)
	if err == nil {
		t.Fatal("expected empty shard state to be rejected")
	}
}

//
// Make slash record for testing
//

func makeSlashRecord() slash.Record {
	return slash.Record{
		Evidence: slash.Evidence{
			ConflictingVotes: slash.ConflictingVotes{
				FirstVote:  makeVoteData(offKey, doubleSignBlock1),
				SecondVote: makeVoteData(offKey, doubleSignBlock2),
			},
			Moment: slash.Moment{
				Epoch:   big.NewInt(doubleSignEpoch),
				ShardID: doubleSignShardID,
				Height:  doubleSignBlockNumber,
				ViewID:  doubleSignViewID,
			},
			Offender: offAddr,
		},
		Reporter: makeTestAddress("reporter"),
	}
}

//
// Make validator for testing
//

func makeDefaultValidatorWrapper() *staking.ValidatorWrapper {
	pubKeys := []bls.SerializedPublicKey{offPub}
	v := defaultTestValidator(pubKeys)

	ds := staking.Delegations{}
	ds = append(ds, staking.Delegation{
		DelegatorAddress: offAddr,
		Amount:           new(big.Int).Set(fourtyKOnes),
	})

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

func defaultTestValidator(pubKeys []bls.SerializedPublicKey) staking.Validator {
	comm := staking.Commission{
		CommissionRates: staking.CommissionRates{
			Rate:          numeric.MustNewDecFromStr("0.167983520183826780"),
			MaxRate:       numeric.MustNewDecFromStr("0.179184469782137200"),
			MaxChangeRate: numeric.MustNewDecFromStr("0.152212761523253600"),
		},
		UpdateHeight: big.NewInt(10),
	}

	desc := staking.Description{
		Name:            "someoneA",
		Identity:        "someoneB",
		Website:         "someoneC",
		SecurityContact: "someoneD",
		Details:         "someoneE",
	}
	return staking.Validator{
		Address:              offAddr,
		SlotPubKeys:          pubKeys,
		LastEpochInCommittee: big.NewInt(lastEpochInComm),
		MinSelfDelegation:    new(big.Int).Set(tenKOnes),
		MaxTotalDelegation:   new(big.Int).Set(thousandKOnes),
		Status:               effective.Active,
		Commission:           comm,
		Description:          desc,
		CreationHeight:       big.NewInt(creationHeight),
	}
}

//
// Make commitee for testing
//

func makeDefaultCommittee() shard.State {
	epoch := big.NewInt(doubleSignEpoch)
	maker := newShardSlotMaker(keyPairs)
	sstate := shard.State{
		Epoch:  epoch,
		Shards: make([]shard.Committee, 0, int(numShard)),
	}
	for sid := uint32(0); sid != numNodePerShard; sid++ {
		sstate.Shards = append(sstate.Shards, makeShardBySlotMaker(sid, maker))
	}
	return sstate
}

type shardSlotMaker struct {
	kps []blsKeyPair
	i   int
}

func makeShardBySlotMaker(shardID uint32, maker shardSlotMaker) shard.Committee {
	cmt := shard.Committee{
		ShardID: shardID,
		Slots:   make(shard.SlotList, 0, numNodePerShard),
	}
	for nid := 0; nid != numNodePerShard; nid++ {
		cmt.Slots = append(cmt.Slots, maker.makeSlot())
	}
	return cmt
}

func newShardSlotMaker(kps []blsKeyPair) shardSlotMaker {
	return shardSlotMaker{kps, 0}
}

func (maker *shardSlotMaker) makeSlot() shard.Slot {
	s := shard.Slot{
		EcdsaAddress: makeTestAddress(maker.i),
		BLSPublicKey: maker.kps[maker.i].Pub(), // Yes, will panic when not enough kps
	}
	maker.i++
	return s
}

//
// State DB for testing
//

func makeTestStateDB() *state.DB {
	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	sdb, err := state.New(common.Hash{}, db, nil)
	if err != nil {
		panic(err)
	}

	err = sdb.UpdateValidatorWrapper(offAddr, makeDefaultValidatorWrapper())
	if err != nil {
		panic(err)
	}

	return sdb
}

//
// BLS keys for testing
//

type blsKeyPair struct {
	pri *bls_core.SecretKey
	pub *bls_core.PublicKey
}

func genKeyPairs(size int) []blsKeyPair {
	kps := make([]blsKeyPair, 0, size)
	for i := 0; i != size; i++ {
		kps = append(kps, genKeyPair())
	}
	return kps
}

func genKeyPair() blsKeyPair {
	pri := bls.RandPrivateKey()
	pub := pri.GetPublicKey()
	return blsKeyPair{
		pri: pri,
		pub: pub,
	}
}

func (kp blsKeyPair) Pub() bls.SerializedPublicKey {
	var pub bls.SerializedPublicKey
	copy(pub[:], kp.pub.Serialize())
	return pub
}

func (kp blsKeyPair) Sign(block *types.Block) []byte {
	chain := &fakeBlockChain{config: *params.LocalnetChainConfig}
	msg := consensus_sig.ConstructCommitPayload(chain.Config(), block.Epoch(), block.Hash(),
		block.Number().Uint64(), block.Header().ViewID().Uint64())

	sig := kp.pri.SignHash(msg)

	return sig.Serialize()
}

//
// Mock blockchain for testing
//

type fakeBlockChain struct {
	config         params.ChainConfig
	currentBlock   types.Block
	superCommittee shard.State
	snapshots      map[common.Address]staking.ValidatorWrapper
}

func makeFakeBlockChain() *fakeBlockChain {
	return &fakeBlockChain{
		config:         *params.LocalnetChainConfig,
		currentBlock:   *makeBlockForTest(currentEpoch, 0),
		superCommittee: makeDefaultCommittee(),
		snapshots:      make(map[common.Address]staking.ValidatorWrapper),
	}
}

func makeBlockForTest(epoch int64, index int) *types.Block {
	h := blockfactory.NewTestHeader()

	h.SetEpoch(big.NewInt(epoch))
	h.SetNumber(big.NewInt(doubleSignBlockNumber))
	h.SetViewID(big.NewInt(doubleSignViewID))
	h.SetRoot(common.BigToHash(big.NewInt(int64(index))))

	return types.NewBlockWithHeader(h)
}

func (bc *fakeBlockChain) CurrentBlock() *types.Block {
	return &bc.currentBlock
}
func (bc *fakeBlockChain) CurrentHeader() *block.Header {
	return bc.currentBlock.Header()
}
func (bc *fakeBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block { return nil }
func (bc *fakeBlockChain) GetHeader(hash common.Hash, number uint64) *block.Header {
	header := bc.currentBlock.Header()
	if header != nil && header.Hash() == hash && header.Number().Uint64() == number {
		return header
	}
	return nil
}
func (bc *fakeBlockChain) GetHeaderByHash(hash common.Hash) *block.Header           { return nil }
func (bc *fakeBlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts        { return nil }
func (bc *fakeBlockChain) ContractCode(hash common.Hash) ([]byte, error)            { return []byte{}, nil }
func (bc *fakeBlockChain) ValidatorCode(hash common.Hash) ([]byte, error)           { return []byte{}, nil }
func (bc *fakeBlockChain) ShardID() uint32                                          { return 0 }
func (bc *fakeBlockChain) ReadShardState(epoch *big.Int) (*shard.State, error)      { return nil, nil }
func (bc *fakeBlockChain) TrieDB() *trie.Database                                   { return nil }
func (bc *fakeBlockChain) TrieNode(hash common.Hash) ([]byte, error)                { return []byte{}, nil }
func (bc *fakeBlockChain) WriteCommitSig(blockNum uint64, lastCommits []byte) error { return nil }
func (bc *fakeBlockChain) GetHeaderByNumber(number uint64) *block.Header            { return nil }
func (bc *fakeBlockChain) ReadValidatorList() ([]common.Address, error)             { return nil, nil }
func (bc *fakeBlockChain) ReadCommitSig(blockNum uint64) ([]byte, error)            { return nil, nil }
func (bc *fakeBlockChain) ReadBlockRewardAccumulator(uint64) (*big.Int, error)      { return nil, nil }
func (bc *fakeBlockChain) ValidatorCandidates() []common.Address                    { return nil }
func (cr *fakeBlockChain) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorSnapshotAtEpoch(epoch *big.Int, offender common.Address) (*types2.ValidatorSnapshot, error) {
	return &types2.ValidatorSnapshot{
		Validator: makeDefaultValidatorWrapper(),
		Epoch:     epoch,
	}, nil
}
func (bc *fakeBlockChain) ReadValidatorInformation(addr common.Address) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *fakeBlockChain) Config() *params.ChainConfig {
	return &bc.config
}
func (cr *fakeBlockChain) StateAt(root common.Hash) (*state.DB, error) {
	return nil, nil
}
func (cr *fakeBlockChain) Snapshots() *snapshot.Tree {
	return nil
}
func (bc *fakeBlockChain) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return nil, nil
}
func (bc *fakeBlockChain) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	return &bc.superCommittee, nil
}

//
// Fake header for testing
//

func makeFakeHeader() *block.Header {
	h := blockfactory.NewTestHeader()
	h.SetCoinbase(leaderAddr)
	return h
}

//
// Utilities for testing
//

func makeTestAddress(item interface{}) common.Address {
	s := fmt.Sprintf("harmony.one.%v", item)
	return common.BytesToAddress([]byte(s))
}

func makeVoteData(kp blsKeyPair, block *types.Block) slash.Vote {
	return slash.Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{kp.Pub()},
		BlockHeaderHash: block.Hash(),
		Signature:       kp.Sign(block),
	}
}

func TestVerifyHeaderTimestampValidation(t *testing.T) {
	chain := makeFakeBlockChain()
	eng := NewEngine()

	parent := blockfactory.NewTestHeader()
	parent.SetNumber(big.NewInt(doubleSignBlockNumber))
	parent.SetEpoch(big.NewInt(currentEpoch))
	parent.SetTime(big.NewInt(time.Now().Unix()))
	chain.currentBlock = *types.NewBlockWithHeader(parent)
	parent = chain.CurrentHeader()

	mkChild := func(ts int64) *block.Header {
		h := blockfactory.NewTestHeader()
		h.SetParentHash(parent.Hash())
		h.SetNumber(new(big.Int).Add(parent.Number(), big.NewInt(1)))
		h.SetEpoch(parent.Epoch())
		h.SetTime(big.NewInt(ts))
		return h
	}

	if err := eng.VerifyHeader(chain, mkChild(parent.Time().Int64()+1), false); err != nil {
		t.Fatalf("expected valid timestamp, got %v", err)
	}

	if err := eng.VerifyHeader(chain, mkChild(parent.Time().Int64()), false); err == nil {
		t.Fatal("expected error for non-increasing timestamp")
	}

	if err := eng.VerifyHeader(chain, mkChild(time.Now().Add(allowedFutureBlockTime+time.Second).Unix()), false); err != engine.ErrFutureBlock {
		t.Fatalf("expected ErrFutureBlock, got %v", err)
	}
}

func TestVerifyHeaderTimestampValidationBackwardCompatibleBeforeFork(t *testing.T) {
	chain := makeFakeBlockChain()
	eng := NewEngine()

	// Keep activation epoch above the header epoch.
	chain.config.TimestampValidationEpoch = big.NewInt(100)

	parent := blockfactory.NewTestHeader()
	parent.SetNumber(big.NewInt(doubleSignBlockNumber))
	parent.SetEpoch(big.NewInt(4))
	parent.SetTime(big.NewInt(time.Now().Unix()))
	chain.currentBlock = *types.NewBlockWithHeader(parent)
	parent = chain.CurrentHeader()

	mkChild := func(ts int64) *block.Header {
		h := blockfactory.NewTestHeader()
		h.SetParentHash(parent.Hash())
		h.SetNumber(new(big.Int).Add(parent.Number(), big.NewInt(1)))
		h.SetEpoch(parent.Epoch())
		h.SetTime(big.NewInt(ts))
		return h
	}

	older := mkChild(parent.Time().Int64() - 1)
	if err := eng.VerifyHeader(chain, older, false); err != nil {
		t.Fatalf("expected old timestamp allowed pre-fork, got %v", err)
	}

	future := mkChild(time.Now().Add(allowedFutureBlockTime + time.Minute).Unix())
	if err := eng.VerifyHeader(chain, future, false); err != nil {
		t.Fatalf("expected future timestamp allowed pre-fork, got %v", err)
	}
}

func TestVerifiedSigCacheKeyIncludesShardID(t *testing.T) {
	hash := common.Hash{1, 2, 3}
	var sig bls.SerializedSignature
	bitmap := []byte{0xff}

	if newVerifiedSigKey(1, hash, sig, bitmap) == newVerifiedSigKey(2, hash, sig, bitmap) {
		t.Fatal("verified signature cache keys must include shard ID")
	}
}

// setupTimestampValidationChain returns a chain wired with the timestamp
// validation fork active, the parent header committed at parentTime, and a
// helper to build child headers parented to it.
func setupTimestampValidationChain(t *testing.T, parentTime int64) (
	*fakeBlockChain, *engineImpl, func(ts int64) *block.Header,
) {
	t.Helper()
	chain := makeFakeBlockChain()
	eng := NewEngine()

	// Force fork active for any epoch >= 0.
	chain.config.TimestampValidationEpoch = big.NewInt(0)

	parent := blockfactory.NewTestHeader()
	parent.SetNumber(big.NewInt(doubleSignBlockNumber))
	parent.SetEpoch(big.NewInt(currentEpoch))
	parent.SetTime(big.NewInt(parentTime))
	chain.currentBlock = *types.NewBlockWithHeader(parent)
	parent = chain.CurrentHeader()

	mkChild := func(ts int64) *block.Header {
		h := blockfactory.NewTestHeader()
		h.SetParentHash(parent.Hash())
		h.SetNumber(new(big.Int).Add(parent.Number(), big.NewInt(1)))
		h.SetEpoch(parent.Epoch())
		h.SetTime(big.NewInt(ts))
		return h
	}
	return chain, eng, mkChild
}

// TestVerifyHeaderTimestamp_Monotonic covers the strict-monotonic rule
// (header.Time MUST be > parent.Time) at the boundary.
func TestVerifyHeaderTimestamp_Monotonic(t *testing.T) {
	now := time.Now().Unix()
	chain, eng, mkChild := setupTimestampValidationChain(t, now)

	tests := []struct {
		name        string
		headerTime  int64
		expectError bool
	}{
		{"equal_to_parent_rejected", now, true},
		{"one_below_parent_rejected", now - 1, true},
		{"far_below_parent_rejected", now - 1000, true},
		{"one_above_parent_accepted", now + 1, false},
		{"small_step_accepted", now + 2, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := eng.VerifyHeader(chain, mkChild(tc.headerTime), false)
			if tc.expectError && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

// TestVerifyHeaderTimestamp_WallClockFutureLimit covers the
// allowedFutureBlockTime ceiling (header.Time MUST be <= now + 15s).
func TestVerifyHeaderTimestamp_WallClockFutureLimit(t *testing.T) {
	// Parent right at wall clock so the wall-clock arm is the binding constraint
	// rather than the step arm. (parent + maxStep would otherwise be tighter.)
	now := time.Now().Unix()
	chain, eng, mkChild := setupTimestampValidationChain(t, now)

	skewSec := int64(allowedFutureBlockTime.Seconds())

	tests := []struct {
		name          string
		offsetFromNow int64
		wantErr       error
	}{
		// Inside the wall-clock window AND inside the step window (parent==now):
		{"at_skew_boundary_within_step", skewSec - int64(maxBlockTimeStep.Seconds()), nil},
		// Beyond wall-clock window must be ErrFutureBlock.
		{"one_past_skew_rejected", skewSec + 1, engine.ErrFutureBlock},
		{"far_past_skew_rejected", skewSec + 60, engine.ErrFutureBlock},
		{"way_in_future_rejected", 3600, engine.ErrFutureBlock},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := eng.VerifyHeader(chain, mkChild(now+tc.offsetFromNow), false)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestVerifyHeaderTimestamp_StepLimitNormalOp exercises the per-block
// forward-step bound when parent is roughly aligned with wall clock.
// In this regime, parent+maxStep is the binding constraint.
func TestVerifyHeaderTimestamp_StepLimitNormalOp(t *testing.T) {
	now := time.Now().Unix()
	// Parent slightly behind wall (typical BlockPeriod gap).
	parentTime := now - 2
	chain, eng, mkChild := setupTimestampValidationChain(t, parentTime)

	step := int64(maxBlockTimeStep.Seconds())

	tests := []struct {
		name       string
		headerTime int64
		wantErr    error
	}{
		{"one_above_parent", parentTime + 1, nil},
		{"five_above_parent", parentTime + 5, nil},
		{"at_step_boundary_accepted", parentTime + step, nil},
		{"one_past_step_rejected", parentTime + step + 1, engine.ErrFutureBlock},
		{"well_past_step_rejected", parentTime + step + 5, engine.ErrFutureBlock},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := eng.VerifyHeader(chain, mkChild(tc.headerTime), false)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestVerifyHeaderTimestamp_StallRecovery exercises the wall-aware relaxation
// of the step limit. When parent is far behind wall (a real network stall),
// the validator should accept blocks up to current wall clock so chain time
// catches up in a single block instead of crawling at maxStep/block.
func TestVerifyHeaderTimestamp_StallRecovery(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name       string
		stallSec   int64 // how far parent is behind wall
		headerTime func(parent, wall int64) int64
		wantErr    error
	}{
		{
			name:       "short_view_change_block_at_wall",
			stallSec:   30,
			headerTime: func(_, wall int64) int64 { return wall },
			wantErr:    nil,
		},
		{
			name:       "five_minute_stall_block_at_wall",
			stallSec:   300,
			headerTime: func(_, wall int64) int64 { return wall },
			wantErr:    nil,
		},
		{
			name:       "one_hour_stall_block_at_wall",
			stallSec:   3600,
			headerTime: func(_, wall int64) int64 { return wall },
			wantErr:    nil,
		},
		{
			name:       "stall_block_below_wall_still_above_parent_step",
			stallSec:   300,
			headerTime: func(parent, _ int64) int64 { return parent + 50 },
			wantErr:    nil,
		},
		{
			name:       "stall_block_at_parent_plus_step",
			stallSec:   300,
			headerTime: func(parent, _ int64) int64 { return parent + int64(maxBlockTimeStep.Seconds()) },
			wantErr:    nil,
		},
		{
			name:       "stall_block_beyond_wall_rejected",
			stallSec:   300,
			headerTime: func(_, wall int64) int64 { return wall + int64(allowedFutureBlockTime.Seconds()) + 5 },
			wantErr:    engine.ErrFutureBlock,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parentTime := now - tc.stallSec
			chain, eng, mkChild := setupTimestampValidationChain(t, parentTime)
			err := eng.VerifyHeader(chain, mkChild(tc.headerTime(parentTime, now)), false)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestVerifyHeaderTimestamp_ParentAheadOfWall covers the case where parent is
// already slightly in the future (e.g., from a previous +skew leader). The
// max(parent+maxStep, wall) arm yields parent+maxStep, but the wall+15s arm
// is the actual binding constraint when parent is enough into the future.
func TestVerifyHeaderTimestamp_ParentAheadOfWall(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name        string
		parentAhead int64 // parent.Time = now + parentAhead
		headerTime  int64 // header.Time absolute (computed lazily below)
		wantErr     error
	}{
		// parent = now+5: step arm allows parent+12 = now+17; wall arm allows now+15.
		// header at now+15 should be rejected by step arm? No — step is parent+12=now+17 ≥ now+15 ✓.
		// And wall arm allows ≤ now+15 ✓. So accepted.
		{"parent_ahead_within_both_arms", 5, 0 /* set below */, nil},
		// parent+12 = now+17 > now+15 (wall+15). Reject under wall+15 ceiling.
		{"parent_ahead_violates_wall_ceiling", 5, 0, engine.ErrFutureBlock},
		// header at parent+13 violates step arm too.
		{"parent_ahead_violates_step_arm", 5, 0, engine.ErrFutureBlock},
	}

	// fill in headerTime values that depend on `now`
	tests[0].headerTime = now + 15     // wall+15 exactly
	tests[1].headerTime = now + 16     // one past wall+15
	tests[2].headerTime = now + 5 + 13 // parent+13 also > wall+15 but also fails step

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parentTime := now + tc.parentAhead
			chain, eng, mkChild := setupTimestampValidationChain(t, parentTime)
			err := eng.VerifyHeader(chain, mkChild(tc.headerTime), false)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tc.wantErr != nil && err != tc.wantErr {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

// TestVerifyHeaderTimestamp_CascadingSkewBound is the security test: a
// hypothetical +15s-skewed leader cannot push chain time more than
// maxBlockTimeStep ahead of parent. The validator (with synced clock) MUST
// reject the abusive block.
func TestVerifyHeaderTimestamp_CascadingSkewBound(t *testing.T) {
	now := time.Now().Unix()
	parentTime := now - 2 // typical block period gap, parent slightly behind real time
	chain, eng, mkChild := setupTimestampValidationChain(t, parentTime)

	step := int64(maxBlockTimeStep.Seconds())

	// Honest leader at parent + step is the maximum chain-time advancement.
	if err := eng.VerifyHeader(chain, mkChild(parentTime+step), false); err != nil {
		t.Fatalf("expected boundary accept, got %v", err)
	}
	// Adversarial leader trying to push chain time +15s above real wall must be
	// rejected (parent + step is the binding constraint, not wall + 15s).
	skewedHeader := mkChild(now + int64(allowedFutureBlockTime.Seconds()))
	if err := eng.VerifyHeader(chain, skewedHeader, false); err != engine.ErrFutureBlock {
		t.Fatalf("expected ErrFutureBlock for cascading-skew push, got %v", err)
	}
}

// TestVerifyHeaderTimestamp_UnknownAncestor verifies the existing precedence
// of the ErrUnknownAncestor check vs. the new timestamp checks.
func TestVerifyHeaderTimestamp_UnknownAncestor(t *testing.T) {
	chain := makeFakeBlockChain()
	eng := NewEngine()
	chain.config.TimestampValidationEpoch = big.NewInt(0)

	orphan := blockfactory.NewTestHeader()
	orphan.SetParentHash(common.HexToHash("0xdead"))
	orphan.SetNumber(big.NewInt(doubleSignBlockNumber + 1))
	orphan.SetEpoch(big.NewInt(currentEpoch))
	orphan.SetTime(big.NewInt(time.Now().Unix()))

	if err := eng.VerifyHeader(chain, orphan, false); err != engine.ErrUnknownAncestor {
		t.Fatalf("expected ErrUnknownAncestor, got %v", err)
	}
}
