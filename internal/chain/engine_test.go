package chain

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

type fakeReader struct {
	core.FakeChainReader
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
	chain, _ := core.NewBlockChain(database, nil, nil, nil, vm.Config{}, nil)
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

func TestBulkPruneStaleStakingData(t *testing.T) {
	blockFactory := blockfactory.ForTest
	header := blockFactory.NewHeader(common.Big0) // epoch
	chain := fakeReader{core.FakeChainReader{InternalConfig: params.LocalnetChainConfig}}
	db := getDatabase()
	// now make the two wrappers and store them
	wrapper := sampleWrapper(validator1)
	wrapper.Status = effective.Inactive
	wrapper.Delegations = staking.Delegations{
		staking.NewDelegation(wrapper.Address, big.NewInt(0)),
		staking.NewDelegation(delegator1, big.NewInt(0)),                                                    // stale
		staking.NewDelegation(delegator2, big.NewInt(0)),                                                    // stale
		staking.NewDelegation(delegator3, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))), // not stale
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
	// inputs are: validator1
	// 0 -> self delegation
	// 1 -> stale delegation
	// 2 -> stale delegation
	// 3 -> 0 delegation but not stale

	// validator2:
	// 0 -> self delegation (validator2)
	// 1 -> non stale delegation (with amount) (delegator1)
	// 2 -> stale delegation (delegator2)
	// 3 -> non stale delegation (delegator3)
	// 4 -> stale delegation (validator1)

	expected := make(map[common.Address](map[common.Address]uint64))

	expected[delegator1] = make(map[common.Address]uint64)
	expected[delegator1][validator1] = math.MaxUint64

	expected[delegator2] = make(map[common.Address]uint64)
	expected[delegator2][validator1] = math.MaxUint64
	expected[delegator2][validator2] = math.MaxUint64

	expected[delegator3] = make(map[common.Address]uint64)
	expected[delegator3][validator1] = 2
	expected[delegator3][validator2] = 1

	expected[validator1] = make(map[common.Address]uint64)
	expected[validator1][validator2] = math.MaxUint64

	var delegationsToAlter map[common.Address](map[common.Address]uint64)
	var err error
	if delegationsToAlter, err = bulkPruneStaleStakingData(&chain, header, db, nil); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if err := staketest.CompareDelegationsToAlter(expected, delegationsToAlter); err != nil {
		t.Error(err)
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
		staking.NewDelegation(delegator1, big.NewInt(0)),                                                    // stale
		staking.NewDelegation(delegator2, big.NewInt(0)),                                                    // stale
		staking.NewDelegation(delegator3, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))), // not stale
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
	// inputs are: validator1
	// 0 -> self delegation
	// 1 -> stale delegation
	// 2 -> stale delegation
	// 3 -> 0 delegation but not stale

	// validator2:
	// 0 -> self delegation (validator2)
	// 1 -> non stale delegation (with amount) (delegator1)
	// 2 -> stale delegation (delegator2)
	// 3 -> non stale delegation (delegator3)
	// 4 -> stale delegation (validator1)

	expected := make(map[common.Address](map[common.Address]uint64))

	expected[delegator1] = make(map[common.Address]uint64)
	expected[delegator1][validator1] = math.MaxUint64

	expected[delegator2] = make(map[common.Address]uint64)
	expected[delegator2][validator1] = math.MaxUint64
	expected[delegator2][validator2] = math.MaxUint64

	expected[delegator3] = make(map[common.Address]uint64)
	expected[delegator3][validator1] = math.MaxUint64 // manually marked for removal (last so no other changes)
	expected[delegator3][validator2] = 1

	expected[validator1] = make(map[common.Address]uint64)
	expected[validator1][validator2] = math.MaxUint64

	delegationsToAlter := make(map[common.Address](map[common.Address]uint64))
	delegationsToAlter[delegator3] = make(map[common.Address]uint64)
	delegationsToAlter[delegator3][validator1] = math.MaxUint64
	var err error
	// pay out undelegations
	delegationsToRemove := make(map[common.Address][]uint64)
	delegationsToRemove[validator1] = []uint64{1, 2}
	delegationsToRemove[validator2] = []uint64{2, 4}
	// and besides this, remove
	if delegationsToAlter, err = pruneStaleStakingData(&chain, header, db, delegationsToAlter, delegationsToRemove); err != nil {
		t.Fatalf("Got error %v", err)
	}
	if err := staketest.CompareDelegationsToAlter(expected, delegationsToAlter); err != nil {
		t.Error(err)
	}
}
