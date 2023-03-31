package core

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/core/rawdb"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
	staketest "github.com/harmony-one/harmony/staking/types/test"
)

const (
	defNumWrappersInState = 5
	defNumPubPerAddr      = 2

	validatorIndex  = 0
	validator2Index = 1
	delegatorIndex  = 6
)

var (
	blsKeys = makeKeyPairs(20)

	createValidatorAddr = makeTestAddr("validator")
	validatorAddr       = makeTestAddr(validatorIndex)
	validatorAddr2      = makeTestAddr(validator2Index)
	delegatorAddr       = makeTestAddr(delegatorIndex)
)

var (
	oneBig          = big.NewInt(1e18)
	fiveKOnes       = new(big.Int).Mul(big.NewInt(5000), oneBig)
	tenKOnes        = new(big.Int).Mul(big.NewInt(10000), oneBig)
	twelveKOnes     = new(big.Int).Mul(big.NewInt(12000), oneBig)
	fifteenKOnes    = new(big.Int).Mul(big.NewInt(15000), oneBig)
	twentyKOnes     = new(big.Int).Mul(big.NewInt(20000), oneBig)
	twentyFiveKOnes = new(big.Int).Mul(big.NewInt(25000), oneBig)
	thirtyKOnes     = new(big.Int).Mul(big.NewInt(30000), oneBig)
	hundredKOnes    = new(big.Int).Mul(big.NewInt(100000), oneBig)

	negRate           = numeric.NewDecWithPrec(-1, 10)
	pointZeroOneDec   = numeric.NewDecWithPrec(1, 2)
	pointOneDec       = numeric.NewDecWithPrec(1, 1)
	pointTwoDec       = numeric.NewDecWithPrec(2, 1)
	pointFourDec      = numeric.NewDecWithPrec(4, 1)
	pointFiveDec      = numeric.NewDecWithPrec(5, 1)
	pointSevenDec     = numeric.NewDecWithPrec(7, 1)
	pointEightFiveDec = numeric.NewDecWithPrec(85, 2)
	pointNineDec      = numeric.NewDecWithPrec(9, 1)
	oneDec            = numeric.OneDec()
)

const (
	defaultEpoch           = 5
	defaultNextEpoch       = 6
	defaultSnapBlockNumber = 90
	defaultBlockNumber     = 100
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
		Rate:          pointOneDec,
		MaxRate:       pointNineDec,
		MaxChangeRate: pointFiveDec,
	}
)

func TestCheckDuplicateFields(t *testing.T) {
	tests := []struct {
		bc        ChainContext
		sdb       *state.DB
		validator common.Address
		identity  string
		pubs      []bls.SerializedPublicKey

		expErr error
	}{
		{
			// new validator
			bc:        makeFakeChainContextForStake(),
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{blsKeys[11].pub},

			expErr: nil,
		},
		{
			// validator skip self check
			bc:        makeFakeChainContextForStake(),
			sdb:       makeStateDBForStake(t),
			validator: makeTestAddr(0),
			identity:  makeIdentityStr(0),
			pubs:      []bls.SerializedPublicKey{blsKeys[0].pub, blsKeys[1].pub},

			expErr: nil,
		},
		{
			// empty bls keys
			bc:        makeFakeChainContextForStake(),
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{},

			expErr: nil,
		},
		{
			// empty identity will not collide
			bc: makeFakeChainContextForStake(),
			sdb: func(t *testing.T) *state.DB {
				sdb := makeStateDBForStake(t)
				vw, err := sdb.ValidatorWrapper(makeTestAddr(0), false, true)
				if err != nil {
					t.Fatal(err)
				}
				vw.Identity = ""

				err = sdb.UpdateValidatorWrapper(makeTestAddr(0), vw)
				if err != nil {
					t.Fatal(err)
				}
				return sdb
			}(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{blsKeys[11].pub},

			expErr: nil,
		},
		{
			// chain error
			bc:        &fakeErrChainContext{},
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{blsKeys[11].pub},

			expErr: errors.New("error intended"),
		},
		{
			// validators read from chain not in state
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				addr := makeTestAddr("not exist in state")
				w := staketest.GetDefaultValidatorWrapper()
				chain.vWrappers[addr] = &w
				return chain
			}(),
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{blsKeys[11].pub},

			expErr: errors.New("address not present in state"),
		},
		{
			// duplicate identity
			bc:        makeFakeChainContextForStake(),
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr(0),
			pubs:      []bls.SerializedPublicKey{blsKeys[11].pub},

			expErr: errDupIdentity,
		},
		{
			// bls key duplication
			bc:        makeFakeChainContextForStake(),
			sdb:       makeStateDBForStake(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []bls.SerializedPublicKey{blsKeys[0].pub},

			expErr: errDupBlsKey,
		},
	}
	for i, test := range tests {
		err := checkDuplicateFields(test.bc, test.sdb, test.validator, test.identity, test.pubs)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
	}
}

func TestVerifyAndCreateValidatorFromMsg(t *testing.T) {
	tests := []struct {
		sdb      vm.StateDB
		chain    ChainContext
		epoch    *big.Int
		blockNum *big.Int
		msg      staking.CreateValidator

		expWrapper staking.ValidatorWrapper
		expErr     error
	}{
		{
			// 0: valid request
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expWrapper: defaultExpWrapperCreateValidator(),
		},
		{
			// 1: nil state db
			sdb:      nil,
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errStateDBIsMissing,
		},
		{
			// 2: nil chain context
			sdb:      makeStateDBForStake(t),
			chain:    nil,
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errChainContextMissing,
		},
		{
			// 3: nil epoch
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    nil,
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errEpochMissing,
		},
		{
			// 4: nil block number
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: nil,
			msg:      defaultMsgCreateValidator(),

			expErr: errBlockNumMissing,
		},
		{
			// 5: negative amount
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.Amount = big.NewInt(-1)
				return m
			}(),
			expErr: errNegativeAmount,
		},
		{
			// 6: the address isValidatorFlag is true
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				sdb.SetValidatorFlag(createValidatorAddr)
				return sdb
			}(),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errValidatorExist,
		},
		{
			// 7: bls collision (checkDuplicateFields)
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.SlotPubKeys = []bls.SerializedPublicKey{blsKeys[0].pub}
				return m
			}(),

			expErr: errors.New("BLS key exists"),
		},
		{
			// 8: insufficient balance
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				bal := new(big.Int).Sub(staketest.DefaultDelAmount, common.Big1)
				sdb.SetBalance(createValidatorAddr, bal)
				return sdb
			}(),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errInsufficientBalanceForStake,
		},
		{
			// 9: incorrect signature
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.SlotKeySigs = []bls.SerializedSignature{blsKeys[12].sig}
				return m
			}(),

			expErr: errors.New("bls keys and corresponding signatures"),
		},
		{
			// 10: small self delegation amount (fail sanity check)
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.Amount = new(big.Int).Sub(m.MinSelfDelegation, common.Big1)
				return m
			}(),

			expErr: errors.New("self delegation can not be less than min_self_delegation"),
		},
		{
			// 11: amount exactly minSelfDelegation. Should not return error
			sdb:      makeStateDBForStake(t),
			chain:    makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.Amount = new(big.Int).Set(m.MinSelfDelegation)
				return m
			}(),

			expWrapper: func() staking.ValidatorWrapper {
				w := defaultExpWrapperCreateValidator()
				w.Delegations[0].Amount = new(big.Int).Set(w.MinSelfDelegation)
				return w
			}(),
		},
	}
	for i, test := range tests {
		w, err := VerifyAndCreateValidatorFromMsg(test.sdb, test.chain, test.epoch,
			test.blockNum, &test.msg)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, err)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if err := staketest.CheckValidatorWrapperEqual(*w, test.expWrapper); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func defaultMsgCreateValidator() staking.CreateValidator {
	pub, sig := blsKeys[11].pub, blsKeys[11].sig
	cv := staking.CreateValidator{
		ValidatorAddress:   createValidatorAddr,
		Description:        defaultDesc,
		CommissionRates:    defaultCommissionRates,
		MinSelfDelegation:  staketest.DefaultMinSelfDel,
		MaxTotalDelegation: staketest.DefaultMaxTotalDel,
		SlotPubKeys:        []bls.SerializedPublicKey{pub},
		SlotKeySigs:        []bls.SerializedSignature{sig},
		Amount:             staketest.DefaultDelAmount,
	}
	return cv
}

func defaultExpWrapperCreateValidator() staking.ValidatorWrapper {
	pub := blsKeys[11].pub
	v := staking.Validator{
		Address:              createValidatorAddr,
		SlotPubKeys:          []bls.SerializedPublicKey{pub},
		LastEpochInCommittee: new(big.Int),
		MinSelfDelegation:    staketest.DefaultMinSelfDel,
		MaxTotalDelegation:   staketest.DefaultMaxTotalDel,
		Status:               effective.Active,
		Commission: staking.Commission{
			CommissionRates: defaultCommissionRates,
			UpdateHeight:    big.NewInt(defaultBlockNumber),
		},
		Description:    defaultDesc,
		CreationHeight: big.NewInt(defaultBlockNumber),
	}
	ds := staking.Delegations{
		staking.NewDelegation(createValidatorAddr, staketest.DefaultDelAmount),
	}
	w := staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
		BlockReward: big.NewInt(0),
	}
	w.Counters.NumBlocksSigned = common.Big0
	w.Counters.NumBlocksToSign = common.Big0
	return w
}

func TestVerifyAndEditValidatorFromMsg(t *testing.T) {
	tests := []struct {
		sdb             vm.StateDB
		bc              ChainContext
		epoch, blockNum *big.Int
		msg             staking.EditValidator
		expWrapper      staking.ValidatorWrapper
		expErr          error
	}{
		{
			// 0: positive case
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expWrapper: defaultExpWrapperEditValidator(),
		},
		{
			// 1: If the rate is not changed, UpdateHeight is not update
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = nil
				return msg
			}(),

			expWrapper: func() staking.ValidatorWrapper {
				vw := defaultExpWrapperEditValidator()
				vw.UpdateHeight = big.NewInt(defaultSnapBlockNumber)
				vw.Rate = pointFourDec
				return vw
			}(),
		},
		{
			// 2: nil state db
			sdb:      nil,
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errStateDBIsMissing,
		},
		{
			// 3: nil chain
			sdb:      makeStateDBForStake(t),
			bc:       nil,
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errChainContextMissing,
		},
		{
			// 4: nil block number
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: nil,
			msg:      defaultMsgEditValidator(),

			expErr: errBlockNumMissing,
		},
		{
			// 5: edited validator flag not set in state
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.ValidatorAddress = makeTestAddr("addr not in chain")
				return msg
			}(),

			expErr: errValidatorNotExist,
		},
		{
			// 6: bls key collision
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.SlotKeyToAdd = &blsKeys[3].pub
				msg.SlotKeyToAddSig = &blsKeys[3].sig
				return msg
			}(),

			expErr: errDupBlsKey,
		},
		{
			// 7: validatorWrapper not in state
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				sdb.SetValidatorFlag(makeTestAddr("someone"))
				return sdb
			}(),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				addr := makeTestAddr("someone")
				w := staketest.GetDefaultValidatorWrapperWithAddr(addr, nil)
				chain.vWrappers[addr] = &w
				return chain
			}(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.ValidatorAddress = makeTestAddr("someone")
				return msg
			}(),

			expErr: errors.New("address not present in state"),
		},
		{
			// 8: signature cannot be verified
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.SlotKeyToAddSig = &blsKeys[13].sig
				return msg
			}(),

			expErr: errors.New("bls keys and corresponding signatures could not be verified"),
		},
		{
			// 9: Rate is greater the maxRate
			sdb: makeStateDBForStake(t),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				vw := chain.vWrappers[validatorAddr]
				vw.Rate = pointSevenDec
				chain.vWrappers[validatorAddr] = vw
				return chain
			}(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = &oneDec
				return msg
			}(),

			expErr: errCommissionRateChangeTooHigh,
		},
		{
			// 10: validator not in snapshot
			sdb: makeStateDBForStake(t),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				delete(chain.vWrappers, validatorAddr)
				return chain
			}(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errors.New("validator snapshot not found"),
		},
		{
			// 11: rate is greater than maxChangeRate
			sdb:      makeStateDBForStake(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = &pointEightFiveDec
				return msg
			}(),

			expErr: errCommissionRateChangeTooFast,
		},
		{
			// 12: fails in sanity check (rate below zero)
			sdb: makeStateDBForStake(t),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				vw := chain.vWrappers[validatorAddr]
				vw.Rate = pointOneDec
				chain.vWrappers[validatorAddr] = vw
				return chain
			}(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = &negRate
				return msg
			}(),

			expErr: errors.New("rate should be a value ranging from 0.0 to 1.0"),
		},
		{
			// 13: cannot update a banned validator
			sdb: func(t *testing.T) *state.DB {
				sdb := makeStateDBForStake(t)
				vw, err := sdb.ValidatorWrapper(validatorAddr, false, true)
				if err != nil {
					t.Fatal(err)
				}
				vw.Status = effective.Banned
				if err := sdb.UpdateValidatorWrapper(validatorAddr, vw); err != nil {
					t.Fatal(err)
				}
				return sdb
			}(t),
			bc:       makeFakeChainContextForStake(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errors.New("banned status"),
		},
		{
			// 14: Rate is lower than min rate of 5%
			sdb: makeStateDBForStake(t),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				vw := chain.vWrappers[validatorAddr]
				vw.Rate = pointFourDec
				chain.vWrappers[validatorAddr] = vw
				return chain
			}(),
			epoch:    big.NewInt(20),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = &pointZeroOneDec
				return msg
			}(),

			expErr: errCommissionRateChangeTooLow,
		},
		{
			// 15: Rate is ok within the promo period
			sdb: makeStateDBForStake(t),
			bc: func() *fakeChainContext {
				chain := makeFakeChainContextForStake()
				vw := chain.vWrappers[validatorAddr]
				vw.Rate = pointFourDec
				chain.vWrappers[validatorAddr] = vw
				return chain
			}(),
			epoch:    big.NewInt(15),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.EditValidator {
				msg := defaultMsgEditValidator()
				msg.CommissionRate = &pointZeroOneDec
				return msg
			}(),

			expWrapper: func() staking.ValidatorWrapper {
				vw := defaultExpWrapperEditValidator()
				vw.UpdateHeight = big.NewInt(defaultBlockNumber)
				vw.Rate = pointZeroOneDec
				return vw
			}(),
		},
	}
	for i, test := range tests {
		w, err := VerifyAndEditValidatorFromMsg(test.sdb, test.bc, test.epoch, test.blockNum,
			&test.msg)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: unexpected Error: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if err := staketest.CheckValidatorWrapperEqual(*w, test.expWrapper); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

var (
	editDesc = staking.Description{
		Name:            "batman",
		Identity:        "batman",
		Website:         "",
		SecurityContact: "",
		Details:         "",
	}

	editExpDesc = staking.Description{
		Name:            "batman",
		Identity:        "batman",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
)

func defaultMsgEditValidator() staking.EditValidator {
	var (
		pub0Copy  bls.SerializedPublicKey
		pub12Copy bls.SerializedPublicKey
		sig12Copy bls.SerializedSignature
	)
	copy(pub0Copy[:], blsKeys[0].pub[:])
	copy(pub12Copy[:], blsKeys[12].pub[:])
	copy(sig12Copy[:], blsKeys[12].sig[:])

	return staking.EditValidator{
		ValidatorAddress: validatorAddr,
		Description:      editDesc,
		CommissionRate:   &pointTwoDec,
		SlotKeyToRemove:  &pub0Copy,
		SlotKeyToAdd:     &pub12Copy,
		SlotKeyToAddSig:  &sig12Copy,
		EPOSStatus:       effective.Inactive,
	}
}

func defaultExpWrapperEditValidator() staking.ValidatorWrapper {
	w := makeVWrapperByIndex(validatorIndex)
	w.SlotPubKeys = append(w.SlotPubKeys[1:], blsKeys[12].pub)
	w.Description = editExpDesc
	w.Rate = pointTwoDec
	w.UpdateHeight = big.NewInt(defaultBlockNumber)
	w.Status = effective.Inactive
	return w
}

func TestVerifyAndDelegateFromMsg(t *testing.T) {
	tests := []struct {
		sdb        vm.StateDB
		msg        staking.Delegate
		ds         []staking.DelegationIndex
		epoch      *big.Int
		redelegate bool

		expVWrappers []staking.ValidatorWrapper
		expAmt       *big.Int
		expRedel     map[common.Address]*big.Int
		expErr       error
	}{
		{
			// 0: new delegate
			sdb:        makeStateDBForStake(t),
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expVWrappers: []staking.ValidatorWrapper{defaultExpVWrapperDelegate()},
			expAmt:       tenKOnes,
		},
		{
			// 1: add amount to current delegate
			sdb:        makeStateDBForStake(t),
			msg:        defaultMsgSelfDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expVWrappers: []staking.ValidatorWrapper{defaultExpVWrapperSelfDelegate()},
			expAmt:       tenKOnes,
		},
		{
			// 2: nil state db
			sdb:        nil,
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errStateDBIsMissing,
		},
		{
			// 3: validatorFlag not set
			sdb: makeStateDBForStake(t),
			ds:  makeMsgCollectRewards(),
			msg: func() staking.Delegate {
				msg := defaultMsgDelegate()
				msg.ValidatorAddress = makeTestAddr("not in state")
				return msg
			}(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errValidatorNotExist,
		},
		{
			// 4: negative amount
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				msg := defaultMsgDelegate()
				msg.Amount = big.NewInt(-1)
				return msg
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errNegativeAmount,
		},
		{
			// 5: small amount
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				msg := defaultMsgDelegate()
				msg.Amount = big.NewInt(100)
				return msg
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errDelegationTooSmall,
		},
		{
			// 6: missing validator wrapper
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				sdb.SetValidatorFlag(createValidatorAddr)
				return sdb
			}(),
			msg: func() staking.Delegate {
				d := defaultMsgDelegate()
				d.ValidatorAddress = createValidatorAddr
				return d
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errors.New("address not present in state"),
		},
		{
			// 7: cannot transfer since not enough amount
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				sdb.SetBalance(delegatorAddr, big.NewInt(100))
				return sdb
			}(),
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errInsufficientBalanceForStake,
		},
		{
			// 8: self delegation not pass sanity check
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				d := defaultMsgSelfDelegate()
				d.Amount = hundredKOnes
				return d
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errors.New(" total delegation can not be bigger than max_total_delegation"),
		},
		{
			// 9: Delegation does not pass sanity check
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				d := defaultMsgDelegate()
				d.Amount = hundredKOnes
				return d
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(0),
			redelegate: false,

			expErr: errors.New(" total delegation can not be bigger than max_total_delegation"),
		},
		{
			// 10: full redelegate without using balance
			sdb:        makeStateForRedelegate(t),
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(7),
			redelegate: true,

			expVWrappers: defaultExpVWrappersRedelegate(),
			expAmt:       big.NewInt(0),
			expRedel: map[common.Address]*big.Int{
				validatorAddr:  fiveKOnes,
				validatorAddr2: fiveKOnes,
			},
		},
		{
			// 11: redelegate with undelegation epoch too recent, have to use some balance
			sdb:        makeStateForRedelegate(t),
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(6),
			redelegate: true,

			expVWrappers: func() []staking.ValidatorWrapper {
				wrappers := defaultExpVWrappersRedelegate()
				wrappers[1].Delegations[1].Undelegations = append(
					wrappers[1].Delegations[1].Undelegations,
					staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultNextEpoch)})
				return wrappers
			}(),
			expAmt: fiveKOnes,
			expRedel: map[common.Address]*big.Int{
				validatorAddr: fiveKOnes,
			},
		},
		{
			// 12: redelegate with not enough undelegated token, have to use some balance
			sdb: makeStateForRedelegate(t),
			msg: func() staking.Delegate {
				del := defaultMsgDelegate()
				del.Amount = twentyKOnes
				return del
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(7),
			redelegate: true,

			expVWrappers: func() []staking.ValidatorWrapper {
				wrappers := defaultExpVWrappersRedelegate()
				wrappers[0].Delegations[1].Amount = big.NewInt(0).Add(twentyKOnes, twentyKOnes)
				return wrappers
			}(),
			expAmt: tenKOnes,
			expRedel: map[common.Address]*big.Int{
				validatorAddr:  fiveKOnes,
				validatorAddr2: fiveKOnes,
			},
		},
		{
			// 13: no redelegation and full balance used
			sdb:        makeStateForRedelegate(t),
			msg:        defaultMsgDelegate(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(5),
			redelegate: true,

			expVWrappers: func() []staking.ValidatorWrapper {
				wrappers := defaultExpVWrappersRedelegate()
				wrappers[0].Delegations[1].Undelegations = append(
					wrappers[0].Delegations[1].Undelegations,
					staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultEpoch)})
				wrappers[1].Delegations[1].Undelegations = append(
					wrappers[1].Delegations[1].Undelegations,
					staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultNextEpoch)})
				return wrappers
			}(),
			expAmt: tenKOnes,
		},
		{
			// 14: redelegate error delegation index out of bound
			sdb: makeStateForRedelegate(t),
			msg: defaultMsgSelfDelegate(),
			ds: func() []staking.DelegationIndex {
				dis := makeMsgCollectRewards()
				dis[1].Index = 2
				return dis
			}(),
			epoch:      big.NewInt(0),
			redelegate: true,

			expErr: errors.New("index out of bound"),
		},
		{
			// 15: redelegate error delegation index out of bound
			sdb: makeStateForRedelegate(t),
			msg: defaultMsgSelfDelegate(),
			ds: func() []staking.DelegationIndex {
				dis := makeMsgCollectRewards()
				dis[1].Index = 2
				return dis
			}(),
			epoch:      big.NewInt(0),
			redelegate: true,

			expErr: errors.New("index out of bound"),
		},
		{
			// 16: no redelegation and full balance used (validator delegate to self)
			sdb: makeStateForRedelegate(t),
			msg: func() staking.Delegate {
				del := defaultMsgDelegate()
				del.DelegatorAddress = del.ValidatorAddress
				return del
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(5),
			redelegate: true,

			expVWrappers: func() []staking.ValidatorWrapper {
				wrappers := defaultExpVWrappersRedelegate()
				wrappers[0].Delegations[1].Undelegations = append(
					wrappers[0].Delegations[1].Undelegations,
					staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultEpoch)})
				wrappers[1].Delegations[1].Undelegations = append(
					wrappers[1].Delegations[1].Undelegations,
					staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultNextEpoch)})

				wrappers[0].Delegations[1].Amount = twentyKOnes
				wrappers[0].Delegations[0].Amount = big.NewInt(0).Add(twentyKOnes, tenKOnes)
				return wrappers
			}(),
			expAmt: tenKOnes,
		},
		{
			// 17: small amount v2
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				msg := defaultMsgDelegate()
				msg.Amount = new(big.Int).Mul(big.NewInt(90), oneBig)
				return msg
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(100),
			redelegate: false,

			expErr: errDelegationTooSmallV2,
		},
		{
			// 18: valid amount v2
			sdb: makeStateDBForStake(t),
			msg: func() staking.Delegate {
				msg := defaultMsgDelegate()
				msg.Amount = new(big.Int).Mul(big.NewInt(500), oneBig)
				return msg
			}(),
			ds:         makeMsgCollectRewards(),
			epoch:      big.NewInt(100),
			redelegate: false,

			expVWrappers: func() []staking.ValidatorWrapper {
				wrapper := defaultExpVWrapperDelegate()
				wrapper.Delegations[1].Amount = new(big.Int).Mul(big.NewInt(500), oneBig)
				return []staking.ValidatorWrapper{wrapper}
			}(),
			expAmt: new(big.Int).Mul(big.NewInt(500), oneBig),
		},
	}
	for i, test := range tests {
		config := &params.ChainConfig{}
		config.MinDelegation100Epoch = big.NewInt(100)
		if test.redelegate {
			config.RedelegationEpoch = test.epoch
		} else {
			config.RedelegationEpoch = big.NewInt(test.epoch.Int64() + 1)
		}
		ws, amt, amtRedel, err := VerifyAndDelegateFromMsg(test.sdb, test.epoch, &test.msg, test.ds, config)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if amt.Cmp(test.expAmt) != 0 {
			t.Errorf("Test %v: unexpected amount %v / %v", i, amt, test.expAmt)
		}

		if len(amtRedel) != len(test.expRedel) {
			t.Errorf("Test %v: wrong expected redelegation length %d / %d", i, len(amtRedel), len(test.expRedel))
		} else {
			for key, value := range test.expRedel {
				actValue, ok := amtRedel[key]
				if !ok {
					t.Errorf("Test %v: missing expected redelegation key/value %v / %v", i, key, value)
				}
				if value.Cmp(actValue) != 0 {
					t.Errorf("Test %v: unexpeced redelegation value %v / %v", i, actValue, value)
				}
			}
		}
		for j := range ws {
			if err := staketest.CheckValidatorWrapperEqual(*ws[j], test.expVWrappers[j]); err != nil {
				t.Errorf("Test %v: %v", i, err)
			}
		}
	}
}

func makeStateForRedelegate(t *testing.T) *state.DB {
	sdb := makeStateDBForStake(t)

	if err := addStateUndelegationForAddr(sdb, validatorAddr, big.NewInt(defaultEpoch)); err != nil {
		t.Fatal(err)
	}
	if err := addStateUndelegationForAddr(sdb, validatorAddr2, big.NewInt(defaultNextEpoch)); err != nil {
		t.Fatal(err)
	}

	sdb.IntermediateRoot(true)
	return sdb
}

func addStateUndelegationForAddr(sdb *state.DB, addr common.Address, epoch *big.Int) error {
	w, err := sdb.ValidatorWrapper(addr, false, true)
	if err != nil {
		return err
	}
	w.Delegations = append(w.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes)),
	)
	w.Delegations[1].Undelegations = staking.Undelegations{staking.Undelegation{Amount: fiveKOnes, Epoch: epoch}}

	return sdb.UpdateValidatorWrapper(addr, w)
}

func defaultExpVWrappersRedelegate() []staking.ValidatorWrapper {
	w1 := makeVWrapperByIndex(validatorIndex)
	w1.Delegations = append(w1.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(thirtyKOnes)),
	)

	w2 := makeVWrapperByIndex(validator2Index)
	w2.Delegations = append(w2.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes)),
	)
	return []staking.ValidatorWrapper{w1, w2}
}

func defaultMsgDelegate() staking.Delegate {
	return staking.Delegate{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           new(big.Int).Set(tenKOnes),
	}
}

func defaultExpVWrapperDelegate() staking.ValidatorWrapper {
	w := makeVWrapperByIndex(validatorIndex)
	w.Delegations = append(w.Delegations, staking.NewDelegation(delegatorAddr, tenKOnes))
	return w
}

func defaultMsgSelfDelegate() staking.Delegate {
	return staking.Delegate{
		DelegatorAddress: validatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           new(big.Int).Set(tenKOnes),
	}
}

func defaultExpVWrapperSelfDelegate() staking.ValidatorWrapper {
	w := makeVWrapperByIndex(validatorIndex)
	w.Delegations[0].Amount = new(big.Int).Add(tenKOnes, staketest.DefaultDelAmount)
	return w
}

func TestVerifyAndUndelegateFromMsg(t *testing.T) {
	tests := []struct {
		sdb   vm.StateDB
		epoch *big.Int
		msg   staking.Undelegate

		expVWrapper staking.ValidatorWrapper
		expErr      error
	}{
		{
			// 0: Undelegate at delegation with an entry already exist at the same epoch.
			// Will increase the amount in undelegate entry
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg:   defaultMsgUndelegate(),

			expVWrapper: defaultExpVWrapperUndelegateSameEpoch(t),
		},
		{
			// 1: Undelegate with undelegation entry exist but not in same epoch.
			// Will create a new undelegate entry
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultNextEpoch),
			msg:   defaultMsgUndelegate(),

			expVWrapper: defaultExpVWrapperUndelegateNextEpoch(t),
		},
		{
			// 2: Undelegate from a delegation record with no undelegation entry.
			// Will create a new undelegate entry
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg:   defaultMsgSelfUndelegate(),

			expVWrapper: defaultVWrapperSelfUndelegate(t),
		},
		{
			// 3: Self delegation below min self delegation, change status to Inactive
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgSelfUndelegate()
				msg.Amount = new(big.Int).Set(fifteenKOnes)
				return msg
			}(),

			expVWrapper: func(t *testing.T) staking.ValidatorWrapper {
				w := defaultVWrapperSelfUndelegate(t)

				w.Delegations[0].Amount = new(big.Int).Set(fiveKOnes)
				w.Delegations[0].Undelegations[0].Amount = new(big.Int).Set(fifteenKOnes)
				w.Status = effective.Inactive

				return w
			}(t),
		},
		{
			// 4: Extract tokens from banned validator
			sdb: func(t *testing.T) *state.DB {
				sdb := makeDefaultStateForUndelegate(t)
				w, err := sdb.ValidatorWrapper(validatorAddr, false, true)
				if err != nil {
					t.Fatal(err)
				}
				w.Status = effective.Banned
				if err := sdb.UpdateValidatorWrapper(validatorAddr, w); err != nil {
					t.Fatal(err)
				}
				return sdb
			}(t),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgSelfUndelegate()
				msg.Amount = new(big.Int).Set(fifteenKOnes)
				return msg
			}(),

			expVWrapper: func(t *testing.T) staking.ValidatorWrapper {
				w := defaultVWrapperSelfUndelegate(t)

				w.Delegations[0].Amount = new(big.Int).Set(fiveKOnes)
				w.Delegations[0].Undelegations[0].Amount = new(big.Int).Set(fifteenKOnes)
				w.Status = effective.Banned

				return w
			}(t),
		},
		{
			// 5: nil state db
			sdb:   nil,
			epoch: big.NewInt(defaultEpoch),
			msg:   defaultMsgUndelegate(),

			expErr: errStateDBIsMissing,
		},
		{
			// 6: nil epoch
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: nil,
			msg:   defaultMsgUndelegate(),

			expErr: errEpochMissing,
		},
		{
			// 7: negative amount
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgUndelegate()
				msg.Amount = big.NewInt(-1)
				return msg
			}(),

			expErr: errNegativeAmount,
		},
		{
			// 8: validator flag not set
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				w := makeVWrapperByIndex(6)
				if err := sdb.UpdateValidatorWrapper(makeTestAddr(6), &w); err != nil {
					t.Fatal(err)
				}
				return sdb
			}(),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgUndelegate()
				msg.ValidatorAddress = makeTestAddr(6)
				return msg
			}(),

			expErr: errValidatorNotExist,
		},
		{
			// 9: vWrapper not in state
			sdb: func() *state.DB {
				sdb := makeStateDBForStake(t)
				sdb.SetValidatorFlag(makeTestAddr(6))
				return sdb
			}(),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgUndelegate()
				msg.ValidatorAddress = makeTestAddr(6)
				return msg
			}(),

			expErr: errors.New("address not present in state"),
		},
		{
			// 10: Insufficient balance to undelegate
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgUndelegate()
				msg.Amount = new(big.Int).Set(hundredKOnes)
				return msg
			}(),

			expErr: errors.New("insufficient balance to undelegate"),
		},
		{
			// 11: No delegation record
			sdb:   makeDefaultStateForUndelegate(t),
			epoch: big.NewInt(defaultEpoch),
			msg: func() staking.Undelegate {
				msg := defaultMsgUndelegate()
				msg.DelegatorAddress = makeTestAddr("not exist")
				return msg
			}(),

			expErr: errNoDelegationToUndelegate,
		},
	}
	for i, test := range tests {
		w, err := VerifyAndUndelegateFromMsg(test.sdb, test.epoch, &test.msg)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if err := staketest.CheckValidatorWrapperEqual(*w, test.expVWrapper); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func makeDefaultSnapVWrapperForUndelegate(t *testing.T) staking.ValidatorWrapper {
	w := makeVWrapperByIndex(validatorIndex)

	newDelegation := staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes))
	if err := newDelegation.Undelegate(big.NewInt(defaultEpoch), fiveKOnes); err != nil {
		t.Fatal(err)
	}
	w.Delegations = append(w.Delegations, newDelegation)

	return w
}

func makeDefaultStateForUndelegate(t *testing.T) *state.DB {
	sdb := makeStateDBForStake(t)
	w := makeDefaultSnapVWrapperForUndelegate(t)

	if err := sdb.UpdateValidatorWrapper(validatorAddr, &w); err != nil {
		t.Fatal(err)
	}
	sdb.IntermediateRoot(true)
	return sdb
}

// undelegate from delegator which has already go one entry for undelegation
func defaultMsgUndelegate() staking.Undelegate {
	return staking.Undelegate{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           fiveKOnes,
	}
}

func defaultExpVWrapperUndelegateSameEpoch(t *testing.T) staking.ValidatorWrapper {
	w := makeDefaultSnapVWrapperForUndelegate(t)

	amt := w.Delegations[1].Undelegations[0].Amount
	w.Delegations[1].Undelegations[0].Amount = new(big.Int).
		Add(w.Delegations[1].Undelegations[0].Amount, amt)
	w.Delegations[1].Amount = new(big.Int).Sub(w.Delegations[1].Amount, fiveKOnes)

	return w
}

func defaultExpVWrapperUndelegateNextEpoch(t *testing.T) staking.ValidatorWrapper {
	w := makeDefaultSnapVWrapperForUndelegate(t)

	w.Delegations[1].Undelegations = append(w.Delegations[1].Undelegations,
		staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultNextEpoch)})
	w.Delegations[1].Amount = new(big.Int).Sub(w.Delegations[1].Amount, fiveKOnes)

	return w
}

// undelegate from self undelegation (new undelegates)
func defaultMsgSelfUndelegate() staking.Undelegate {
	return staking.Undelegate{
		DelegatorAddress: validatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           fiveKOnes,
	}
}

func defaultVWrapperSelfUndelegate(t *testing.T) staking.ValidatorWrapper {
	w := makeDefaultSnapVWrapperForUndelegate(t)

	w.Delegations[0].Undelegations = staking.Undelegations{
		staking.Undelegation{Amount: fiveKOnes, Epoch: big.NewInt(defaultEpoch)},
	}
	w.Delegations[0].Amount = new(big.Int).Sub(w.Delegations[0].Amount, fiveKOnes)

	return w
}

var (
	reward00 = twentyKOnes
	reward01 = tenKOnes
	reward10 = thirtyKOnes
	reward11 = twentyFiveKOnes
)

func TestVerifyAndCollectRewardsFromDelegation(t *testing.T) {
	tests := []struct {
		sdb vm.StateDB
		ds  []staking.DelegationIndex

		expVWrappers    []*staking.ValidatorWrapper
		expTotalRewards *big.Int
		expErr          error
	}{
		{
			// 0: Positive test case
			sdb: makeStateForReward(t),
			ds:  makeMsgCollectRewards(),

			expVWrappers:    expVWrappersForReward(),
			expTotalRewards: new(big.Int).Add(reward01, reward11),
		},
		{
			// 1: No rewards to collect
			sdb: makeStateDBForStake(t),
			ds:  []staking.DelegationIndex{{ValidatorAddress: validatorAddr2, Index: 0}},

			expErr: errNoRewardsToCollect,
		},
		{
			// 2: nil state db
			sdb: nil,
			ds:  makeMsgCollectRewards(),

			expErr: errStateDBIsMissing,
		},
		{
			// 3: ValidatorWrapper not in state
			sdb: makeStateForReward(t),
			ds: func() []staking.DelegationIndex {
				msg := makeMsgCollectRewards()
				msg[1].ValidatorAddress = makeTestAddr("addr not exist")
				return msg
			}(),

			expErr: errors.New("address not present in state"),
		},
		{
			// 4: Wrong input message - index out of range
			sdb: makeStateForReward(t),
			ds: func() []staking.DelegationIndex {
				dis := makeMsgCollectRewards()
				dis[1].Index = 2
				return dis
			}(),

			expErr: errors.New("index out of bound"),
		},
	}
	for i, test := range tests {
		ws, tReward, err := VerifyAndCollectRewardsFromDelegation(test.sdb, test.ds)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Fatalf("Test %v: %v", i, err)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if len(ws) != len(test.expVWrappers) {
			t.Fatalf("vwrapper size unexpected: %v / %v", len(ws), len(test.expVWrappers))
		}
		for wi := range ws {
			if err := staketest.CheckValidatorWrapperEqual(*ws[wi], *test.expVWrappers[wi]); err != nil {
				t.Errorf("%v wrapper: %v", wi, err)
			}
		}
		if tReward.Cmp(test.expTotalRewards) != 0 {
			t.Errorf("Test %v: total Rewards unexpected: %v / %v", i, tReward, test.expTotalRewards)
		}
	}
}

func makeMsgCollectRewards() []staking.DelegationIndex {
	dis := []staking.DelegationIndex{
		{
			ValidatorAddress: validatorAddr,
			Index:            1,
			BlockNum:         big.NewInt(defaultBlockNumber),
		}, {
			ValidatorAddress: validatorAddr2,
			Index:            1,
			BlockNum:         big.NewInt(defaultBlockNumber),
		},
	}
	return dis
}

func makeStateForReward(t *testing.T) *state.DB {
	sdb := makeStateDBForStake(t)

	rewards0 := []*big.Int{reward00, reward01}
	if err := addStateRewardForAddr(sdb, validatorAddr, rewards0); err != nil {
		t.Fatal(err)
	}
	rewards1 := []*big.Int{reward10, reward11}
	if err := addStateRewardForAddr(sdb, validatorAddr2, rewards1); err != nil {
		t.Fatal(err)
	}

	sdb.IntermediateRoot(true)
	return sdb
}

func addStateRewardForAddr(sdb *state.DB, addr common.Address, rewards []*big.Int) error {
	w, err := sdb.ValidatorWrapper(addr, false, true)
	if err != nil {
		return err
	}
	w.Delegations = append(w.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes)),
	)
	w.Delegations[1].Undelegations = staking.Undelegations{}
	w.Delegations[0].Reward = new(big.Int).Set(rewards[0])
	w.Delegations[1].Reward = new(big.Int).Set(rewards[1])

	return sdb.UpdateValidatorWrapper(addr, w)
}

func expVWrappersForReward() []*staking.ValidatorWrapper {
	w1 := makeVWrapperByIndex(validatorIndex)
	w1.Delegations = append(w1.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes)),
	)
	w1.Delegations[1].Undelegations = staking.Undelegations{}
	w1.Delegations[0].Reward = new(big.Int).Set(reward00)
	w1.Delegations[1].Reward = new(big.Int).SetUint64(0)

	w2 := makeVWrapperByIndex(validator2Index)
	w2.Delegations = append(w2.Delegations,
		staking.NewDelegation(delegatorAddr, new(big.Int).Set(twentyKOnes)),
	)
	w2.Delegations[1].Undelegations = staking.Undelegations{}
	w2.Delegations[0].Reward = new(big.Int).Set(reward10)
	w2.Delegations[1].Reward = new(big.Int).SetUint64(0)
	return []*staking.ValidatorWrapper{&w1, &w2}
}

// makeFakeChainContextForStake makes the default fakeChainContext for staking test
func makeFakeChainContextForStake() *fakeChainContext {
	ws := makeVWrappersForStake(defNumWrappersInState, defNumPubPerAddr)
	return makeFakeChainContext(ws)
}

// makeStateDBForStake make the default state db for staking test
func makeStateDBForStake(t *testing.T) *state.DB {
	sdb, err := newTestStateDB()
	if err != nil {
		t.Fatal(err)
	}
	ws := makeVWrappersForStake(defNumWrappersInState, defNumPubPerAddr)
	if err := updateStateValidators(sdb, ws); err != nil {
		t.Fatalf("make default state: %v", err)
	}
	sdb.AddBalance(createValidatorAddr, hundredKOnes)
	sdb.AddBalance(delegatorAddr, hundredKOnes)

	sdb.IntermediateRoot(true)

	return sdb
}

func updateStateValidators(sdb *state.DB, ws []*staking.ValidatorWrapper) error {
	for i, w := range ws {
		sdb.SetValidatorFlag(w.Address)
		sdb.AddBalance(w.Address, hundredKOnes)
		sdb.SetValidatorFirstElectionEpoch(w.Address, big.NewInt(10))
		if err := sdb.UpdateValidatorWrapper(w.Address, w); err != nil {
			return fmt.Errorf("update %v vw error: %v", i, err)
		}
	}
	return nil
}

func makeVWrapperByIndex(index int) staking.ValidatorWrapper {
	pubGetter := newBLSPubGetter(blsKeys[index*defNumPubPerAddr:])

	return makeStateVWrapperFromGetter(index, defNumPubPerAddr, pubGetter)
}

func newTestStateDB() (*state.DB, error) {
	return state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
}

// makeVWrappersForStake makes the default staking.ValidatorWrappers for
// initialization of default state db for staking test
func makeVWrappersForStake(num, numPubsPerVal int) []*staking.ValidatorWrapper {
	ws := make([]*staking.ValidatorWrapper, 0, num)
	pubGetter := newBLSPubGetter(blsKeys)
	for i := 0; i != num; i++ {
		w := makeStateVWrapperFromGetter(i, numPubsPerVal, pubGetter)
		ws = append(ws, &w)
	}
	return ws
}

func makeStateVWrapperFromGetter(index int, numPubs int, pubGetter *BLSPubGetter) staking.ValidatorWrapper {
	addr := makeTestAddr(index)
	pubs := make([]bls.SerializedPublicKey, 0, numPubs)
	for i := 0; i != numPubs; i++ {
		pubs = append(pubs, pubGetter.getPub())
	}
	w := staketest.GetDefaultValidatorWrapperWithAddr(addr, pubs)
	w.Identity = makeIdentityStr(index)
	w.UpdateHeight = big.NewInt(defaultSnapBlockNumber)
	return w
}

type BLSPubGetter struct {
	keys  []blsPubSigPair
	index int
}

func newBLSPubGetter(keys []blsPubSigPair) *BLSPubGetter {
	return &BLSPubGetter{
		keys:  keys,
		index: 0,
	}
}

func (g *BLSPubGetter) getPub() bls.SerializedPublicKey {
	key := g.keys[g.index]
	g.index++
	return key.pub
}

// fakeChainContext is the fake structure of ChainContext for testing
type fakeChainContext struct {
	vWrappers map[common.Address]*staking.ValidatorWrapper
}

func makeFakeChainContext(ws []*staking.ValidatorWrapper) *fakeChainContext {
	m := make(map[common.Address]*staking.ValidatorWrapper)
	for _, w := range ws {
		wCpy := staketest.CopyValidatorWrapper(*w)
		m[w.Address] = &wCpy
	}
	return &fakeChainContext{
		vWrappers: m,
	}
}

func (chain *fakeChainContext) ReadValidatorList() ([]common.Address, error) {
	vs := make([]common.Address, 0, len(chain.vWrappers))
	for addr := range chain.vWrappers {
		vs = append(vs, addr)
	}
	return vs, nil
}

func (chain *fakeChainContext) Engine() consensus_engine.Engine {
	return nil
}

func (chain *fakeChainContext) Config() *params.ChainConfig {
	config := &params.ChainConfig{}
	config.MinCommissionRateEpoch = big.NewInt(0)
	config.MinCommissionPromoPeriod = big.NewInt(10)
	return config
}

func (chain *fakeChainContext) GetHeader(common.Hash, uint64) *block.Header {
	return nil
}

func (chain *fakeChainContext) ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error) {
	return nil, nil
}

func (chain *fakeChainContext) ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (staking.DelegationIndexes, error) {
	return nil, nil
}

func (chain *fakeChainContext) ShardID() uint32 {
	return shard.BeaconChainShardID
}

func (chain *fakeChainContext) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	w, ok := chain.vWrappers[addr]
	if !ok {
		return nil, fmt.Errorf("addr not exist in snapshot")
	}
	cp := staketest.CopyValidatorWrapper(*w)
	return &staking.ValidatorSnapshot{
		Validator: &cp,
		Epoch:     big.NewInt(defaultEpoch),
	}, nil
}

type fakeErrChainContext struct{}

func (chain *fakeErrChainContext) ReadValidatorList() ([]common.Address, error) {
	return nil, errors.New("error intended from chain")
}

func (chain *fakeErrChainContext) Engine() consensus_engine.Engine {
	return nil
}

func (chain *fakeErrChainContext) GetHeader(common.Hash, uint64) *block.Header {
	return nil
}

func (chain *fakeErrChainContext) Config() *params.ChainConfig {
	config := &params.ChainConfig{}
	config.MinCommissionRateEpoch = big.NewInt(10)
	return config
}

func (chain *fakeErrChainContext) ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error) {
	return nil, nil
}

func (chain *fakeErrChainContext) ReadDelegationsByDelegatorAt(delegator common.Address, blockNum *big.Int) (staking.DelegationIndexes, error) {
	return nil, nil
}

func (chain *fakeErrChainContext) ShardID() uint32 {
	return 900 // arbitrary number different from BeaconChainShardID
}

func (chain *fakeErrChainContext) ReadValidatorSnapshot(common.Address) (*staking.ValidatorSnapshot, error) {
	return nil, errors.New("error intended")
}

func makeIdentityStr(item interface{}) string {
	return fmt.Sprintf("harmony-one-%v", item)
}

func makeTestAddr(item interface{}) common.Address {
	s := fmt.Sprintf("harmony-one-%v", item)
	return common.BytesToAddress([]byte(s))
}

func makeKeyPairs(size int) []blsPubSigPair {
	pairs := make([]blsPubSigPair, 0, size)
	for i := 0; i != size; i++ {
		pairs = append(pairs, makeBLSKeyPair())
	}
	return pairs
}

type blsPubSigPair struct {
	pub bls.SerializedPublicKey
	sig bls.SerializedSignature
}

func makeBLSKeyPair() blsPubSigPair {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub bls.SerializedPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig bls.SerializedSignature
	copy(shardSig[:], sig.Serialize())

	return blsPubSigPair{shardPub, shardSig}
}

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	if (got == nil) || (expect == nil) {
		return nil
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, expect)
	}
	return nil
}
