package core

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/bls"
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
)

var (
	blsKeys = makeKeyPairs(20)

	createValidatorAddr = makeTestAddr("validator")
	validatorAddr       = makeTestAddr(0)
	delegatorAddr       = makeTestAddr(1)
)

var (
	oneBig          = big.NewInt(1e18)
	oneKOnes        = new(big.Int).Mul(big.NewInt(1000), oneBig)
	fiveKOnes       = new(big.Int).Mul(big.NewInt(5000), oneBig)
	tenKOnes        = new(big.Int).Mul(big.NewInt(10000), oneBig)
	fifteenOnes     = new(big.Int).Mul(big.NewInt(15000), oneBig)
	twentyKOnes     = new(big.Int).Mul(big.NewInt(20000), oneBig)
	twentyFiveKOnes = new(big.Int).Mul(big.NewInt(25000), oneBig)
	thirtyKOnes     = new(big.Int).Mul(big.NewInt(30000), oneBig)
	hundredKOnes    = new(big.Int).Mul(big.NewInt(100000), oneBig)

	negRate           = numeric.NewDecWithPrec(-1, 10)
	zeroDec           = numeric.ZeroDec()
	pointOneDec       = numeric.NewDecWithPrec(1, 1)
	pointTwoDec       = numeric.NewDecWithPrec(2, 1)
	pointFiveDec      = numeric.NewDecWithPrec(5, 1)
	pointSevenDec     = numeric.NewDecWithPrec(7, 1)
	pointEightFiveDec = numeric.NewDecWithPrec(85, 2)
	pointNineDec      = numeric.NewDecWithPrec(9, 1)
	oneDec            = numeric.OneDec()
)

const (
	defaultEpoch           = 5
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
		pubs      []shard.BLSPublicKey

		expErr error
	}{
		{
			// new validator
			bc:        makeDefaultFakeChainContext(),
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []shard.BLSPublicKey{blsKeys[11].pub},

			expErr: nil,
		},
		{
			// validator skip self check
			bc:        makeDefaultFakeChainContext(),
			sdb:       makeDefaultStateDB(t),
			validator: makeTestAddr(0),
			identity:  makeIdentityStr(0),
			pubs:      []shard.BLSPublicKey{blsKeys[0].pub, blsKeys[1].pub},

			expErr: nil,
		},
		{
			// empty bls keys
			bc:        makeDefaultFakeChainContext(),
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []shard.BLSPublicKey{},

			expErr: nil,
		},
		{
			// empty identity will not collide
			bc: makeDefaultFakeChainContext(),
			sdb: func(t *testing.T) *state.DB {
				sdb := makeDefaultStateDB(t)
				vw, err := sdb.ValidatorWrapper(makeTestAddr(0))
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
			pubs:      []shard.BLSPublicKey{blsKeys[11].pub},

			expErr: nil,
		},
		{
			// chain error
			bc:        &fakeErrChainContext{},
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []shard.BLSPublicKey{blsKeys[11].pub},

			expErr: errors.New("error intended"),
		},
		{
			// validators read from chain not in state
			bc: func() *fakeChainContext {
				chain := makeDefaultFakeChainContext()
				addr := makeTestAddr("not exist in state")
				w := staketest.GetDefaultValidatorWrapper()
				chain.vWrappers[addr] = &w
				return chain
			}(),
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []shard.BLSPublicKey{blsKeys[11].pub},

			expErr: errors.New("address not present in state"),
		},
		{
			// duplicate identity
			bc:        makeDefaultFakeChainContext(),
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr(0),
			pubs:      []shard.BLSPublicKey{blsKeys[11].pub},

			expErr: errDupIdentity,
		},
		{
			// bls key duplication
			bc:        makeDefaultFakeChainContext(),
			sdb:       makeDefaultStateDB(t),
			validator: createValidatorAddr,
			identity:  makeIdentityStr("new validator"),
			pubs:      []shard.BLSPublicKey{blsKeys[0].pub},

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
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expWrapper: defaultExpWrapperCreateValidator(),
		},
		{
			// 1: nil state db
			sdb:      nil,
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errStateDBIsMissing,
		},
		{
			// 2: nil chain context
			sdb:      makeDefaultStateDB(t),
			chain:    nil,
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errChainContextMissing,
		},
		{
			// 3: nil epoch
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
			epoch:    nil,
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errEpochMissing,
		},
		{
			// 4: nil block number
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: nil,
			msg:      defaultMsgCreateValidator(),

			expErr: errBlockNumMissing,
		},
		{
			// 5: negative amount
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
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
				sdb := makeDefaultStateDB(t)
				sdb.SetValidatorFlag(createValidatorAddr)
				return sdb
			}(),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errValidatorExist,
		},
		{
			// 7: bls collision (checkDuplicateFields)
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.SlotPubKeys = []shard.BLSPublicKey{blsKeys[0].pub}
				return m
			}(),

			expErr: errors.New("BLS key exists"),
		},
		{
			// 8: insufficient balance
			sdb: func() *state.DB {
				sdb := makeDefaultStateDB(t)
				bal := new(big.Int).Sub(staketest.DefaultDelAmount, common.Big1)
				sdb.SetBalance(createValidatorAddr, bal)
				return sdb
			}(),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgCreateValidator(),

			expErr: errInsufficientBalanceForStake,
		},
		{
			// 9: incorrect signature
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg: func() staking.CreateValidator {
				m := defaultMsgCreateValidator()
				m.SlotKeySigs = []shard.BLSSignature{blsKeys[12].sig}
				return m
			}(),

			expErr: errors.New("bls keys and corresponding signatures"),
		},
		{
			// 10: small self delegation amount (fail sanity check)
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
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
			sdb:      makeDefaultStateDB(t),
			chain:    makeDefaultFakeChainContext(),
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

		if !reflect.DeepEqual(*w, test.expWrapper) {
			t.Errorf("Test %v: vWrapper not deep equal", i)
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
		SlotPubKeys:        []shard.BLSPublicKey{pub},
		SlotKeySigs:        []shard.BLSSignature{sig},
		Amount:             staketest.DefaultDelAmount,
	}
	return cv
}

func defaultExpWrapperCreateValidator() staking.ValidatorWrapper {
	pub := blsKeys[11].pub
	v := staking.Validator{
		Address:              createValidatorAddr,
		SlotPubKeys:          []shard.BLSPublicKey{pub},
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
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expWrapper: defaultExpWrapperEditValidator(),
		},
		{
			// 1: If the rate is not changed, UpdateHeight is not update
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
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
				vw.Rate = pointFiveDec
				return vw
			}(),
		},
		{
			// 2: nil state db
			sdb:      nil,
			bc:       makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errStateDBIsMissing,
		},
		{
			// 3: nil chain
			sdb:      makeDefaultStateDB(t),
			bc:       nil,
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errChainContextMissing,
		},
		{
			// 4: nil block number
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: nil,
			msg:      defaultMsgEditValidator(),

			expErr: errBlockNumMissing,
		},
		{
			// 5: edited validator flag not set in state
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
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
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
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
				sdb := makeDefaultStateDB(t)
				sdb.SetValidatorFlag(makeTestAddr("someone"))
				return sdb
			}(),
			bc: func() *fakeChainContext {
				chain := makeDefaultFakeChainContext()
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
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
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
			sdb: makeDefaultStateDB(t),
			bc: func() *fakeChainContext {
				chain := makeDefaultFakeChainContext()
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
			sdb: makeDefaultStateDB(t),
			bc: func() *fakeChainContext {
				chain := makeDefaultFakeChainContext()
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
			sdb:      makeDefaultStateDB(t),
			bc:       makeDefaultFakeChainContext(),
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
			sdb: makeDefaultStateDB(t),
			bc: func() *fakeChainContext {
				chain := makeDefaultFakeChainContext()
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

			expErr: errors.New("change rate and max rate should be within 0-100 percent"),
		},
		{
			// 13: cannot update a banned validator
			sdb: func(t *testing.T) *state.DB {
				sdb := makeDefaultStateDB(t)
				vw, err := sdb.ValidatorWrapper(validatorAddr)
				if err != nil {
					t.Fatal(err)
				}
				vw.Status = effective.Banned
				if err := sdb.UpdateValidatorWrapper(validatorAddr, vw); err != nil {
					t.Fatal(err)
				}
				return sdb
			}(t),
			bc:       makeDefaultFakeChainContext(),
			epoch:    big.NewInt(defaultEpoch),
			blockNum: big.NewInt(defaultBlockNumber),
			msg:      defaultMsgEditValidator(),

			expErr: errors.New("banned status"),
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
		if !reflect.DeepEqual(w, &test.expWrapper) {
			t.Errorf("Test %v: wrapper not expected", i)
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
		pub0Copy  shard.BLSPublicKey
		pub12Copy shard.BLSPublicKey
		sig12Copy shard.BLSSignature
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
	newPubs := []shard.BLSPublicKey{blsKeys[1].pub, blsKeys[12].pub}
	w := staketest.GetDefaultValidatorWrapperWithAddr(makeTestAddr(0), newPubs)
	w.Description = editExpDesc
	w.Rate = pointTwoDec
	w.UpdateHeight = big.NewInt(defaultBlockNumber)
	w.Status = effective.Inactive
	return w
}

func TestVerifyAndDelegateFromMsg(t *testing.T) {
	tests := []struct {
		sdb vm.StateDB
		msg staking.Delegate

		expVWrapper staking.ValidatorWrapper
		expAmt      *big.Int
		expErr      error
	}{
		{},
	}
	for i, test := range tests {
		w, amt, err := VerifyAndDelegateFromMsg(test.sdb, &test.msg)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

		if amt.Cmp(test.expAmt) != 0 {
			t.Errorf("Test %v: unexpected amount %v / %v", i, amt, test.expAmt)
		}
		if !reflect.DeepEqual(w, test.expVWrapper) {
			t.Errorf("Test %v: vWrapper not expected", i)
		}
	}
}

func defaultDelegateMsg() staking.Delegate {
	return staking.Delegate{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: validatorAddr,
		Amount:           new(big.Int).Set(tenKOnes),
	}
}

func makeDefaultFakeChainContext() *fakeChainContext {
	ws := makeDefaultCurrentStateVWrappers(defNumWrappersInState, defNumPubPerAddr)
	return makeFakeChainContext(ws)
}

func makeDefaultStateDB(t *testing.T) *state.DB {
	sdb, err := newTestStateDB()
	if err != nil {
		t.Fatal(err)
	}
	ws := makeDefaultCurrentStateVWrappers(defNumWrappersInState, defNumPubPerAddr)
	if err := updateStateVWrappers(sdb, ws); err != nil {
		t.Fatalf("make default state: %v", err)
	}

	sdb.IntermediateRoot(true)

	return sdb
}

func updateStateVWrappers(sdb *state.DB, ws []*staking.ValidatorWrapper) error {
	for i, w := range ws {
		sdb.SetValidatorFlag(w.Address)
		sdb.AddBalance(w.Address, hundredKOnes)
		if err := sdb.UpdateValidatorWrapper(w.Address, w); err != nil {
			return fmt.Errorf("update %v vw error: %v", i, err)
		}
	}
	return nil
}

func newTestStateDB() (*state.DB, error) {
	return state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
}

// makeDefaultCurrentStateVWrappers makes the default staking.ValidatorWrappers for
// initialization of default state db
func makeDefaultCurrentStateVWrappers(num, numPubsPerVal int) []*staking.ValidatorWrapper {
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
	pubs := make([]shard.BLSPublicKey, 0, numPubs)
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

func (g *BLSPubGetter) getPub() shard.BLSPublicKey {
	key := g.keys[g.index]
	g.index++
	return key.pub
}

//func TestVerifyAndCreateValidatorFromMsg(t *testing.T) {
//
//}
//
//type cvTestCase struct {
//	sdb   vm.StateDB
//	chain ChainContext
//	msg   *staking.CreateValidator
//
//	gotVW  *staking.ValidatorWrapper
//	gotErr error
//
//	expErr     error
//	expVW      *staking.ValidatorWrapper
//	expBalance *big.Int
//}
//
//func (tc *cvTestCase) apply() {
//	epoch := big.NewInt(defaultEpoch)
//	bn := big.NewInt(defaultBlockNumber)
//	tc.gotVW, tc.gotErr = VerifyAndCreateValidatorFromMsg(tc.sdb, tc.chain, epoch, bn, tc.msg)
//}
//
//func (tc *cvTestCase) checkResult() error {
//	if err := assertError(tc.gotErr, tc.expErr); err != nil {
//		return err
//	}
//
//}

//// testDataSetup setup the data for testing in init process.
//func testDataSetup() {
//	validatorAddress = common.Address(common2.MustBech32ToAddress("one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"))
//	validatorBalance = big.NewInt(5e18)
//
//	defaultMaxTotalDelegation = new(big.Int).Mul(big.NewInt(90000), oneAsBigInt)
//	defaultMinSelfDelegation = new(big.Int).Mul(big.NewInt(10000), oneAsBigInt)
//	defaultSelfDelegation = new(big.Int).Mul(big.NewInt(50000), oneAsBigInt)
//
//	blsKeys = newBLSKeyPool()
//}
//
//// defaultMsgCreateValidator makes a createValidator message for testing.
//// Returned message could be treated as a copy of a valid message prototype.
//func defaultMsgCreateValidator() *staking.CreateValidator {
//	desc := staking.Description{
//		Name:            "SuperHero",
//		Identity:        "YouWouldNotKnow",
//		Website:         "Secret Website",
//		SecurityContact: "LicenseToKill",
//		Details:         "blah blah blah",
//	}
//	rate, _ := numeric.NewDecFromStr("0.1")
//	maxRate, _ := numeric.NewDecFromStr("0.5")
//	maxChangeRate, _ := numeric.NewDecFromStr("0.05")
//	commission := staking.CommissionRates{
//		Rate:          rate,
//		MaxRate:       maxRate,
//		MaxChangeRate: maxChangeRate,
//	}
//	minSelfDel := big.NewInt(1e18)
//	maxTotalDel := big.NewInt(9e18)
//	slotPubKeys := []shard.BLSPublicKey{blsKeys.get(0).pub}
//	slotKeySigs := []shard.BLSSignature{blsKeys.get(0).sig}
//	amount := validatorBalance
//	v := staking.CreateValidator{
//		ValidatorAddress:   validatorAddress,
//		Description:        desc,
//		CommissionRates:    commission,
//		MinSelfDelegation:  minSelfDel,
//		MaxTotalDelegation: maxTotalDel,
//		SlotPubKeys:        slotPubKeys,
//		SlotKeySigs:        slotKeySigs,
//		Amount:             amount,
//	}
//	return &v
//}

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

func (chain *fakeChainContext) GetHeader(common.Hash, uint64) *block.Header {
	return nil
}

func (chain *fakeChainContext) ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error) {
	return nil, nil
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

func (chain *fakeErrChainContext) ReadDelegationsByDelegator(common.Address) (staking.DelegationIndexes, error) {
	return nil, nil
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
	pub shard.BLSPublicKey
	sig shard.BLSSignature
}

func makeBLSKeyPair() blsPubSigPair {
	blsPriv := bls.RandPrivateKey()
	blsPub := blsPriv.GetPublicKey()
	msgHash := hash.Keccak256([]byte(staking.BLSVerificationStr))
	sig := blsPriv.SignHash(msgHash)

	var shardPub shard.BLSPublicKey
	copy(shardPub[:], blsPub.Serialize())

	var shardSig shard.BLSSignature
	copy(shardSig[:], sig.Serialize())

	return blsPubSigPair{shardPub, shardSig}
}

func assertError(got, expect error) error {
	if (got == nil) != (expect == nil) {
		return fmt.Errorf("unexpected error %v / %v", got, expect)
	}
	if (got == nil) || (expect == nil) {
		return nil
	}
	if !strings.Contains(got.Error(), expect.Error()) {
		return fmt.Errorf("unexpected error %v / %v", got, expect)
	}
	return nil
}
