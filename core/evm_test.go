package core

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	staking "github.com/harmony-one/harmony/staking/types"
)

func getTestEnvironment(testBankKey ecdsa.PrivateKey) (*BlockChainImpl, *state.DB, *block.Header, ethdb.Database) {
	// initialize
	var (
		testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds   = new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(40000))
		chainConfig     = params.TestChainConfig
		blockFactory    = blockfactory.ForTest
		database        = rawdb.NewMemoryDatabase()
		gspec           = Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 0,
		}
		engine = chain2.NewEngine()
	)
	genesis := gspec.MustCommit(database)

	// fake blockchain
	cacheConfig := &CacheConfig{SnapshotLimit: 0}
	chain, _ := NewBlockChain(database, nil, nil, cacheConfig, gspec.Config, engine, vm.Config{})
	db, _ := chain.StateAt(genesis.Root())

	// make a fake block header (use epoch 1 so that locked tokens can be tested)
	header := blockFactory.NewHeader(common.Big0)

	return chain, db, header, database
}

func TestEVMStaking(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, database := getTestEnvironment(*key)
	batch := database.NewBatch()
	header.SetNumber(big.NewInt(1))

	// fake transaction
	tx := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	// transaction as message (chainId = 2)
	msg, _ := tx.AsMessage(types.NewEIP155Signer(common.Big2))
	// context
	ctx := NewEVMContext(msg, header, chain, nil /* coinbase */)

	// createValidator test
	createValidator := sampleCreateValidator(*key)
	err := ctx.CreateValidator(db, nil, &createValidator)
	if err != nil {
		t.Errorf("Got error %v in CreateValidator", err)
	}
	// write it to snapshot so that we can use it in edit
	// use a copy because we are editing below (wrapper.Delegations)
	wrapper, err := db.ValidatorWrapper(createValidator.ValidatorAddress, false, true)
	err = chain.WriteValidatorSnapshot(batch, &staking.ValidatorSnapshot{Validator: wrapper, Epoch: header.Epoch()})
	// also write the delegation so we can use it in CollectRewards
	selfIndex := staking.DelegationIndex{
		ValidatorAddress: createValidator.ValidatorAddress,
		Index:            uint64(0),
		BlockNum:         common.Big0, // block number at which delegation starts
	}
	err = chain.writeDelegationsByDelegator(batch, createValidator.ValidatorAddress, []staking.DelegationIndex{selfIndex})

	// editValidator test
	editValidator := sampleEditValidator(*key)
	editValidator.SlotKeyToRemove = &createValidator.SlotPubKeys[0]
	err = ctx.EditValidator(db, nil, &editValidator)
	if err != nil {
		t.Errorf("Got error %v in EditValidator", err)
	}

	// delegate test
	delegate := sampleDelegate(*key)
	// add undelegations in epoch0
	wrapper.Delegations[0].Undelegations = []staking.Undelegation{
		{
			Amount: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(10000)),
			Epoch:  common.Big0,
		},
	}
	// redelegate using epoch1, so that we can cover the locked tokens use case as well
	ctx2 := NewEVMContext(msg, blockfactory.ForTest.NewHeader(common.Big1), chain, nil)
	err = db.UpdateValidatorWrapper(wrapper.Address, wrapper)
	err = ctx2.Delegate(db, nil, &delegate)
	if err != nil {
		t.Errorf("Got error %v in Delegate", err)
	}

	// undelegate test
	undelegate := sampleUndelegate(*key)
	err = ctx.Undelegate(db, nil, &undelegate)
	if err != nil {
		t.Errorf("Got error %v in Undelegate", err)
	}

	// collectRewards test
	collectRewards := sampleCollectRewards(*key)
	// add block rewards to make sure there are some to collect
	wrapper.Delegations[0].Undelegations = []staking.Undelegation{}
	wrapper.Delegations[0].Reward = common.Big257
	db.UpdateValidatorWrapper(wrapper.Address, wrapper)
	err = ctx.CollectRewards(db, nil, &collectRewards)
	if err != nil {
		t.Errorf("Got error %v in CollectRewards", err)
	}

	//// migration test - when from has no delegations
	//toKey, _ := crypto.GenerateKey()
	//migration := sampleMigrationMsg(*toKey, *key)
	//delegates, err := ctx.MigrateDelegations(db, &migration)
	//expectedError := errors.New("No delegations to migrate")
	//if err != nil && expectedError.Error() != err.Error() {
	//	t.Errorf("Got error %v in MigrateDelegations but expected %v", err, expectedError)
	//}
	//if len(delegates) > 0 {
	//	t.Errorf("Got delegates to migrate when none were expected")
	//}
	//// migration test - when from == to
	//migration = sampleMigrationMsg(*toKey, *toKey)
	//delegates, err = ctx.MigrateDelegations(db, &migration)
	//expectedError = errors.New("From and To are the same address")
	//if err != nil && expectedError.Error() != err.Error() {
	//	t.Errorf("Got error %v in MigrateDelegations but expected %v", err, expectedError)
	//}
	//if len(delegates) > 0 {
	//	t.Errorf("Got delegates to migrate when none were expected")
	//}
	//// migration test - when `to` has no delegations
	//snapshot := db.Snapshot()
	//migration = sampleMigrationMsg(*key, *toKey)
	//wrapper, _ = db.ValidatorWrapper(wrapper.Address, true, false)
	//expectedAmount := wrapper.Delegations[0].Amount
	//delegates, err = ctx.MigrateDelegations(db, &migration)
	//if err != nil {
	//	t.Errorf("Got error %v in MigrateDelegations", err)
	//}
	//if len(delegates) != 1 {
	//	t.Errorf("Got %d delegations to migrate, expected just 1", len(delegates))
	//}
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, false, true)
	//if wrapper.Delegations[0].Amount.Cmp(common.Big0) != 0 {
	//	t.Errorf("Expected delegation at index 0 to have amount 0, but it has amount %d",
	//		wrapper.Delegations[0].Amount)
	//}
	//if wrapper.Delegations[1].Amount.Cmp(expectedAmount) != 0 {
	//	t.Errorf("Expected delegation at index 1 to have amount %d, but it has amount %d",
	//		expectedAmount, wrapper.Delegations[1].Amount)
	//}
	//db.RevertToSnapshot(snapshot)
	//snapshot = db.Snapshot()
	//// migration test - when `from` delegation amount = 0 and no undelegations
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, false, true)
	//wrapper.Delegations[0].Undelegations = make([]staking.Undelegation, 0)
	//wrapper.Delegations[0].Amount = common.Big0
	//wrapper.Status = effective.Inactive
	//err = db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper)
	//if err != nil {
	//	t.Errorf("Got error %v in UpdateValidatorWrapperWithRevert", err)
	//}
	//delegates, err = ctx.MigrateDelegations(db, &migration)
	//if err != nil {
	//	t.Errorf("Got error %v in MigrateDelegations", err)
	//}
	//if len(delegates) != 0 {
	//	t.Errorf("Got %d delegations to migrate, expected none", len(delegates))
	//}
	//db.RevertToSnapshot(snapshot)
	//snapshot = db.Snapshot()
	//// migration test - when `to` has one delegation
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, false, true)
	//wrapper.Delegations = append(wrapper.Delegations, staking.NewDelegation(
	//	migration.To, new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100))))
	//expectedAmount = big.NewInt(0).Add(
	//	wrapper.Delegations[0].Amount, wrapper.Delegations[1].Amount,
	//)
	//err = db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper)
	//if err != nil {
	//	t.Errorf("Got error %v in UpdateValidatorWrapperWithRevert", err)
	//}
	//delegates, err = ctx.MigrateDelegations(db, &migration)
	//if err != nil {
	//	t.Errorf("Got error %v in MigrateDelegations", err)
	//}
	//if len(delegates) != 1 {
	//	t.Errorf("Got %d delegations to migrate, expected just 1", len(delegates))
	//}
	//// read from updated wrapper
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, true, false)
	//if wrapper.Delegations[0].Amount.Cmp(common.Big0) != 0 {
	//	t.Errorf("Expected delegation at index 0 to have amount 0, but it has amount %d",
	//		wrapper.Delegations[0].Amount)
	//}
	//if wrapper.Delegations[1].Amount.Cmp(expectedAmount) != 0 {
	//	t.Errorf("Expected delegation at index 1 to have amount %d, but it has amount %d",
	//		expectedAmount, wrapper.Delegations[1].Amount)
	//}
	//db.RevertToSnapshot(snapshot)
	//snapshot = db.Snapshot()
	//// migration test - when `to` has one undelegation in the current epoch and so does `from`
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, false, true)
	//delegation := staking.NewDelegation(migration.To, big.NewInt(0))
	//delegation.Undelegations = []staking.Undelegation{
	//	staking.Undelegation{
	//		Amount: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)),
	//		Epoch:  common.Big0,
	//	},
	//}
	//wrapper.Delegations[0].Undelegate(
	//	big.NewInt(0), new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(100)),
	//)
	//expectedAmount = big.NewInt(0).Add(
	//	wrapper.Delegations[0].Amount, big.NewInt(0),
	//)
	//wrapper.Delegations = append(wrapper.Delegations, delegation)
	//expectedDelegations := []staking.Delegation{
	//	staking.Delegation{
	//		DelegatorAddress: wrapper.Address,
	//		Amount:           big.NewInt(0),
	//		Reward:           big.NewInt(0),
	//		Undelegations:    []staking.Undelegation{},
	//	},
	//	staking.Delegation{
	//		DelegatorAddress: crypto.PubkeyToAddress(toKey.PublicKey),
	//		Amount:           expectedAmount,
	//		Undelegations: []staking.Undelegation{
	//			staking.Undelegation{
	//				Amount: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(200)),
	//				Epoch:  common.Big0,
	//			},
	//		},
	//	},
	//}
	//err = db.UpdateValidatorWrapperWithRevert(wrapper.Address, wrapper)
	//if err != nil {
	//	t.Errorf("Got error %v in UpdateValidatorWrapperWithRevert", err)
	//}
	//delegates, err = ctx.MigrateDelegations(db, &migration)
	//if err != nil {
	//	t.Errorf("Got error %v in MigrateDelegations", err)
	//}
	//if len(delegates) != 1 {
	//	t.Errorf("Got %d delegations to migrate, expected just 1", len(delegates))
	//}
	//// read from updated wrapper
	//wrapper, _ = db.ValidatorWrapper(createValidator.ValidatorAddress, true, false)
	//// and finally the check for delegations being equal
	//if err := staketest.CheckDelegationsEqual(
	//	wrapper.Delegations,
	//	expectedDelegations,
	//); err != nil {
	//	t.Errorf("Got error %s in CheckDelegationsEqual", err)
	//}

	//// test for migration gas
	//db.RevertToSnapshot(snapshot)
	//// to calculate gas we need to test 0 / 1 / 2 delegations to migrate as per ReadDelegationsByDelegator
	//// case 0
	//evm := vm.NewEVM(ctx, db, params.TestChainConfig, vm.Config{})
	//gas, err := ctx.CalculateMigrationGas(db, &staking.MigrationMsg{
	//	From: crypto.PubkeyToAddress(toKey.PublicKey),
	//	To:   crypto.PubkeyToAddress(key.PublicKey),
	//}, evm.ChainConfig().IsS3(evm.EpochNumber), evm.ChainConfig().IsS3(evm.EpochNumber))
	//var expectedGasMin uint64 = params.TxGas
	//if err != nil {
	//	t.Errorf("Gas error %s", err)
	//}
	//if gas < expectedGasMin {
	//	t.Errorf("Gas for 0 migration was expected to be at least %d but got %d", expectedGasMin, gas)
	//}
	//// case 1
	//gas, err = ctx.CalculateMigrationGas(db, &migration,
	//	evm.ChainConfig().IsS3(evm.EpochNumber), evm.ChainConfig().IsS3(evm.EpochNumber))
	//expectedGasMin = params.TxGas
	//if err != nil {
	//	t.Errorf("Gas error %s", err)
	//}
	//if gas < expectedGasMin {
	//	t.Errorf("Gas for 1 migration was expected to be at least %d but got %d", expectedGasMin, gas)
	//}
	//// case 2
	//createValidator = sampleCreateValidator(*toKey)
	//db.AddBalance(createValidator.ValidatorAddress, createValidator.Amount)
	//err = ctx.CreateValidator(db, &createValidator)
	//delegate = sampleDelegate(*toKey)
	//delegate.DelegatorAddress = crypto.PubkeyToAddress(key.PublicKey)
	//_ = ctx.Delegate(db, &delegate)
	//delegationIndex := staking.DelegationIndex{
	//	ValidatorAddress: crypto.PubkeyToAddress(toKey.PublicKey),
	//	Index:            uint64(1),
	//	BlockNum:         common.Big0,
	//}
	//err = chain.writeDelegationsByDelegator(batch, migration.From, []staking.DelegationIndex{selfIndex, delegationIndex})
	//gas, err = ctx.CalculateMigrationGas(db, &migration,
	//	evm.ChainConfig().IsS3(evm.EpochNumber), evm.ChainConfig().IsS3(evm.EpochNumber))
	//expectedGasMin = 2 * params.TxGas
	//if err != nil {
	//	t.Errorf("Gas error %s", err)
	//}
	//if gas < expectedGasMin {
	//	t.Errorf("Gas for 2 migrations was expected to be at least %d but got %d", expectedGasMin, gas)
	//}
}

func generateBLSKeyAndSig() (bls.SerializedPublicKey, bls.SerializedSignature) {
	p := &bls_core.PublicKey{}
	p.DeserializeHexStr(testBLSPubKey)
	pub := bls.SerializedPublicKey{}
	pub.FromLibBLSPublicKey(p)
	messageBytes := []byte(staking.BLSVerificationStr)
	privateKey := &bls_core.SecretKey{}
	privateKey.DeserializeHexStr(testBLSPrvKey)
	msgHash := hash.Keccak256(messageBytes)
	signature := privateKey.SignHash(msgHash[:])
	var sig bls.SerializedSignature
	copy(sig[:], signature.Serialize())
	return pub, sig
}

func sampleCreateValidator(key ecdsa.PrivateKey) staking.CreateValidator {
	pub, sig := generateBLSKeyAndSig()

	ra, _ := numeric.NewDecFromStr("0.7")
	maxRate, _ := numeric.NewDecFromStr("1")
	maxChangeRate, _ := numeric.NewDecFromStr("0.5")
	return staking.CreateValidator{
		Description: staking.Description{
			Name:            "SuperHero",
			Identity:        "YouWouldNotKnow",
			Website:         "Secret Website",
			SecurityContact: "LicenseToKill",
			Details:         "blah blah blah",
		},
		CommissionRates: staking.CommissionRates{
			Rate:          ra,
			MaxRate:       maxRate,
			MaxChangeRate: maxChangeRate,
		},
		MinSelfDelegation:  new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(10000)),
		MaxTotalDelegation: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(20000)),
		ValidatorAddress:   crypto.PubkeyToAddress(key.PublicKey),
		SlotPubKeys:        []bls.SerializedPublicKey{pub},
		SlotKeySigs:        []bls.SerializedSignature{sig},
		Amount:             new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(15000)),
	}
}

func sampleEditValidator(key ecdsa.PrivateKey) staking.EditValidator {
	// generate new key and sig
	slotKeyToAdd, slotKeyToAddSig := generateBLSKeyAndSig()

	// rate
	ra, _ := numeric.NewDecFromStr("0.8")

	return staking.EditValidator{
		Description: staking.Description{
			Name:            "Alice",
			Identity:        "alice",
			Website:         "alice.harmony.one",
			SecurityContact: "Bob",
			Details:         "Don't mess with me!!!",
		},
		CommissionRate:     &ra,
		MinSelfDelegation:  new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(10000)),
		MaxTotalDelegation: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(20000)),
		SlotKeyToRemove:    nil,
		SlotKeyToAdd:       &slotKeyToAdd,
		SlotKeyToAddSig:    &slotKeyToAddSig,
		ValidatorAddress:   crypto.PubkeyToAddress(key.PublicKey),
	}
}

func sampleDelegate(key ecdsa.PrivateKey) staking.Delegate {
	address := crypto.PubkeyToAddress(key.PublicKey)
	return staking.Delegate{
		DelegatorAddress: address,
		ValidatorAddress: address,
		// additional delegation of 1000 ONE
		Amount: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(1000)),
	}
}

func sampleUndelegate(key ecdsa.PrivateKey) staking.Undelegate {
	address := crypto.PubkeyToAddress(key.PublicKey)
	return staking.Undelegate{
		DelegatorAddress: address,
		ValidatorAddress: address,
		// undelegate the delegation of 1000 ONE
		Amount: new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(1000)),
	}
}

func sampleCollectRewards(key ecdsa.PrivateKey) staking.CollectRewards {
	address := crypto.PubkeyToAddress(key.PublicKey)
	return staking.CollectRewards{
		DelegatorAddress: address,
	}
}

func sampleMigrationMsg(from ecdsa.PrivateKey, to ecdsa.PrivateKey) staking.MigrationMsg {
	fromAddress := crypto.PubkeyToAddress(from.PublicKey)
	toAddress := crypto.PubkeyToAddress(to.PublicKey)
	return staking.MigrationMsg{
		From: fromAddress,
		To:   toAddress,
	}
}

func TestWriteCapablePrecompilesIntegration(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	// gp := new(GasPool).AddGas(math.MaxUint64)
	tx := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	msg, _ := tx.AsMessage(types.NewEIP155Signer(common.Big2))
	ctx := NewEVMContext(msg, header, chain, nil /* coinbase */)
	evm := vm.NewEVM(ctx, db, params.TestChainConfig, vm.Config{})
	// interpreter := vm.NewEVMInterpreter(evm, vm.Config{})
	address := common.BytesToAddress([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 252})
	// caller ContractRef, addr common.Address, input []byte, gas uint64, value *big.Int)
	_, _, err := evm.Call(vm.AccountRef(common.Address{}), address,
		[]byte{109, 107, 47, 119, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 19},
		math.MaxUint64, new(big.Int))
	expectedError := errors.New("abi: cannot marshal in to go type: length insufficient 31 require 32")
	if err != nil {
		if err.Error() != expectedError.Error() {
			t.Errorf(fmt.Sprintf("Got error %v in evm.Call but expected %v", err, expectedError))
		}
	}

	// now add a validator, and send its address as caller
	createValidator := sampleCreateValidator(*key)
	err = ctx.CreateValidator(db, nil, &createValidator)
	_, _, err = evm.Call(vm.AccountRef(common.Address{}),
		createValidator.ValidatorAddress,
		[]byte{},
		math.MaxUint64, new(big.Int))
	if err != nil {
		t.Errorf(fmt.Sprintf("Got error %v in evm.Call", err))
	}

	// now without staking precompile
	cfg := params.TestChainConfig
	cfg.StakingPrecompileEpoch = big.NewInt(10000000)
	evm = vm.NewEVM(ctx, db, cfg, vm.Config{})
	_, _, err = evm.Call(vm.AccountRef(common.Address{}),
		createValidator.ValidatorAddress,
		[]byte{},
		math.MaxUint64, new(big.Int))
	if err != nil {
		t.Errorf(fmt.Sprintf("Got error %v in evm.Call", err))
	}
}
