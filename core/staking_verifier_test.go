package core

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/hash"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

var (
	validatorAddress common.Address
	validatorBalance *big.Int

	defaultMaxTotalDelegation *big.Int
	defaultMinSelfDelegation  *big.Int
	defaultSelfDelegation     *big.Int

	blsKeys blsKeyPool
)

func init() {
	testDataSetup()
}

func TestCheckDuplicateFields(t *testing.T) {
	tests := []struct {
		stateValidators []validatorWrapperParams
		validator       common.Address
		identity        string
		blsKeys         []shard.BLSPublicKey
		expErr          error
	}{
		{
			// Nil error
			stateValidators: []validatorWrapperParams{
				{
					address:  makeAddr(1),
					identity: makeIdentityStr(1),
					blsKeys:  []shard.BLSPublicKey{blsKeys.get(1).pub},
				},
			},
			validator: validatorAddress,
			identity:  makeIdentityStr(0),
			blsKeys:   []shard.BLSPublicKey{blsKeys.get(0).pub},
			expErr:    nil,
		},
		{
			// Nil error, check for self
			stateValidators: []validatorWrapperParams{
				{
					address:  validatorAddress,
					identity: makeIdentityStr(0),
					blsKeys:  []shard.BLSPublicKey{blsKeys.get(0).pub},
				},
			},
			validator: validatorAddress,
			identity:  makeIdentityStr(0),
			blsKeys:   []shard.BLSPublicKey{blsKeys.get(0).pub},
			expErr:    nil,
		},
		{
			// duplicate identity
			stateValidators: []validatorWrapperParams{
				{
					address:  makeAddr(1),
					identity: makeIdentityStr(0),
					blsKeys:  []shard.BLSPublicKey{blsKeys.get(1).pub},
				},
			},
			validator: validatorAddress,
			identity:  makeIdentityStr(0),
			blsKeys:   []shard.BLSPublicKey{blsKeys.get(0).pub},
			expErr:    errors.New("duplicate identity"),
		},
		{
			// duplicate bls keys
			stateValidators: []validatorWrapperParams{
				{
					address:  makeAddr(1),
					identity: makeIdentityStr(1),
					blsKeys:  []shard.BLSPublicKey{blsKeys.get(0).pub},
				},
			},
			validator: validatorAddress,
			identity:  makeIdentityStr(0),
			blsKeys:   []shard.BLSPublicKey{blsKeys.get(0).pub},
			expErr:    errors.New("duplicate bls keys"),
		},
	}
	for i, test := range tests {
		cc, sdb, err := makeTestChainContextAndStateDB(test.stateValidators)
		if err != nil {
			t.Fatal(err)
		}
		err = checkDuplicateFields(cc, sdb, test.validator, test.identity, test.blsKeys)
		if (err == nil) != (test.expErr == nil) {
			t.Errorf("Test %v: got error %v, expect error %v", i, err, test.expErr)
		}
	}
}

// testDataSetup setup the data for testing in init process.
func testDataSetup() {
	validatorAddress = common.Address(common2.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
	validatorBalance = big.NewInt(5e18)

	defaultMaxTotalDelegation = new(big.Int).Mul(big.NewInt(90000), oneAsBigInt)
	defaultMinSelfDelegation = new(big.Int).Mul(big.NewInt(10000), oneAsBigInt)
	defaultSelfDelegation = new(big.Int).Mul(big.NewInt(50000), oneAsBigInt)

	blsKeys = newBLSKeyPool()
}

// makeCreateValidatorMsg makes a createValidator message for testing.
// Returned message could be treated as a copy of a valid message prototype.
func makeCreateValidatorMsg() *staking.CreateValidator {
	desc := staking.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
	rate, _ := numeric.NewDecFromStr("0.1")
	maxRate, _ := numeric.NewDecFromStr("0.5")
	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
	commission := staking.CommissionRates{
		Rate:          rate,
		MaxRate:       maxRate,
		MaxChangeRate: maxChangeRate,
	}
	minSelfDel := big.NewInt(1e18)
	maxTotalDel := big.NewInt(9e18)
	slotPubKeys := []shard.BLSPublicKey{blsKeys.get(0).pub}
	slotKeySigs := []shard.BLSSignature{blsKeys.get(0).sig}
	amount := validatorBalance
	v := staking.CreateValidator{
		ValidatorAddress:   validatorAddress,
		Description:        desc,
		CommissionRates:    commission,
		MinSelfDelegation:  minSelfDel,
		MaxTotalDelegation: maxTotalDel,
		SlotPubKeys:        slotPubKeys,
		SlotKeySigs:        slotKeySigs,
		Amount:             amount,
	}
	return &v
}

// fakeChainContext is the fake structure of ChainContext for testing
type fakeChainContext struct {
	validators []common.Address
}

func makeFakeChainContext(validators []common.Address) *fakeChainContext {
	return &fakeChainContext{
		validators: validators,
	}
}

func (fcc *fakeChainContext) ReadValidatorList() ([]common.Address, error) {
	validators := make([]common.Address, len(fcc.validators))
	copy(validators, fcc.validators)
	return validators, nil
}

func (fcc *fakeChainContext) Engine() consensus_engine.Engine {
	return nil
}

func (fcc *fakeChainContext) GetHeader(common.Hash, uint64) *block.Header {
	return nil
}

func (fcc *fakeChainContext) ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error) {
	return nil, nil
}

func (fcc *fakeChainContext) ReadValidatorSnapshot(common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, nil
}

func makeTestChainContextAndStateDB(params []validatorWrapperParams) (ChainContext, vm.StateDB, error) {
	validators := make([]common.Address, 0, len(params))
	sdb, err := makeTestStateDB()
	if err != nil {
		return nil, nil, err
	}
	for _, param := range params {
		validators = append(validators, param.address)
		wrapper := makeValidatorWrapper(param)
		if err := sdb.UpdateValidatorWrapper(param.address, &wrapper); err != nil {
			return nil, nil, err
		}
	}
	cc := makeFakeChainContext(validators)
	return cc, sdb, nil
}

func makeValidatorWrapper(params validatorWrapperParams) staking.ValidatorWrapper {
	params.applyDefaults()

	var wrapper staking.ValidatorWrapper
	wrapper.Address = params.address
	wrapper.Identity = params.identity
	wrapper.SlotPubKeys = params.blsKeys
	wrapper.Status = params.status
	wrapper.MaxTotalDelegation = params.maxTotalDelegation
	wrapper.MinSelfDelegation = params.minSelfDelegation
	wrapper.Delegations = []staking.Delegation{
		{
			DelegatorAddress: params.address,
			Amount:           params.selfDelegation,
			Reward:           common.Big0,
		},
	}
	wrapper.Counters.NumBlocksToSign = params.numBlocksToSign
	wrapper.Counters.NumBlocksSigned = params.numBlocksSigned
	wrapper.Rate = params.rate
	wrapper.MaxRate = params.maxRate
	wrapper.MaxChangeRate = params.maxChangeRate
	return wrapper
}

type validatorWrapperParams struct {
	// Required fields
	address  common.Address
	identity string
	blsKeys  []shard.BLSPublicKey

	// Optional fields. If not set, default to default values
	status             effective.Eligibility
	maxTotalDelegation *big.Int
	minSelfDelegation  *big.Int
	selfDelegation     *big.Int
	numBlocksToSign    *big.Int
	numBlocksSigned    *big.Int
	rate               numeric.Dec
	maxRate            numeric.Dec
	maxChangeRate      numeric.Dec
}

func (params *validatorWrapperParams) applyDefaults() {
	if params.status == effective.Nil {
		params.status = effective.Active
	}
	if params.maxTotalDelegation == nil {
		params.maxTotalDelegation = defaultMaxTotalDelegation
	}
	if params.minSelfDelegation == nil {
		params.minSelfDelegation = defaultMinSelfDelegation
	}
	if params.selfDelegation == nil {
		params.selfDelegation = defaultSelfDelegation
	}
	if params.numBlocksSigned == nil {
		params.numBlocksSigned = common.Big0
	}
	if params.numBlocksToSign == nil {
		params.numBlocksToSign = common.Big0
	}
	if params.rate.IsNil() {
		params.rate = numeric.ZeroDec()
	}
	if params.maxRate.IsNil() {
		params.maxRate = numeric.ZeroDec()
	}
	if params.maxChangeRate.IsNil() {
		params.maxChangeRate = numeric.ZeroDec()
	}
}

func makeIdentityStr(index uint64) string {
	return fmt.Sprintf("harmony-one-%v", index)
}

func makeAddr(index uint64) common.Address {
	var addr common.Address
	binary.LittleEndian.PutUint64(addr[:], uint64(index))
	return addr
}

// makeTestStateDB creates a new state db for testing
func makeTestStateDB() (*state.DB, error) {
	return state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
}

type blsKeyPool struct {
	keys []blsKeyPair
}

func newBLSKeyPool() blsKeyPool {
	keys := make([]blsKeyPair, 0, 16)
	return blsKeyPool{keys}
}

func (kp *blsKeyPool) get(index int) blsKeyPair {
	if index < len(kp.keys) {
		return kp.keys[index]
	}
	for i := len(kp.keys); i <= index; i++ {
		kp.keys = append(kp.keys, makeBLSKeyPair())
	}
	return kp.keys[index]
}

type blsKeyPair struct {
	pub shard.BLSPublicKey
	sig shard.BLSSignature
}

func makeBLSKeyPair() blsKeyPair {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub shard.BLSPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig shard.BLSSignature
	copy(shardSig[:], sig.Serialize())

	return blsKeyPair{shardPub, shardSig}
}
