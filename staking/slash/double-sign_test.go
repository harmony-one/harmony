package slash

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/core/types"
	bls2 "github.com/harmony-one/harmony/crypto/bls"
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
	hundredKOnes    = new(big.Int).Mul(big.NewInt(1000000), bigOne)
)

const (
	// validator creation parameters
	doubleSignShardID     = 0
	doubleSignEpoch       = 3
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

	reporterAddr = makeTestAddress("reporter")
)

func TestVerify(t *testing.T) {
	tests := []struct {
		editInput func(chain *fakeBlockChain, db *fakeStateDB, r *Record)
		expErr    error
	}{
		{
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {},
			expErr:    nil,
		},
		{
			// not vWrapper in state
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				delete(db.vWrappers, offAddr)
			},
			expErr: errors.New("address vWrapper not exist"),
		},
		{
			// banned status
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				db.vWrappers[offAddr].Status = effective.Banned
			},
			expErr: errAlreadyBannedValidator,
		},
		{
			// same offender and reporter
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Reporter = offAddr
			},
			expErr: errReporterAndOffenderSame,
		},
		{
			// same block in conflicting votes
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.SecondVote = r.Evidence.FirstVote.Copy()
			},
			expErr: errSlashBlockNoConflict,
		},
		{
			// second vote signed with a different bls key
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				secondVote := makeVoteData(keyPairs[24], doubleSignBlock2)
				r.Evidence.SecondVote = secondVote
			},
			expErr: errBallotSignerKeysNotSame,
		},
		{
			// block is in the future
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				chain.currentBlock = *makeBlockForTest(doubleSignEpoch-1, 0)
			},
			expErr: errSlashFromFutureEpoch,
		},
		{
			// error from blockchain.ReadShardState (fakeChainErrEpoch)
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.Epoch = big.NewInt(fakeChainErrEpoch)
			},
			expErr: errFakeChainUnexpectEpoch,
		},
		{
			// Invalid shardID
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.ShardID = 10
			},
			expErr: shard.ErrShardIDNotInSuperCommittee,
		},
		{
			// Missing bls from committee
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.FirstVote = makeVoteData(keyPairs[24], doubleSignBlock1)
				r.Evidence.SecondVote = makeVoteData(keyPairs[24], doubleSignBlock2)
			},
			expErr: shard.ErrValidNotInCommittee,
		},
		{
			// offender address not match vote
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				chain.superCommittee.Shards[offenderShard].Slots[offenderShardIndex].
					EcdsaAddress = makeTestAddress("other")
			},
			expErr: errors.New("does not match the signer's address"),
		},
		{
			// empty signature
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.FirstVote.Signature = nil
			},
			expErr: errors.New("Empty buf"),
		},
		{
			// false signature
			editInput: func(chain *fakeBlockChain, db *fakeStateDB, r *Record) {
				r.Evidence.FirstVote.Signature = r.Evidence.SecondVote.Signature
			},
			expErr: errors.New("could not verify bls key signature on slash"),
		},
	}
	for i, test := range tests {
		bc, sdb := defaultBlockChainAndState()
		r := defaultSlashRecord()
		if test.editInput != nil {
			test.editInput(bc, sdb, &r)
		}
		rawState, rawRecord := sdb.copy(), r.Copy()

		err := Verify(bc, sdb, &r)

		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if !reflect.DeepEqual(r, rawRecord) {
			t.Errorf("Test %v: record has value changed", i)
		}
		if err := sdb.assertEqual(rawState); err != nil {
			t.Errorf("Test %v: state changed: %v", i, err)
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

func TestPayDownAsMuchAsCan(t *testing.T) {
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
				TotalSlashed:      new(big.Int).Set(tenKOnes),
				TotalSnitchReward: new(big.Int).Set(tenKOnes),
			},
			expDebt: common.Big0,
			expAmt:  fiveKOnes,
			expDiff: &Application{
				TotalSlashed:      thirtyFiveKOnes,
				TotalSnitchReward: tenKOnes,
			},
		},
		{
			debt: new(big.Int).Set(thirtyKOnes),
			amt:  new(big.Int).Set(twentyFiveKOnes),
			diff: &Application{
				TotalSlashed:      new(big.Int).Set(tenKOnes),
				TotalSnitchReward: new(big.Int).Set(tenKOnes),
			},
			expDebt: fiveKOnes,
			expAmt:  common.Big0,
			expDiff: &Application{
				TotalSlashed:      thirtyFiveKOnes,
				TotalSnitchReward: tenKOnes,
			},
		},
	}
	for i, test := range tests {
		vwSnap := defaultValidatorWrapper()
		vwCur := defaultCurrentValidatorWrapper()

		err := payDownAsMuchAsCan(vwSnap, vwCur, test.debt, test.amt, test.diff)
		if assErr := assertError(err, test.expErr); assErr != nil {
			t.Errorf("Test %v: %v", i, assErr)
		}
		if err != nil || test.expErr != nil {
			continue
		}

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
		if test.diff.TotalSnitchReward.Cmp(test.expDiff.TotalSnitchReward) != 0 {
			t.Errorf("Test %v: unexpected totalSnitchReward %v / %v", i,
				test.diff.TotalSnitchReward, test.expDiff.TotalSnitchReward)
		}
	}
}

func TestDelegatorSlashApply(t *testing.T) {
	tests := []slashApplyTestCase{
		{
			rate:     numeric.ZeroDec(),
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			expDels: []expDelegation{
				{
					expAmt:      twentyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{tenKOnes, tenKOnes},
				},
				{
					expAmt:      fourtyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed: common.Big0,
			expSnitch:  common.Big0,
		},
		{
			rate:     numeric.NewDecWithPrec(25, 2),
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			expDels: []expDelegation{
				{
					expAmt:      tenKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{tenKOnes, tenKOnes},
				},
				{
					expAmt:      fourtyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed: tenKOnes,
			expSnitch:  fiveKOnes,
		},
		{
			rate:     numeric.NewDecWithPrec(625, 3),
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			expDels: []expDelegation{
				{
					expAmt:      common.Big0,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{tenKOnes, fiveKOnes},
				},
				{
					expAmt:      fourtyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed: twentyFiveKOnes,
			expSnitch:  new(big.Int).Div(twentyFiveKOnes, common.Big2),
		},
		{
			rate:     numeric.NewDecWithPrec(875, 3),
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			expDels: []expDelegation{
				{
					expAmt:      common.Big0,
					expReward:   fiveKOnes,
					expUndelAmt: []*big.Int{tenKOnes, common.Big0},
				},
				{
					expAmt:      fourtyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed: thirtyFiveKOnes,
			expSnitch:  new(big.Int).Div(thirtyFiveKOnes, common.Big2),
		},
		{
			rate:     numeric.NewDecWithPrec(150, 2),
			snapshot: defaultSnapValidatorWrapper(),
			current:  defaultCurrentValidatorWrapper(),
			expDels: []expDelegation{
				{
					expAmt:      common.Big0,
					expReward:   common.Big0,
					expUndelAmt: []*big.Int{tenKOnes, common.Big0},
				},
				{
					expAmt:      fourtyKOnes,
					expReward:   tenKOnes,
					expUndelAmt: []*big.Int{},
				},
			},
			expSlashed: fourtyKOnes,
			expSnitch:  twentyKOnes,
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

type slashApplyTestCase struct {
	snapshot, current *staking.ValidatorWrapper
	rate              numeric.Dec

	reporter   common.Address
	state      *fakeStateDB
	slashTrack *Application
	gotErr     error

	expDels               []expDelegation
	expSlashed, expSnitch *big.Int
	expErr                error
}

func (tc *slashApplyTestCase) makeData() {
	tc.reporter = reporterAddr
	tc.state = newFakeStateDB()
	tc.slashTrack = &Application{
		TotalSlashed:      new(big.Int).Set(common.Big0),
		TotalSnitchReward: new(big.Int).Set(common.Big0),
	}
}

func (tc *slashApplyTestCase) apply() {
	tc.gotErr = delegatorSlashApply(tc.snapshot, tc.current, tc.rate, tc.state, tc.reporter,
		big.NewInt(doubleSignEpoch), tc.slashTrack)
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
	if tc.slashTrack.TotalSnitchReward.Cmp(tc.expSnitch) != 0 {
		return fmt.Errorf("unexpected snitch reward %v / %v", tc.slashTrack.TotalSnitchReward,
			tc.expSnitch)
	}
	if err := tc.state.assertBalance(tc.reporter, tc.expSnitch); err != nil {
		return fmt.Errorf("state: %v", err)
	}
	return nil
}

type expDelegation struct {
	expAmt, expReward *big.Int
	expUndelAmt       []*big.Int
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
			rate:     numeric.NewDecWithPrec(625, 3),

			expSlashed: twentyFiveKOnes,
			expSnitch:  new(big.Int).Div(twentyFiveKOnes, common.Big2),
		},
		{
			// missing snapshot in chain
			current: defaultCurrentValidatorWrapper(),
			slashes: Records{defaultSlashRecord()},
			rate:    numeric.NewDecWithPrec(625, 3),

			expErr: errors.New("could not find validator"),
		},
		{
			// missing vWrapper in state
			snapshot: defaultSnapValidatorWrapper(),
			slashes:  Records{defaultSlashRecord()},
			rate:     numeric.NewDecWithPrec(625, 3),

			expErr: errValidatorNotFoundDuringSlash,
		},
	}
	for i, test := range tests {
		test.makeData()

		test.apply()

		if err := test.checkResult(); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

type applyTestCase struct {
	snapshot, current *staking.ValidatorWrapper
	slashes           Records
	rate              numeric.Dec

	chain            *fakeBlockChain
	state, stateSnap *fakeStateDB
	gotErr           error
	gotDiff          *Application

	expSlashed *big.Int
	expSnitch  *big.Int
	expErr     error
}

func (tc *applyTestCase) makeData() {
	tc.chain = defaultFakeBlockChain()
	if tc.snapshot != nil {
		tc.chain.snapshots[tc.snapshot.Address] = *tc.snapshot
	}

	tc.state = newFakeStateDB()
	if tc.current != nil {
		tc.state.vWrappers[tc.current.Address] = tc.current
	}
	tc.stateSnap = tc.state.copy()
	return
}

func (tc *applyTestCase) apply() {
	tc.gotDiff, tc.gotErr = Apply(tc.chain, tc.state, tc.slashes, tc.rate)
}

func (tc *applyTestCase) checkResult() error {
	if err := assertError(tc.gotErr, tc.expErr); err != nil {
		return fmt.Errorf("unexpected error %v / %v", tc.gotErr, tc.expErr)
	}
	if (tc.gotErr != nil) || (tc.expErr != nil) {
		return nil
	}
	if tc.gotDiff.TotalSnitchReward.Cmp(tc.expSnitch) != 0 {
		return fmt.Errorf("unexpected snitch %v / %v", tc.gotDiff.TotalSnitchReward,
			tc.expSnitch)
	}
	if tc.gotDiff.TotalSlashed.Cmp(tc.expSlashed) != 0 {
		return fmt.Errorf("unexpected total slash %v / %v", tc.gotDiff.TotalSlashed,
			tc.expSlashed)
	}
	if err := tc.checkState(); err != nil {
		return fmt.Errorf("state check: %v", err)
	}
	return nil
}

// checkState checks whether the state has been banned and whether the delegations
// are different from the original
func (tc *applyTestCase) checkState() error {
	vw, err := tc.state.ValidatorWrapper(offAddr)
	if err != nil {
		return err
	}
	if vw.Status != effective.Banned {
		return fmt.Errorf("status not banned")
	}

	vwSnap, err := tc.stateSnap.ValidatorWrapper(offAddr)
	if err != nil {
		return err
	}
	if tc.rate != numeric.ZeroDec() && reflect.DeepEqual(vwSnap.Delegations, vw.Delegations) {
		return fmt.Errorf("status still unchanged")
	}
	return nil
}

func TestRecord_Copy(t *testing.T) {
	tests := []struct {
		r Record
	}{
		{
			r: defaultSlashRecord(),
		},
		{
			// Zero values
			r: Record{
				Evidence: Evidence{
					Moment: Moment{Epoch: common.Big0},
					ConflictingVotes: ConflictingVotes{
						FirstVote:  Vote{Signature: make([]byte, 0)},
						SecondVote: Vote{Signature: make([]byte, 0)},
					},
				},
			},
		},
		{
			// Empty values
			r: Record{},
		},
	}
	for i, test := range tests {
		cp := test.r.Copy()

		if err := assertRecordDeepCopy(cp, test.r); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

func assertRecordDeepCopy(r1, r2 Record) error {
	if !reflect.DeepEqual(r1, r2) {
		return fmt.Errorf("not deep equal")
	}
	if r1.Evidence.Epoch != nil && r1.Evidence.Epoch == r2.Evidence.Epoch {
		return fmt.Errorf("epoch not deep copy")
	}
	if err := assertVoteDeepCopy(r1.Evidence.FirstVote, r2.Evidence.FirstVote); err != nil {
		return fmt.Errorf("FirstVote: %v", err)
	}
	if err := assertVoteDeepCopy(r1.Evidence.SecondVote, r2.Evidence.SecondVote); err != nil {
		return fmt.Errorf("SecondVote: %v", err)
	}
	return nil
}

func assertVoteDeepCopy(v1, v2 Vote) error {
	if !reflect.DeepEqual(v1, v2) {
		return fmt.Errorf("not deep equal")
	}
	if len(v1.Signature) != 0 && &v1.Signature[0] == &v2.Signature[0] {
		return fmt.Errorf("signature same pointer")
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
		SignerPubKey:    kp.Pub(),
		BlockHeaderHash: block.Hash(),
		Signature:       kp.Sign(block),
	}
}

func makeTestAddress(item interface{}) common.Address {
	s := fmt.Sprintf("harmony.one.%s", item)
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
	pubKeys := []shard.BLSPublicKey{offPub}
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
	pubKeys := []shard.BLSPublicKey{offPub}
	v := defaultTestValidator(pubKeys)
	ds := defaultDelegationsWithUndelegates()

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

// defaultTestValidator makes a valid Validator kps structure
func defaultTestValidator(pubKeys []shard.BLSPublicKey) staking.Validator {
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
		MaxTotalDelegation:   new(big.Int).Set(hundredKOnes),
		Status:               effective.Active,
		Commission:           comm,
		Description:          desc,
		CreationHeight:       big.NewInt(creationHeight),
	}
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
//  address is generated by makeTestAddress
//  bls key is get from the variable keyPairs []blsKeyPair
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
	pri *bls.SecretKey
	pub *bls.PublicKey
}

func genKeyPairs(size int) []blsKeyPair {
	kps := make([]blsKeyPair, 0, size)
	for i := 0; i != size; i++ {
		kps = append(kps, genKeyPair())
	}
	return kps
}

func genKeyPair() blsKeyPair {
	pri := bls2.RandPrivateKey()
	pub := pri.GetPublicKey()
	return blsKeyPair{
		pri: pri,
		pub: pub,
	}
}

func (kp blsKeyPair) Pub() shard.BLSPublicKey {
	var pub shard.BLSPublicKey
	copy(pub[:], kp.pub.Serialize())
	return pub
}

func (kp blsKeyPair) Sign(block *types.Block) []byte {
	chain := &fakeBlockChain{config: *params.LocalnetChainConfig}
	msg := consensus_sig.ConstructCommitPayload(chain, block.Epoch(), block.Hash(),
		block.Number().Uint64(), block.Header().ViewID().Uint64())

	sig := kp.pri.SignHash(msg)

	return sig.Serialize()
}

func defaultBlockChainAndState() (*fakeBlockChain, *fakeStateDB) {
	fbc := defaultFakeBlockChain()
	sdb := defaultFakeStateDB()
	return fbc, sdb
}

func defaultFakeStateDB() *fakeStateDB {
	sdb := &fakeStateDB{
		balances:  make(map[common.Address]*big.Int),
		vWrappers: make(map[common.Address]*staking.ValidatorWrapper),
	}
	sdb.vWrappers[offAddr] = defaultCurrentValidatorWrapper()
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

// Simply testing serialization / deserialization of slash records working correctly
//func TestRoundTripSlashRecord(t *testing.T) {
//	slashes := exampleSlashRecords()
//	serializedA := slashes.String()
//	data, err := rlp.EncodeToBytes(slashes)
//	if err != nil {
//		t.Errorf("encoding slash records failed %s", err.Error())
//	}
//	roundTrip := Records{}
//	if err := rlp.DecodeBytes(data, &roundTrip); err != nil {
//		t.Errorf("decoding slash records failed %s", err.Error())
//	}
//	serializedB := roundTrip.String()
//	if serializedA != serializedB {
//		t.Error("rlp encode/decode round trip records failed")
//	}
//}
//
//func TestSetDifference(t *testing.T) {
//	setA, setB := exampleSlashRecords(), exampleSlashRecords()
//	additionalSlash := defaultSlashRecord()
//	additionalSlash.Evidence.Epoch.Add(additionalSlash.Evidence.Epoch, common.Big1)
//	setB = append(setB, additionalSlash)
//	diff := setA.SetDifference(setB)
//	if diff[0].Hash() != additionalSlash.Hash() {
//		t.Errorf("did not get set difference of slash")
//	}
//}

// TODO bytes used for this example are stale, need to update RLP dump
// func TestApply(t *testing.T) {
// 	slashes := exampleSlashRecords()
// {
// 	stateHandle := defaultStateWithAccountsApplied()
// 	testScenario(t, stateHandle, slashes, scenarioRealWorldSample1())
// }
// }
//
//func TestVerify(t *testing.T) {
//	stateHandle := defaultStateWithAccountsApplied()
//	// TODO: test this
//}

//func TestTwoPercentSlashed(t *testing.T) {
//	slashes := exampleSlashRecords()
//	stateHandle := defaultStateWithAccountsApplied()
//	testScenario(t, stateHandle, slashes, scenarioTwoPercent)
//}
//
//func setupScenario(){
//	{
//		s := scenarioTwoPercent
//		s.slashRate = 0.02
//		s.result = &Application{
//			TotalSlashed:      totalSlashedExpected(s.slashRate),      // big.NewInt(int64(s.slashRate * 5.0 * denominations.One)),
//			TotalSnitchReward: totalSnitchRewardExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 2.5 * denominations.One)),
//		}
//		s.snapshot = defaultValidatorWrapper([]shard.BLSPublicKey{offPub})
//		s.current = defaultCurrentValidatorWrapper([]shard.BLSPublicKey{offPub})
//	}
//	{
//		s := scenarioEightyPercent
//		s.slashRate = 0.80
//		s.result = &Application{
//			TotalSlashed:      totalSlashedExpected(s.slashRate),      // big.NewInt(int64(s.slashRate * 5.0 * denominations.One)),
//			TotalSnitchReward: totalSnitchRewardExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 2.5 * denominations.One)),
//		}
//		s.snapshot = defaultValidatorWrapper([]shard.BLSPublicKey{offPub})
//		s.current = defaultCurrentValidatorWrapper([]shard.BLSPublicKey{offPub})
//	}
//}
//
//type scenario struct {
//	snapshot, current *staking.ValidatorWrapper
//	slashRate         float64
//	result            *Application
//}
//
//func defaultFundingScenario() *scenario {
//	return &scenario{
//		snapshot:  nil,
//		current:   nil,
//		slashRate: 0.02,
//		result:    nil,
//	}
//}
//// func TestEightyPercentSlashed(t *testing.T) {
//// 	slashes := exampleSlashRecords()
//// 	stateHandle := defaultStateWithAccountsApplied()
//// 	testScenario(t, stateHandle, slashes, scenarioEightyPercent)
//// }
//
//func TestDoubleSignSlashRates(t *testing.T) {
//	for _, scenario := range doubleSignScenarios {
//		slashes := exampleSlashRecords()
//		stateHandle := defaultStateWithAccountsApplied()
//		testScenario(t, stateHandle, slashes, scenario)
//	}
//}
//var (
//	scenarioTwoPercent    = defaultFundingScenario()
//	scenarioEightyPercent = defaultFundingScenario()
//)
//func testScenario(
//	t *testing.T, stateHandle *state.DB, slashes Records, s *scenario,
//) {
//	if err := stateHandle.UpdateValidatorWrapper(
//		offAddr, s.snapshot,
//	); err != nil {
//		t.Fatalf("creation of validator failed %s", err.Error())
//	}
//
//	stateHandle.IntermediateRoot(false)
//	stateHandle.Commit(false)
//
//	if err := stateHandle.UpdateValidatorWrapper(
//		offAddr, s.current,
//	); err != nil {
//		t.Fatalf("update of validator failed %s", err.Error())
//	}
//
//	stateHandle.IntermediateRoot(false)
//	stateHandle.Commit(false)
//	// NOTE See dump.json to see what account
//	// state looks like as of this point
//
//	slashResult, err := Apply(
//		mockOutSnapshotReader{staking.ValidatorSnapshot{s.snapshot, big.NewInt(0)}},
//		stateHandle,
//		slashes,
//		numeric.MustNewDecFromStr(
//			fmt.Sprintf("%f", s.slashRate),
//		),
//	)
//
//	if err != nil {
//		t.Fatalf("rate: %v, slash application failed %s", s.slashRate, err.Error())
//	}
//
//	if sn := slashResult.TotalSlashed; sn.Cmp(
//		s.result.TotalSlashed,
//	) != 0 {
//		t.Errorf(
//			"total slash incorrect have %v want %v",
//			sn,
//			s.result.TotalSlashed,
//		)
//	}
//
//	if sn := slashResult.TotalSnitchReward; sn.Cmp(
//		s.result.TotalSnitchReward,
//	) != 0 {
//		t.Errorf(
//			"total snitch incorrect have %v want %v",
//			sn,
//			s.result.TotalSnitchReward,
//		)
//	}
//}
