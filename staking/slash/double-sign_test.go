package slash

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/harmony-one/harmony/core/rawdb"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	bigOne          = big.NewInt(1e18)
	fiveKOnes       = new(big.Int).Mul(big.NewInt(5000), bigOne)
	tenKOnes        = new(big.Int).Mul(big.NewInt(10000), bigOne)
	twentyKOnes     = new(big.Int).Mul(big.NewInt(20000), bigOne)
	twentyFiveKOnes = new(big.Int).Mul(big.NewInt(25000), bigOne)
	thirtyKOnes     = new(big.Int).Mul(big.NewInt(30000), bigOne)
	thirtyFiveKOnes = new(big.Int).Mul(big.NewInt(35000), bigOne)
	fourtyKOnes     = new(big.Int).Mul(big.NewInt(40000), bigOne)
	fiftyKOnes      = new(big.Int).Mul(big.NewInt(50000), bigOne)
	hundredKOnes    = new(big.Int).Mul(big.NewInt(100000), bigOne)
	thousandKOnes   = new(big.Int).Mul(big.NewInt(1000000), bigOne)
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
)

const (
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

	leaderAddr   = makeTestAddress("leader")
	reporterAddr = makeTestAddress("reporter")
)

func TestVerify(t *testing.T) {
	tests := []struct {
		r      Record
		sdb    *state.DB
		chain  *fakeBlockChain
		expErr error
	}{
		{
			r:     defaultSlashRecord(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: nil,
		},
		{
			// not vWrapper in state
			r:     defaultSlashRecord(),
			sdb:   makeTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errors.New("address not present in state"),
		},
		{
			// banned status
			r: defaultSlashRecord(),
			sdb: func() *state.DB {
				sdb := makeTestStateDB()
				w := defaultValidatorWrapper()
				w.Status = effective.Banned
				if err := sdb.UpdateValidatorWrapper(offAddr, w); err != nil {
					panic(err)
				}
				return sdb
			}(),
			chain: defaultFakeBlockChain(),

			expErr: errAlreadyBannedValidator,
		},
		{
			// same offender and reporter
			r: func() Record {
				r := defaultSlashRecord()
				r.Reporter = offAddr
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errReporterAndOffenderSame,
		},
		{
			// same block in conflicting votes
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.SecondVote = copyVote(r.Evidence.FirstVote)
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errSlashBlockNoConflict,
		},
		{
			// second vote signed with a different bls key
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.SecondVote = makeVoteData(keyPairs[24], doubleSignBlock2)
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errNoMatchingDoubleSignKeys,
		},
		{
			// block is in the future
			r:   defaultSlashRecord(),
			sdb: defaultTestStateDB(),
			chain: func() *fakeBlockChain {
				bc := defaultFakeBlockChain()
				bc.currentBlock = *makeBlockForTest(doubleSignEpoch-1, 0)
				return bc
			}(),

			expErr: errSlashFromFutureEpoch,
		},
		{
			// error from blockchain.ReadShardState (fakeChainErrEpoch)
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.Epoch = big.NewInt(currentEpoch)
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errFakeChainUnexpectEpoch,
		},
		{
			// Invalid shardID
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.ShardID = 10
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: shard.ErrShardIDNotInSuperCommittee,
		},
		{
			// Missing bls from committee
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.FirstVote = makeVoteData(keyPairs[24], doubleSignBlock1)
				r.Evidence.SecondVote = makeVoteData(keyPairs[24], doubleSignBlock2)
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: shard.ErrValidNotInCommittee,
		},
		{
			// offender address not match vote
			r:   defaultSlashRecord(),
			sdb: defaultTestStateDB(),
			chain: func() *fakeBlockChain {
				bc := defaultFakeBlockChain()
				bc.superCommittee.Shards[offenderShard].Slots[offenderShardIndex].
					EcdsaAddress = makeTestAddress("other")
				return bc
			}(),

			expErr: errors.New("does not match the signer's address"),
		},
		{
			// empty signature
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.FirstVote.Signature = nil
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errors.New("Empty buf"),
		},
		{
			// false signature
			r: func() Record {
				r := defaultSlashRecord()
				r.Evidence.FirstVote.Signature = r.Evidence.SecondVote.Signature
				return r
			}(),
			sdb:   defaultTestStateDB(),
			chain: defaultFakeBlockChain(),

			expErr: errors.New("could not verify bls key signature on slash"),
		},
	}
	for i, test := range tests {
		rawRecord := copyRecord(test.r)

		err := Verify(test.chain, test.sdb, &test.r)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if !reflect.DeepEqual(test.r, rawRecord) {
			t.Errorf("Test %v: record has value changed", i)
		}
	}
}

func TestApplySlashRate(t *testing.T) {
	tests := []struct {
		amount *big.Int
		rate   numeric.Dec
		exp    *big.Int
	}{
		{twentyKOnes, numeric.NewDecWithPrec(5, 1), tenKOnes},
		{bigOne, numeric.NewDecWithPrec(5, 1), big.NewInt(5e17)},
		{new(big.Int).Mul(bigOne, big.NewInt(3)), numeric.NewDecWithPrec(5, 1), big.NewInt(15e17)},
		{twentyKOnes, numeric.ZeroDec(), big.NewInt(0)},
	}
	for i, test := range tests {
		res := applySlashRate(test.amount, test.rate)
		if res.Cmp(test.exp) != 0 {
			t.Errorf("Test %v: unexpected answer %v / %v", i, res, test.exp)
		}
	}
}

func TestSetDifference(t *testing.T) {
	tests := []struct {
		indexes1   []int
		indexes2   []int
		expIndexes []int
	}{
		{[]int{}, []int{}, []int{}},
		{[]int{1}, []int{}, []int{}},
		{[]int{}, []int{1}, []int{1}},
		{[]int{1, 2, 3}, []int{3, 4, 5}, []int{4, 5}},
	}
	for ti, test := range tests {
		rs1 := makeSimpleRecords(test.indexes1)
		rs2 := makeSimpleRecords(test.indexes2)

		diff := rs1.SetDifference(rs2)

		if len(diff) != len(test.expIndexes) {
			t.Errorf("Test %v: size not expected %v, %v", ti, len(diff), len(test.expIndexes))
		}
		for i, r := range diff {
			if r.Reporter != common.BigToAddress(big.NewInt(int64(test.expIndexes[i]))) {
				t.Errorf("Test %v: Records[i] unexpected", ti)
			}
		}
	}
}

func makeSimpleRecords(indexes []int) Records {
	rs := make(Records, 0, len(indexes))
	for _, index := range indexes {
		rs = append(rs, Record{
			Reporter: common.BigToAddress(big.NewInt(int64(index))),
		})
	}
	return rs
}

func TestPayDown(t *testing.T) {
	tests := []struct {
		debt, amt *big.Int
		diff      *Application

		expDebt, expAmt *big.Int
		expDiff         *Application
		expErr          error
	}{
		{
			debt: new(big.Int).Set(twentyFiveKOnes),
			amt:  new(big.Int).Set(thirtyKOnes),
			diff: &Application{
				TotalSlashed:           new(big.Int).Set(tenKOnes),
				TotalBeneficiaryReward: new(big.Int).Set(tenKOnes),
			},
			expDebt: common.Big0,
			expAmt:  fiveKOnes,
			expDiff: &Application{
				TotalSlashed:           thirtyFiveKOnes,
				TotalBeneficiaryReward: tenKOnes,
			},
		},
		{
			debt: new(big.Int).Set(thirtyKOnes),
			amt:  new(big.Int).Set(twentyFiveKOnes),
			diff: &Application{
				TotalSlashed:           new(big.Int).Set(tenKOnes),
				TotalBeneficiaryReward: new(big.Int).Set(tenKOnes),
			},
			expDebt: fiveKOnes,
			expAmt:  common.Big0,
			expDiff: &Application{
				TotalSlashed:           thirtyFiveKOnes,
				TotalBeneficiaryReward: tenKOnes,
			},
		},
	}
	for i, test := range tests {
		payDown(test.amt, test.debt, test.diff.TotalSlashed)
		if test.debt.Cmp(test.expDebt) != 0 {
			t.Errorf("Test %v: unexpected debt %v / %v", i, test.debt, test.expDebt)
		}
		if test.amt.Cmp(test.expAmt) != 0 {
			t.Errorf("Test %v: unexpected amount %v / %v", i, test.amt, test.expAmt)
		}
		if test.diff.TotalSlashed.Cmp(test.expDiff.TotalSlashed) != 0 {
			t.Errorf("Test %v: unexpected expSlashed %v / %v", i, test.diff.TotalSlashed,
				test.expDiff.TotalSlashed)
		}
		if test.diff.TotalBeneficiaryReward.Cmp(test.expDiff.TotalBeneficiaryReward) != 0 {
			t.Errorf("Test %v: unexpected total beneficiary reward %v / %v", i,
				test.diff.TotalBeneficiaryReward, test.expDiff.TotalBeneficiaryReward)
		}
	}
}

func TestApplySlashingToDelegator(t *testing.T) {
	tests := []applySlashingToDelegatorTestCase{
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          big.NewInt(0),
			expDel: expDelegation{
				expAmt:      twentyKOnes,
				expReward:   tenKOnes,
				expUndelAmt: []*big.Int{tenKOnes, tenKOnes},
			},
			expSlashed:           common.Big0,
			expBeneficiaryReward: common.Big0,
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          twentyKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   tenKOnes,
				expUndelAmt: []*big.Int{tenKOnes, tenKOnes},
			},
			expSlashed:           twentyKOnes,
			expBeneficiaryReward: tenKOnes,
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          twentyFiveKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   tenKOnes,
				expUndelAmt: []*big.Int{tenKOnes, fiveKOnes},
			},
			expSlashed:           twentyFiveKOnes,
			expBeneficiaryReward: new(big.Int).Div(twentyFiveKOnes, common.Big2),
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          thirtyKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   tenKOnes,
				expUndelAmt: []*big.Int{tenKOnes, big.NewInt(0)},
			},
			expSlashed:           thirtyKOnes,
			expBeneficiaryReward: new(big.Int).Div(thirtyKOnes, common.Big2),
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          thirtyFiveKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   fiveKOnes,
				expUndelAmt: []*big.Int{tenKOnes, big.NewInt(0)},
			},
			expSlashed:           thirtyFiveKOnes,
			expBeneficiaryReward: new(big.Int).Div(thirtyFiveKOnes, common.Big2),
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          fourtyKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   big.NewInt(0),
				expUndelAmt: []*big.Int{tenKOnes, big.NewInt(0)},
			},
			expSlashed:           fourtyKOnes,
			expBeneficiaryReward: twentyKOnes,
		},
		{
			snapshot:      defaultSnapValidatorWrapper(),
			current:       defaultCurrentValidatorWrapper(),
			delegationIdx: 0,
			debt:          fiftyKOnes,
			expDel: expDelegation{
				expAmt:      big.NewInt(0),
				expReward:   big.NewInt(0),
				expUndelAmt: []*big.Int{tenKOnes, big.NewInt(0)},
			},
			expSlashed:           fourtyKOnes,
			expBeneficiaryReward: twentyKOnes,
		},
	}
	for i, tc := range tests {
		tc.makeData()
		tc.apply()

		if err := tc.checkResult(); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func TestDelegatorSlashApply(t *testing.T) {
	tests := []slashApplyTestCase{
		{
			snapshot: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fourtyKOnes,
				},
			}),
			current: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fourtyKOnes,
				},
			}),
			expDels: []expDelegation{
				{
					expAmt:      twentyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed:           twentyKOnes,
			expBeneficiaryReward: tenKOnes,
		},
		{
			snapshot: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fourtyKOnes,
				},
				{
					address: "del1",
					amount:  fourtyKOnes,
				},
			}),
			current: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fourtyKOnes,
				},
				{
					address: "del1",
					amount:  fourtyKOnes,
				},
			}),
			expDels: []expDelegation{
				{
					expAmt:      twentyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
				{
					expAmt:      new(big.Int).Mul(big.NewInt(24000), bigOne),
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed:           new(big.Int).Mul(big.NewInt(36000), bigOne),
			expBeneficiaryReward: new(big.Int).Mul(big.NewInt(18000), bigOne),
		},
		{
			snapshot: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fiftyKOnes,
				},
				{
					address: "del1",
					amount:  fourtyKOnes,
				},
				{
					address: "del2",
					amount:  tenKOnes,
				},
			}),
			current: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  fiftyKOnes,
				},
				{
					address: "del1",
					amount:  fourtyKOnes,
				},
				{
					address: "del2",
					amount:  tenKOnes,
				},
			}),
			expDels: []expDelegation{
				{
					expAmt:      twentyFiveKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
				{
					expAmt:      new(big.Int).Mul(big.NewInt(24000), bigOne),
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
				{
					expAmt:      new(big.Int).Mul(big.NewInt(6000), bigOne),
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed:           new(big.Int).Mul(big.NewInt(45000), bigOne),
			expBeneficiaryReward: new(big.Int).Mul(big.NewInt(22500), bigOne),
		},
		{
			snapshot: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  hundredKOnes,
				},
				{
					address: "del1",
					amount:  twentyKOnes,
				},
				{
					address: "del2",
					amount:  twentyKOnes,
				},
			}),
			current: generateValidatorWrapper([]testDelegation{
				{
					address: "off",
					amount:  hundredKOnes,
				},
				{
					address:        "del1",
					amount:         common.Big0,
					historyUndel:   twentyKOnes,
					afterSignUndel: common.Big0,
				},
				{
					address:        "del2",
					amount:         common.Big0,
					historyUndel:   common.Big0,
					afterSignUndel: twentyKOnes,
				},
			}),
			expDels: []expDelegation{
				{
					expAmt:      fiftyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
				{
					expAmt:      common.Big0,
					expReward:   common.Big0,
					expUndelAmt: []*big.Int{twentyKOnes, common.Big0},
				},
				{
					expAmt:      common.Big0,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{common.Big0, common.Big0},
				},
			},
			expSlashed:           new(big.Int).Mul(big.NewInt(80000), bigOne),
			expBeneficiaryReward: fourtyKOnes,
		},
	}

	for i, tc := range tests {
		tc.makeData()
		tc.apply()

		if err := tc.checkResult(); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

type applySlashingToDelegatorTestCase struct {
	snapshot, current *staking.ValidatorWrapper
	state             *state.DB
	beneficiary       common.Address
	slashTrack        *Application
	debt              *big.Int
	delegationIdx     int

	gotErr error

	expDel                           expDelegation
	expSlashed, expBeneficiaryReward *big.Int
	expErr                           error
}

type slashApplyTestCase struct {
	snapshot, current *staking.ValidatorWrapper

	beneficiary common.Address
	state       *state.DB
	slashTrack  *Application
	gotErr      error

	expDels                          []expDelegation
	expSlashed, expBeneficiaryReward *big.Int
	expErr                           error
}

func (tc *applySlashingToDelegatorTestCase) makeData() {
	tc.beneficiary = leaderAddr
	tc.state = makeTestStateDB()
	tc.slashTrack = &Application{
		TotalSlashed:           new(big.Int).Set(common.Big0),
		TotalBeneficiaryReward: new(big.Int).Set(common.Big0),
	}
}

func (tc *applySlashingToDelegatorTestCase) apply() {
	tc.gotErr = delegatorSlashApplyDebt(tc.snapshot, tc.current, tc.state, tc.debt, tc.beneficiary,
		big.NewInt(doubleSignEpoch), tc.slashTrack)
}

func (tc *applySlashingToDelegatorTestCase) checkResult() error {
	if err := assertError(tc.gotErr, tc.expErr); err != nil {
		return err
	}
	if err := tc.expDel.checkDelegation(tc.current.Delegations[tc.delegationIdx]); err != nil {
		return fmt.Errorf("delegations[%v]: %v", tc.delegationIdx, err)
	}
	if tc.slashTrack.TotalSlashed.Cmp(tc.expSlashed) != 0 {
		return fmt.Errorf("unexpected total slash %v / %v", tc.slashTrack.TotalSlashed,
			tc.expSlashed)
	}
	if tc.slashTrack.TotalBeneficiaryReward.Cmp(tc.expBeneficiaryReward) != 0 {
		return fmt.Errorf("unexpected beneficiary reward %v / %v", tc.slashTrack.TotalBeneficiaryReward,
			tc.expBeneficiaryReward)
	}
	if bal := tc.state.GetBalance(tc.beneficiary); bal.Cmp(tc.expBeneficiaryReward) != 0 {
		return fmt.Errorf("unexpected balance for beneficiary %v / %v", bal, tc.expBeneficiaryReward)
	}
	return nil
}

func (tc *slashApplyTestCase) makeData() {
	tc.beneficiary = leaderAddr
	tc.state = makeTestStateDB()
	tc.slashTrack = &Application{
		TotalSlashed:           new(big.Int).Set(common.Big0),
		TotalBeneficiaryReward: new(big.Int).Set(common.Big0),
	}
}

func (tc *slashApplyTestCase) apply() {
	tc.gotErr = delegatorSlashApply(tc.snapshot, tc.current, tc.state, tc.beneficiary, big.NewInt(doubleSignEpoch), tc.slashTrack)
}

func (tc *slashApplyTestCase) checkResult() error {
	if err := assertError(tc.gotErr, tc.expErr); err != nil {
		return err
	}
	if len(tc.expDels) != len(tc.current.Delegations) {
		return fmt.Errorf("delegations size not expected %v / %v",
			len(tc.current.Delegations), len(tc.expDels))
	}
	for i, expDel := range tc.expDels {
		if err := expDel.checkDelegation(tc.current.Delegations[i]); err != nil {
			return fmt.Errorf("delegations[%v]: %v", i, err)
		}
	}
	if tc.slashTrack.TotalSlashed.Cmp(tc.expSlashed) != 0 {
		return fmt.Errorf("unexpected total slash %v / %v", tc.slashTrack.TotalSlashed,
			tc.expSlashed)
	}
	if tc.slashTrack.TotalBeneficiaryReward.Cmp(tc.expBeneficiaryReward) != 0 {
		return fmt.Errorf("unexpected beneficiary reward %v / %v", tc.slashTrack.TotalBeneficiaryReward,
			tc.expBeneficiaryReward)
	}
	if bal := tc.state.GetBalance(tc.beneficiary); bal.Cmp(tc.expBeneficiaryReward) != 0 {
		return fmt.Errorf("unexpected balance for beneficiary %v / %v", bal, tc.expBeneficiaryReward)
	}
	return nil
}

type expDelegation struct {
	expAmt, expReward *big.Int
	expUndelAmt       []*big.Int
}

type testDelegation struct {
	address        string
	amount         *big.Int
	historyUndel   *big.Int
	afterSignUndel *big.Int
}

func (ed expDelegation) checkDelegation(d staking.Delegation) error {
	if d.Amount.Cmp(ed.expAmt) != 0 {
		return fmt.Errorf("unexpected amount %v / %v", d.Amount, ed.expAmt)
	}
	if d.Reward.Cmp(ed.expReward) != 0 {
		return fmt.Errorf("unexpected reward %v / %v", d.Reward, ed.expReward)
	}
	if len(d.Undelegations) != len(ed.expUndelAmt) {
		return fmt.Errorf("unexpected undelegation size %v / %v", len(d.Undelegations),
			len(ed.expUndelAmt))
	}
	for i := range d.Undelegations {
		if ed.expUndelAmt[i].Cmp(d.Undelegations[i].Amount) != 0 {
			return fmt.Errorf("[%v]th undelegation unexpected amount %v / %v", i,
				d.Undelegations[i].Amount, ed.expUndelAmt[i])
		}
	}
	return nil
}

func TestApply(t *testing.T) {
	tests := []applyTestCase{
		{
			// positive test case
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			slashes:  Records{defaultSlashRecord()},

			expSlashed:           twentyKOnes,
			expBeneficiaryReward: tenKOnes,
		},
		{
			// missing snapshot in chain
			current: defaultCurrentValidatorWrapper(),
			slashes: Records{defaultSlashRecord()},

			expErr: errors.New("could not find validator"),
		},
		{
			// missing vWrapper in state
			snapshot: defaultSnapValidatorWrapper(),
			slashes:  Records{defaultSlashRecord()},

			expErr: errValidatorNotFoundDuringSlash,
		},
	}
	for i, test := range tests {
		test.makeData(t)

		test.apply()

		if err := test.checkResult(); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

type applyTestCase struct {
	snapshot, current *staking.ValidatorWrapper
	slashes           Records

	chain            *fakeBlockChain
	state, stateSnap *state.DB
	gotErr           error
	gotDiff          *Application

	expSlashed           *big.Int
	expBeneficiaryReward *big.Int
	expErr               error
}

func (tc *applyTestCase) makeData(t *testing.T) {
	tc.chain = defaultFakeBlockChain()
	if tc.snapshot != nil {
		tc.chain.snapshots[tc.snapshot.Address] = *tc.snapshot
	}

	tc.state = makeTestStateDB()
	if tc.current != nil {
		if err := tc.state.UpdateValidatorWrapper(tc.current.Address, tc.current); err != nil {
			t.Error(err)
		}
		if _, err := tc.state.Commit(true); err != nil {
			t.Error(err)
		}
	}
	tc.stateSnap = tc.state.Copy()
}

func (tc *applyTestCase) apply() {
	tc.gotDiff, tc.gotErr = Apply(tc.chain, tc.state, tc.slashes, leaderAddr)
}

func (tc *applyTestCase) checkResult() error {
	if err := assertError(tc.gotErr, tc.expErr); err != nil {
		return fmt.Errorf("unexpected error %v / %v", tc.gotErr, tc.expErr)
	}
	if (tc.gotErr != nil) || (tc.expErr != nil) {
		return nil
	}
	if tc.gotDiff.TotalSlashed.Cmp(tc.expSlashed) != 0 {
		return fmt.Errorf("unexpected total slash %v / %v", tc.gotDiff.TotalSlashed,
			tc.expSlashed)
	}
	if tc.gotDiff.TotalBeneficiaryReward.Cmp(tc.expBeneficiaryReward) != 0 {
		return fmt.Errorf("unexpected beneficiary reward %v / %v", tc.gotDiff.TotalBeneficiaryReward,
			tc.expBeneficiaryReward)
	}
	if err := tc.checkState(); err != nil {
		return fmt.Errorf("state check: %v", err)
	}
	return nil
}

// checkState checks whether the state has been banned and whether the delegations
// are different from the original
func (tc *applyTestCase) checkState() error {
	vw, err := tc.state.ValidatorWrapper(offAddr, true, false)
	if err != nil {
		return err
	}
	if !IsBanned(vw) {
		return fmt.Errorf("status not banned")
	}

	vwSnap, err := tc.stateSnap.ValidatorWrapper(offAddr, true, false)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(vwSnap.Delegations, vw.Delegations) {
		return fmt.Errorf("status still unchanged")
	}
	return nil
}

func defaultSlashRecord() Record {
	return Record{
		Evidence: Evidence{
			ConflictingVotes: ConflictingVotes{
				FirstVote:  makeVoteData(offKey, doubleSignBlock1),
				SecondVote: makeVoteData(offKey, doubleSignBlock2),
			},
			Moment: Moment{
				Epoch:   big.NewInt(doubleSignEpoch),
				ShardID: doubleSignShardID,
				Height:  doubleSignBlockNumber,
				ViewID:  doubleSignViewID,
			},
			Offender: offAddr,
		},
		Reporter: reporterAddr,
	}
}

func makeVoteData(kp blsKeyPair, block *types.Block) Vote {
	return Vote{
		SignerPubKeys:   []bls.SerializedPublicKey{kp.Pub()},
		BlockHeaderHash: block.Hash(),
		Signature:       kp.Sign(block),
	}
}

func makeTestAddress(item interface{}) common.Address {
	s := fmt.Sprintf("harmony.one.%v", item)
	return common.BytesToAddress([]byte(s))
}

func makeBlockForTest(epoch int64, index int) *types.Block {
	h := blockfactory.NewTestHeader()

	h.SetEpoch(big.NewInt(epoch))
	h.SetNumber(big.NewInt(doubleSignBlockNumber))
	h.SetViewID(big.NewInt(doubleSignViewID))
	h.SetRoot(common.BigToHash(big.NewInt(int64(index))))

	return types.NewBlockWithHeader(h)
}

func defaultValidatorWrapper() *staking.ValidatorWrapper {
	pubKeys := []bls.SerializedPublicKey{offPub}
	v := defaultTestValidator(pubKeys)
	ds := defaultTestDelegations()

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

func defaultSnapValidatorWrapper() *staking.ValidatorWrapper {
	return defaultValidatorWrapper()
}

func defaultCurrentValidatorWrapper() *staking.ValidatorWrapper {
	pubKeys := []bls.SerializedPublicKey{offPub}
	v := defaultTestValidator(pubKeys)
	ds := defaultDelegationsWithUndelegates()

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

// defaultTestValidator makes a valid Validator kps structure
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

func generateValidatorWrapper(delData []testDelegation) *staking.ValidatorWrapper {
	pubKeys := []bls.SerializedPublicKey{offPub}
	v := defaultTestValidator(pubKeys)
	ds := generateDelegations(delData)

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

func generateDelegations(delData []testDelegation) staking.Delegations {
	delegations := make(staking.Delegations, len(delData))
	for i, del := range delData {
		delegations[i] = makeDelegation(makeTestAddress(del.address), new(big.Int).Set(del.amount))

		if del.historyUndel != nil {
			delegations[i].Undelegations = append(
				delegations[i].Undelegations,
				staking.Undelegation{
					Amount: new(big.Int).Set(del.historyUndel),
					Epoch:  big.NewInt(doubleSignEpoch - 1),
				},
			)
		}
		if del.afterSignUndel != nil {
			delegations[i].Undelegations = append(
				delegations[i].Undelegations,
				staking.Undelegation{
					Amount: new(big.Int).Set(del.afterSignUndel),
					Epoch:  big.NewInt(doubleSignEpoch + 1),
				},
			)
		}
	}

	return delegations
}

func defaultTestDelegations() staking.Delegations {
	return staking.Delegations{
		makeDelegation(offAddr, new(big.Int).Set(fourtyKOnes)),
		makeDelegation(makeTestAddress("del1"), new(big.Int).Set(fourtyKOnes)),
	}
}

func defaultDelegationsWithUndelegates() staking.Delegations {
	d1 := makeDelegation(offAddr, new(big.Int).Set(twentyKOnes))
	d1.Undelegations = []staking.Undelegation{
		makeHistoryUndelegation(),
		makeDefaultUndelegation(),
	}
	d2 := makeDelegation(makeTestAddress("del2"), new(big.Int).Set(fourtyKOnes))

	return staking.Delegations{d1, d2}
}

func makeDelegation(addr common.Address, amount *big.Int) staking.Delegation {
	return staking.Delegation{
		DelegatorAddress: addr,
		Amount:           amount,
		Reward:           new(big.Int).Set(tenKOnes),
	}
}

func makeDefaultUndelegation() staking.Undelegation {
	return staking.Undelegation{
		Amount: new(big.Int).Set(tenKOnes),
		Epoch:  big.NewInt(doubleSignEpoch + 2),
	}
}

func makeHistoryUndelegation() staking.Undelegation {
	return staking.Undelegation{
		Amount: new(big.Int).Set(tenKOnes),
		Epoch:  big.NewInt(doubleSignEpoch - 1),
	}
}

// makeCommitteeFromKeyPairs makes a shard state for testing.
// address is generated by makeTestAddress
// bls key is get from the variable keyPairs []blsKeyPair
func makeDefaultCommittee() shard.State {
	epoch := big.NewInt(doubleSignEpoch)
	maker := newShardSlotMaker(keyPairs)
	return makeCommitteeBySlotMaker(epoch, maker)
}

func makeCommitteeBySlotMaker(epoch *big.Int, maker shardSlotMaker) shard.State {
	sstate := shard.State{
		Epoch:  epoch,
		Shards: make([]shard.Committee, 0, int(numShard)),
	}
	for sid := uint32(0); sid != numNodePerShard; sid++ {
		sstate.Shards = append(sstate.Shards, makeShardBySlotMaker(sid, maker))
	}
	return sstate
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

type shardSlotMaker struct {
	kps []blsKeyPair
	i   int
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
	msg := consensus_sig.ConstructCommitPayload(params.LocalnetChainConfig, block.Epoch(), block.Hash(),
		block.Number().Uint64(), block.Header().ViewID().Uint64())

	sig := kp.pri.SignHash(msg)

	return sig.Serialize()
}

func defaultTestStateDB() *state.DB {
	sdb := makeTestStateDB()
	err := sdb.UpdateValidatorWrapper(offAddr, defaultCurrentValidatorWrapper())
	if err != nil {
		panic(err)
	}
	return sdb
}

func makeTestStateDB() *state.DB {
	db := state.NewDatabase(rawdb.NewMemoryDatabase())
	sdb, err := state.New(common.Hash{}, db, nil)
	if err != nil {
		panic(err)
	}
	return sdb
}

func defaultFakeBlockChain() *fakeBlockChain {
	return &fakeBlockChain{
		config:         *params.LocalnetChainConfig,
		currentBlock:   *makeBlockForTest(currentEpoch, 0),
		superCommittee: makeDefaultCommittee(),
		snapshots:      make(map[common.Address]staking.ValidatorWrapper),
	}
}

func assertError(got, exp error) error {
	if (got == nil) != (exp == nil) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, exp)
	}
	if got == nil {
		return nil
	}
	if !strings.Contains(got.Error(), exp.Error()) {
		return fmt.Errorf("unexpected error [%v] / [%v]", got, exp)
	}
	return nil
}

// copyRecord makes a deep copy the slash Record
func copyRecord(r Record) Record {
	return Record{
		Evidence: copyEvidence(r.Evidence),
		Reporter: r.Reporter,
	}
}

// copyEvidence makes a deep copy the slash evidence
func copyEvidence(e Evidence) Evidence {
	return Evidence{
		Moment:           copyMoment(e.Moment),
		ConflictingVotes: copyConflictingVotes(e.ConflictingVotes),
		Offender:         e.Offender,
	}
}

// copyMoment makes a deep copy of the Moment structure
func copyMoment(m Moment) Moment {
	cp := Moment{
		ShardID: m.ShardID,
		Height:  m.Height,
		ViewID:  m.ViewID,
	}
	if m.Epoch != nil {
		cp.Epoch = new(big.Int).Set(m.Epoch)
	}
	return cp
}

// copyConflictingVotes makes a deep copy of slash.ConflictingVotes
func copyConflictingVotes(cv ConflictingVotes) ConflictingVotes {
	return ConflictingVotes{
		FirstVote:  copyVote(cv.FirstVote),
		SecondVote: copyVote(cv.SecondVote),
	}
}

// copyVote makes a deep copy of slash.Vote
func copyVote(v Vote) Vote {
	cp := Vote{
		SignerPubKeys:   v.SignerPubKeys,
		BlockHeaderHash: v.BlockHeaderHash,
	}
	if v.Signature != nil {
		cp.Signature = make([]byte, len(v.Signature))
		copy(cp.Signature, v.Signature)
	}
	return cp
}
