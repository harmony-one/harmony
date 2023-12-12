package chain

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/trie"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/engine"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"

	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	types2 "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/state/snapshot"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"

	"github.com/harmony-one/harmony/internal/params"
)

var (
	bigOne        = big.NewInt(1e18)
	tenKOnes      = new(big.Int).Mul(big.NewInt(10000), bigOne)
	twentyKOnes   = new(big.Int).Mul(big.NewInt(20000), bigOne)
	fourtyKOnes   = new(big.Int).Mul(big.NewInt(40000), bigOne)
	thousandKOnes = new(big.Int).Mul(big.NewInt(1000000), bigOne)
)

type fakeReader struct {
	core.FakeChainReader
}

const (
	// validator creation parameters
	doubleSignShardID     = 0
	doubleSignEpoch       = 4
	doubleSignBlockNumber = 37
	doubleSignViewID      = 38

	creationHeight  = 33
	lastEpochInComm = 5
	currentEpoch    = 5

	numShard        = 4
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

func makeTestAddr(item interface{}) common.Address {
	s := fmt.Sprintf("harmony-one-%v", item)
	return common.BytesToAddress([]byte(s))
}

var (
	validator1 = makeTestAddr("validator1")
	validator2 = makeTestAddr("validator2")
	delegator1 = makeTestAddr("delegator1")
	delegator2 = makeTestAddr("delegator2")
	delegator3 = makeTestAddr("delegator3")
)

var (
	defaultDesc = staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}

	defaultCommissionRates = staking.CommissionRates{
		Rate:          numeric.NewDecWithPrec(1, 1),
		MaxRate:       numeric.NewDecWithPrec(9, 1),
		MaxChangeRate: numeric.NewDecWithPrec(5, 1),
	}
)

func (cr *fakeReader) ReadValidatorList() ([]common.Address, error) {
	return []common.Address{validator1, validator2}, nil
}

func getDatabase() *state.DB {
	database := rawdb.NewMemoryDatabase()
	gspec := core.Genesis{Factory: blockfactory.ForTest}
	genesis := gspec.MustCommit(database)
	chain, _ := core.NewBlockChain(database, nil, nil, nil, nil, nil, vm.Config{}) //doublecheck this
	db, _ := chain.StateAt(genesis.Root())
	return db
}

func generateBLSKeyAndSig() (bls.SerializedPublicKey, bls.SerializedSignature) {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub bls.SerializedPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig bls.SerializedSignature
	copy(shardSig[:], sig.Serialize())

	return shardPub, shardSig
}

func sampleWrapper(address common.Address) *staking.ValidatorWrapper {
	pub, _ := generateBLSKeyAndSig()
	v := staking.Validator{
		Address:              address,
		SlotPubKeys:          []bls.SerializedPublicKey{pub},
		LastEpochInCommittee: new(big.Int),
		MinSelfDelegation:    staketest.DefaultMinSelfDel,
		MaxTotalDelegation:   staketest.DefaultMaxTotalDel,
		Commission: staking.Commission{
			CommissionRates: defaultCommissionRates,
			UpdateHeight:    big.NewInt(100),
		},
		Description:    defaultDesc,
		CreationHeight: big.NewInt(100),
	}
	// ds := staking.Delegations{
	//   staking.NewDelegation(address, big.NewInt(0)),
	// }
	w := &staking.ValidatorWrapper{
		Validator:   v,
		BlockReward: big.NewInt(0),
	}
	w.Counters.NumBlocksSigned = common.Big0
	w.Counters.NumBlocksToSign = common.Big0
	return w
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
func (bc *fakeBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block    { return nil }
func (bc *fakeBlockChain) GetHeader(hash common.Hash, number uint64) *block.Header  { return nil }
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
	return params.LocalnetChainConfig
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
	return nil, nil
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

func TestPruneStaleStakingData(t *testing.T) {
	blockFactory := blockfactory.ForTest
	header := blockFactory.NewHeader(common.Big0) // epoch

	chain := fakeReader{core.FakeChainReader{InternalConfig: params.LocalnetChainConfig}}
	db := getDatabase()
	// now make the two wrappers and store them
	wrapper := sampleWrapper(validator1)
	wrapper.Status = effective.Inactive
	wrapper.Delegations = staking.Delegations{
		staking.NewDelegation(wrapper.Address, big.NewInt(0)),
		staking.NewDelegation(delegator1, big.NewInt(0)),
		staking.NewDelegation(delegator2, big.NewInt(0)),
		staking.NewDelegation(delegator3, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
	}
	if err := wrapper.Delegations[3].Undelegate(
		big.NewInt(2), new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)), nil,
	); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if wrapper.Delegations[3].Amount.Cmp(common.Big0) != 0 {
		t.Fatalf("Expected 0 delegation but got %v", wrapper.Delegations[3].Amount)
	}
	if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Fatalf("Got error %v", err)
	}
	wrapper = sampleWrapper(validator2)
	wrapper.Status = effective.Active
	wrapper.Delegations = staking.Delegations{
		staking.NewDelegation(wrapper.Address, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(10000))),
		staking.NewDelegation(delegator1, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))),
		staking.NewDelegation(delegator2, big.NewInt(0)),
		staking.NewDelegation(delegator3, big.NewInt(0)),
		staking.NewDelegation(validator1, big.NewInt(0)),
	}
	wrapper.Delegations[3].Reward = common.Big257
	if err := db.UpdateValidatorWrapper(wrapper.Address, wrapper); err != nil {
		t.Fatalf("Got error %v", err)
	}
	// we expect
	// (1) validator1 to show up with validator2 only (and not validator1 where the delegation is 0)
	// (2) delegator1 to show up with validator1 only (validator2 has amount)
	// (3) delegator2 to show up with both validator1 and validator2
	// (4) delegator3 to show up with neither validator1 (undelegation) nor validator2 (reward)
	delegationsToRemove := make(map[common.Address][]common.Address, 0)
	if err := pruneStaleStakingData(&chain, header, db, delegationsToRemove); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if toRemove, ok := delegationsToRemove[validator1]; ok {
		if len(toRemove) != 1 {
			t.Errorf("Unexpected # of removals for validator1 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) {
					t.Errorf("Found validator1 being removed from validator1's delegations")
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator1]; ok {
		if len(toRemove) != 1 {
			t.Errorf("Unexpected # of removals for delegator1 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if !bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) {
					t.Errorf("Unexpected removal for delegator1; validator1 %s, validator2 %s, validatorAddress %s",
						validator1.Hex(),
						validator2.Hex(),
						validatorAddress.Hex(),
					)
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator2]; ok {
		if len(toRemove) != 2 {
			t.Errorf("Unexpected # of removals for delegator2 %d", len(toRemove))
		}
		if len(toRemove) > 0 {
			for _, validatorAddress := range toRemove {
				if !(bytes.Equal(validatorAddress.Bytes(), validator1.Bytes()) ||
					bytes.Equal(validatorAddress.Bytes(), validator2.Bytes())) {
					t.Errorf("Unexpected removal for delegator2; validator1 %s, validator2 %s, validatorAddress %s",
						validator1.Hex(),
						validator2.Hex(),
						validatorAddress.Hex(),
					)
				}
			}
		}
	}
	if toRemove, ok := delegationsToRemove[delegator3]; ok {
		if len(toRemove) != 0 {
			t.Errorf("Unexpected # of removals for delegator3 %d", len(toRemove))
		}
	}
}
