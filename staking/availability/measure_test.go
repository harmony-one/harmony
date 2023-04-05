package availability

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

func TestBlockSigners(t *testing.T) {
	tests := []struct {
		numSlots               int
		verified               []int
		numPayable, numMissing int
	}{
		{0, []int{}, 0, 0},
		{1, []int{}, 0, 1},
		{1, []int{0}, 1, 0},
		{8, []int{}, 0, 8},
		{8, []int{0}, 1, 7},
		{8, []int{7}, 1, 7},
		{8, []int{1, 3, 5, 7}, 4, 4},
		{8, []int{0, 2, 4, 6}, 4, 4},
		{8, []int{0, 1, 2, 3, 4, 5, 6, 7}, 8, 0},
		{13, []int{0, 1, 4, 5, 6, 9, 12}, 7, 6},
		// TODO: add a real data test case given numSlots of a committee and
		//  number of payable of a certain block
	}
	for i, test := range tests {
		cmt := makeTestCommittee(test.numSlots, 0)
		bm, err := indexesToBitMap(test.verified, test.numSlots)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		pSlots, mSlots, err := BlockSigners(bm, cmt)
		if err != nil {
			t.Fatalf("test %d: %v", i, err)
		}
		if len(pSlots) != test.numPayable || len(mSlots) != test.numMissing {
			t.Errorf("test %d: unexpected result: # pSlots %d/%d, # mSlots %d/%d",
				i, len(pSlots), test.numPayable, len(mSlots), test.numMissing)
			continue
		}
		if err := checkPayableAndMissing(cmt, test.verified, pSlots, mSlots); err != nil {
			t.Errorf("test %d: %v", i, err)
		}
	}
}

func checkPayableAndMissing(cmt *shard.Committee, idxs []int, pSlots, mSlots shard.SlotList) error {
	if len(pSlots)+len(mSlots) != len(cmt.Slots) {
		return fmt.Errorf("slots number not expected: %d(payable) + %d(missings) != %d(committee)",
			len(pSlots), len(mSlots), len(cmt.Slots))
	}
	pIndex, mIndex, iIndex := 0, 0, 0
	for i, slot := range cmt.Slots {
		if iIndex >= len(idxs) || i != idxs[iIndex] {
			// the slot should be missings and we shall check mSlots[mIndex] == slot
			if mIndex >= len(mSlots) || !reflect.DeepEqual(slot, mSlots[mIndex]) {
				return fmt.Errorf("addr %v missed from missings slots", slot.EcdsaAddress.String())
			}
			mIndex++
		} else {
			// check pSlots[pIndex] == slot
			if pIndex >= len(pSlots) || !reflect.DeepEqual(slot, pSlots[pIndex]) {
				return fmt.Errorf("addr %v missed from payable slots", slot.EcdsaAddress.String())
			}
			pIndex++
			iIndex++
		}
	}
	return nil
}

func TestBlockSigners_BitmapOverflow(t *testing.T) {
	tests := []struct {
		numSlots  int
		numBitmap int
		err       error
	}{
		{16, 16, nil},
		{16, 14, nil},
		{16, 8, errors.New("bitmap size too small")},
		{16, 24, errors.New("bitmap size too large")},
	}
	for i, test := range tests {
		cmt := makeTestCommittee(test.numSlots, 0)
		bm, _ := indexesToBitMap([]int{}, test.numBitmap)
		_, _, err := BlockSigners(bm, cmt)
		if (err == nil) != (test.err == nil) {
			t.Errorf("Test %d: BlockSigners got err [%v], expect [%v]", i, err, test.err)
		}
	}
}

func TestBallotResult(t *testing.T) {
	tests := []struct {
		numStateShards, numShardSlots int
		parVerified, chdVerified      int
		parShardID, chdShardID        uint32
		parBN, chdBN                  int64

		expNumPayable, expNumMissing int
		expErr                       error
	}{
		{1, 1, 1, 1, 0, 0, 10, 11, 1, 0, nil},
		{5, 16, 10, 12, 3, 4, 100, 101, 12, 4, nil},
		{5, 16, 10, 12, 5, 6, 100, 101, 12, 4, errors.New("cannot find shard")},
	}
	for i, test := range tests {
		sstate := makeTestShardState(test.numStateShards, test.numShardSlots)
		parHeader := newTestHeader(test.parBN, test.parShardID, test.numShardSlots, test.parVerified)
		chdHeader := newTestHeader(test.chdBN, test.chdShardID, test.numShardSlots, test.chdVerified)

		slots, payable, missing, err := BallotResult(parHeader, chdHeader, sstate, chdHeader.ShardID())
		if err != nil {
			if test.expErr == nil {
				t.Errorf("Test %v: unexpected error: %v", i, err)
			}
			continue
		}

		expCmt, _ := sstate.FindCommitteeByID(test.chdShardID)
		if !reflect.DeepEqual(slots, expCmt.Slots) {
			t.Errorf("Test %v: Ballot result slots not expected", i)
		}
		if len(payable) != test.expNumPayable {
			t.Errorf("Test %v: payable size not expected: %v / %v", i, len(payable), test.expNumPayable)
		}
		if len(missing) != test.expNumMissing {
			t.Errorf("Test %v: missings size not expected: %v / %v", i, len(missing), test.expNumMissing)
		}
	}
}

func TestIncrementValidatorSigningCounts(t *testing.T) {
	tests := []struct {
		numHmySlots, numUserSlots int
		verified                  []int
	}{
		{1, 0, []int{0}},
		{0, 1, []int{0}},
		{10, 6, []int{0, 2, 3, 4, 6, 8, 10, 12, 14}},
		{10, 6, []int{1, 3, 5, 7, 9, 11, 13, 15}},
	}
	for _, test := range tests {
		ctx, err := makeIncStateTestCtx(test.numHmySlots, test.numUserSlots, test.verified)
		if err != nil {
			t.Fatal(err)
		}
		if err := IncrementValidatorSigningCounts(nil, ctx.staked, ctx.state, ctx.signers,
			ctx.missings); err != nil {

			t.Fatal(err)
		}
		if err := ctx.checkResult(); err != nil {
			t.Error(err)
		}
	}
}

func TestComputeCurrentSigning(t *testing.T) {
	tests := []struct {
		snapSigned, curSigned, diffSigned int64
		snapToSign, curToSign, diffToSign int64
		pctNum, pctDiv                    int64
		isBelowThreshold                  bool
	}{
		{0, 0, 0, 0, 0, 0, 0, 1, true},
		{0, 1, 1, 0, 1, 1, 1, 1, false},
		{0, 2, 2, 0, 3, 3, 2, 3, true},
		{0, 1, 1, 0, 3, 3, 1, 3, true},
		{100, 225, 125, 200, 350, 150, 5, 6, false},
		{100, 200, 100, 200, 350, 150, 2, 3, true},
		{100, 200, 100, 200, 400, 200, 1, 2, true},
	}
	for i, test := range tests {
		snapWrapper := makeTestWrapper(common.Address{}, test.snapSigned, test.snapToSign)
		curWrapper := makeTestWrapper(common.Address{}, test.curSigned, test.curToSign)

		computed := ComputeCurrentSigning(&snapWrapper, &curWrapper)

		if computed.Signed.Cmp(new(big.Int).SetInt64(test.diffSigned)) != 0 {
			t.Errorf("test %v: computed signed not expected: %v / %v",
				i, computed.Signed, test.diffSigned)
		}
		if computed.ToSign.Cmp(new(big.Int).SetInt64(test.diffToSign)) != 0 {
			t.Errorf("test %v: computed to sign not expected: %v / %v",
				i, computed.ToSign, test.diffToSign)
		}
		expPct := numeric.NewDec(test.pctNum).Quo(numeric.NewDec(test.pctDiv))
		if !computed.Percentage.Equal(expPct) {
			t.Errorf("test %v: computed percentage not expected: %v / %v",
				i, computed.Percentage, expPct)
		}
		if computed.IsBelowThreshold != test.isBelowThreshold {
			t.Errorf("test %v: computed is below threshold not expected: %v / %v",
				i, computed.IsBelowThreshold, test.isBelowThreshold)
		}
	}
}

func TestComputeAndMutateEPOSStatus(t *testing.T) {
	tests := []struct {
		ctx       *computeEPOSTestCtx
		expErr    error
		expStatus effective.Eligibility
	}{
		// active node
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Active,
				curSigned:  200,
				curToSign:  200,
				curEli:     effective.Active,
			},
			expStatus: effective.Active,
		},
		// active -> inactive
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Active,
				curSigned:  200,
				curToSign:  250,
				curEli:     effective.Active,
			},
			expStatus: effective.Inactive,
		},
		// active -> inactive
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Active,
				curSigned:  100,
				curToSign:  200,
				curEli:     effective.Active,
			},
			expStatus: effective.Inactive,
		},
		// status unchanged: inactive -> inactive
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Inactive,
				curSigned:  200,
				curToSign:  200,
				curEli:     effective.Inactive,
			},
			expStatus: effective.Inactive,
		},
		// status unchanged: inactive
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Inactive,
				curSigned:  200,
				curToSign:  200,
				curEli:     effective.Active,
			},
			expStatus: effective.Active,
		},
		// nil validator wrapper in state
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 100,
				snapEli:    effective.Active,
				curEli:     effective.Nil,
			},
			expErr: errors.New("nil validator wrapper in state"),
		},
		// nil validator wrapper in snapshot
		{
			ctx: &computeEPOSTestCtx{
				addr:      common.Address{20, 20},
				snapEli:   effective.Nil,
				curSigned: 200,
				curToSign: 200,
				curEli:    effective.Active,
			},
			expErr: errors.New("nil validator wrapper in snapshot"),
		},
		// banned node
		{
			ctx: &computeEPOSTestCtx{
				addr:       common.Address{20, 20},
				snapSigned: 100,
				snapToSign: 200,
				snapEli:    effective.Active,
				curSigned:  100,
				curToSign:  200,
				curEli:     effective.Banned,
			},
			expStatus: effective.Banned,
		},
	}
	for i, test := range tests {
		ctx := test.ctx
		ctx.makeStateAndReader()

		err := ComputeAndMutateEPOSStatus(ctx.reader, ctx.state, ctx.addr)
		if err != nil {
			if test.expErr == nil {
				t.Errorf("Test %v: unexpected error: %v", i, err)
			}
			continue
		}

		if err := ctx.checkWrapperStatus(test.expStatus); err != nil {
			t.Errorf("Test %v: %v", i, err)
		}
	}
}

// incStateTestCtx is the helper structure for test case TestIncrementValidatorSigningCounts
type incStateTestCtx struct {
	// Initialized fields
	snapState, state  testStateDB
	cmt               *shard.Committee
	staked            *shard.StakedSlots
	signers, missings shard.SlotList

	// computedSlotMap is parsed map for result checking, which maps from Ecdsa address
	// to the expected behaviour of the address.
	//  typeIncSigned - 0: increase both toSign and signed
	//  typeIncMissing - 1: increase to sign
	//  typeIncHmyNode - 2: keep the code field unchanged
	computedSlotMap map[common.Address]int
}

const (
	typeIncSigned = iota
	typeIncMissing
	typeIncHmyNode
)

// makeIncStateTestCtx create and initialize the test context for TestIncrementValidatorSigningCounts
func makeIncStateTestCtx(numHmySlots, numUserSlots int, verified []int) (*incStateTestCtx, error) {
	cmt := makeTestMixedCommittee(numHmySlots, numUserSlots, 0)
	staked := cmt.StakedValidators()
	bitmap, _ := indexesToBitMap(verified, numUserSlots+numHmySlots)
	signers, missing, err := BlockSigners(bitmap, cmt)
	if err != nil {
		return nil, err
	}
	state := newTestStateDBFromCommittee(cmt)
	snapState := state.snapshot()

	return &incStateTestCtx{
		snapState: snapState,
		state:     state,
		cmt:       cmt,
		staked:    staked,
		signers:   signers,
		missings:  missing,
	}, nil
}

// checkResult checks the state change result for incStateTestCtx
func (ctx *incStateTestCtx) checkResult() error {
	ctx.computeSlotMaps()

	for addr, typeInc := range ctx.computedSlotMap {
		if err := ctx.checkAddrIncStateByType(addr, typeInc); err != nil {
			return err
		}
	}
	return nil
}

// computeSlotMaps compute for computedSlotMap for incStateTestCtx
func (ctx *incStateTestCtx) computeSlotMaps() {
	ctx.computedSlotMap = make(map[common.Address]int)

	for _, signer := range ctx.signers {
		ctx.computedSlotMap[signer.EcdsaAddress] = typeIncSigned
	}
	for _, missing := range ctx.missings {
		ctx.computedSlotMap[missing.EcdsaAddress] = typeIncMissing
	}
	for _, slot := range ctx.cmt.Slots {
		if slot.EffectiveStake == nil {
			ctx.computedSlotMap[slot.EcdsaAddress] = typeIncHmyNode
		}
	}
}

// checkAddrIncStateByType checks whether the state behaviour of a given address follows
// the expected state change rule given typeInc
func (ctx *incStateTestCtx) checkAddrIncStateByType(addr common.Address, typeInc int) error {
	var err error
	switch typeInc {
	case typeIncSigned:
		if err = ctx.checkWrapperChangeByAddr(addr, checkIncWrapperVerified); err != nil {
			err = fmt.Errorf("verified address %s: %v", addr, err)
		}
	case typeIncMissing:
		if err = ctx.checkWrapperChangeByAddr(addr, checkIncWrapperMissing); err != nil {
			err = fmt.Errorf("missing address %s: %v", addr, err)
		}
	case typeIncHmyNode:
		if err = ctx.checkHmyNodeStateChangeByAddr(addr); err != nil {
			err = fmt.Errorf("harmony node address %s: %v", addr, err)
		}
	default:
		err = errors.New("unknown typeInc")
	}
	return err
}

// checkHmyNodeStateChangeByAddr checks the state change for hmy nodes. Since hmy nodes does not
// have wrapper, it is supposed to be unchanged in code field
func (ctx *incStateTestCtx) checkHmyNodeStateChangeByAddr(addr common.Address) error {
	snapCode := ctx.snapState.GetCode(addr, false)
	curCode := ctx.state.GetCode(addr, false)
	if !reflect.DeepEqual(snapCode, curCode) {
		return errors.New("code not expected")
	}
	return nil
}

// checkWrapperChangeByAddr checks whether the wrapper of a given address
// before and after the state change is expected defined by compare function f.
func (ctx *incStateTestCtx) checkWrapperChangeByAddr(addr common.Address,
	f func(w1, w2 *staking.ValidatorWrapper) bool) error {

	snapWrapper, err := ctx.snapState.ValidatorWrapper(addr, true, false)
	if err != nil {
		return err
	}
	curWrapper, err := ctx.state.ValidatorWrapper(addr, true, false)
	if err != nil {
		return err
	}
	if isExpected := f(snapWrapper, curWrapper); !isExpected {
		return errors.New("validatorWrapper not expected")
	}
	return nil
}

// checkIncWrapperVerified is the compare function to check whether validator wrapper
// is expected for nodes who has verified a block.
func checkIncWrapperVerified(snapWrapper, curWrapper *staking.ValidatorWrapper) bool {
	snapSigned := snapWrapper.Counters.NumBlocksSigned
	curSigned := curWrapper.Counters.NumBlocksSigned
	if curSigned.Cmp(new(big.Int).Add(snapSigned, common.Big1)) != 0 {
		return false
	}
	snapToSign := snapWrapper.Counters.NumBlocksToSign
	curToSign := curWrapper.Counters.NumBlocksToSign
	return curToSign.Cmp(new(big.Int).Add(snapToSign, common.Big1)) == 0
}

// checkIncWrapperMissing is the compare function to check whether validator wrapper
// is expected for nodes who has missed a block.
func checkIncWrapperMissing(snapWrapper, curWrapper *staking.ValidatorWrapper) bool {
	snapSigned := snapWrapper.Counters.NumBlocksSigned
	curSigned := curWrapper.Counters.NumBlocksSigned
	if curSigned.Cmp(snapSigned) != 0 {
		return false
	}
	snapToSign := snapWrapper.Counters.NumBlocksToSign
	curToSign := curWrapper.Counters.NumBlocksToSign
	return curToSign.Cmp(new(big.Int).Add(snapToSign, common.Big1)) == 0
}

type computeEPOSTestCtx struct {
	// input arguments
	addr                   common.Address
	snapSigned, snapToSign int64
	snapEli                effective.Eligibility
	curSigned, curToSign   int64
	curEli                 effective.Eligibility

	// computed fields
	state  testStateDB
	reader testReader
}

// makeStateAndReader compute for state and reader given the input arguments
func (ctx *computeEPOSTestCtx) makeStateAndReader() {
	ctx.reader = newTestReader()
	if ctx.snapEli != effective.Nil {
		wrapper := makeTestWrapper(ctx.addr, ctx.snapSigned, ctx.snapToSign)
		wrapper.Status = ctx.snapEli
		ctx.reader.updateValidatorWrapper(ctx.addr, &wrapper)
	}
	ctx.state = newTestStateDB()
	if ctx.curEli != effective.Nil {
		wrapper := makeTestWrapper(ctx.addr, ctx.curSigned, ctx.curToSign)
		wrapper.Status = ctx.curEli
		ctx.state.UpdateValidatorWrapper(ctx.addr, &wrapper)
	}
}

func (ctx *computeEPOSTestCtx) checkWrapperStatus(expStatus effective.Eligibility) error {
	wrapper, err := ctx.state.ValidatorWrapper(ctx.addr, true, false)
	if err != nil {
		return err
	}
	if wrapper.Status != expStatus {
		return fmt.Errorf("wrapper status unexpected: %v / %v", wrapper.Status, expStatus)
	}
	return nil
}

// testHeader is the fake Header for testing
type testHeader struct {
	number           *big.Int
	shardID          uint32
	lastCommitBitmap []byte
}

func newTestHeader(number int64, shardID uint32, numSlots, numVerified int) *testHeader {
	indexes := make([]int, 0, numVerified)
	for i := 0; i != numVerified; i++ {
		indexes = append(indexes, i)
	}
	bitmap, _ := indexesToBitMap(indexes, numSlots)
	return &testHeader{
		number:           new(big.Int).SetInt64(number),
		shardID:          shardID,
		lastCommitBitmap: bitmap,
	}
}

func (th *testHeader) Number() *big.Int {
	return th.number
}

func (th *testHeader) ShardID() uint32 {
	return th.shardID
}

func (th *testHeader) LastCommitBitmap() []byte {
	return th.lastCommitBitmap
}

// testStateDB is the fake state db for testing
type testStateDB map[common.Address]*staking.ValidatorWrapper

// newTestStateDB return an empty testStateDB
func newTestStateDB() testStateDB {
	state := make(testStateDB)
	return state
}

// newTestStateDBFromCommittee creates a testStateDB given a shard committee.
// The validator wrappers are only set for user nodes.
func newTestStateDBFromCommittee(cmt *shard.Committee) testStateDB {
	state := make(testStateDB)
	for _, slot := range cmt.Slots {
		if slot.EffectiveStake == nil {
			continue
		}
		var wrapper staking.ValidatorWrapper
		wrapper.Address = slot.EcdsaAddress
		wrapper.SlotPubKeys = []bls.SerializedPublicKey{slot.BLSPublicKey}
		wrapper.Counters.NumBlocksSigned = new(big.Int).SetInt64(1)
		wrapper.Counters.NumBlocksToSign = new(big.Int).SetInt64(1)

		state[slot.EcdsaAddress] = &wrapper
	}
	return state
}

// snapshot returns a deep copy of the current test state
func (state testStateDB) snapshot() testStateDB {
	res := make(map[common.Address]*staking.ValidatorWrapper)
	for addr, wrapper := range state {
		wrapperCpy := staking.ValidatorWrapper{
			Validator: staking.Validator{
				Address:     addr,
				SlotPubKeys: make([]bls.SerializedPublicKey, 1),
			},
		}
		copy(wrapperCpy.SlotPubKeys, wrapper.SlotPubKeys)
		wrapperCpy.Counters.NumBlocksToSign = new(big.Int).Set(wrapper.Counters.NumBlocksToSign)
		wrapperCpy.Counters.NumBlocksSigned = new(big.Int).Set(wrapper.Counters.NumBlocksSigned)
		res[addr] = &wrapperCpy
	}
	return res
}

func (state testStateDB) ValidatorWrapper(addr common.Address, readOnly bool, copyDelegations bool) (*staking.ValidatorWrapper, error) {
	wrapper, ok := state[addr]
	if !ok {
		return nil, fmt.Errorf("addr not exist in validator wrapper: %v", addr.String())
	}
	return wrapper, nil
}

func (state testStateDB) UpdateValidatorWrapper(addr common.Address, wrapper *staking.ValidatorWrapper) error {
	state[addr] = wrapper
	return nil
}

func (state testStateDB) GetCode(addr common.Address, isValidatorCode bool) []byte {
	wrapper, ok := state[addr]
	if !ok {
		return nil
	}
	b, _ := rlp.EncodeToBytes(wrapper)
	return b
}

// testReader is the fake Reader for testing
type testReader map[common.Address]staking.ValidatorWrapper

// newTestReader creates an empty test reader
func newTestReader() testReader {
	reader := make(testReader)
	return reader
}

func (reader testReader) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	wrapper, ok := reader[addr]
	if !ok {
		return nil, errors.New("not a valid validator address")
	}
	return &staking.ValidatorSnapshot{
		Validator: &wrapper,
	}, nil
}

func (reader testReader) updateValidatorWrapper(addr common.Address, val *staking.ValidatorWrapper) {
	reader[addr] = *val
}

func makeTestShardState(numShards, numSlots int) *shard.State {
	state := &shard.State{
		Epoch:  new(big.Int).SetInt64(0),
		Shards: make([]shard.Committee, 0, numShards),
	}
	for shardID := uint32(0); shardID != uint32(numShards); shardID++ {
		cmt := makeTestCommittee(numSlots, shardID)
		state.Shards = append(state.Shards, *cmt)
	}
	return state
}

func makeTestCommittee(n int, shardID uint32) *shard.Committee {
	slots := make(shard.SlotList, 0, n)
	for i := 0; i != n; i++ {
		slots = append(slots, makeHmySlot(i, shardID))
	}
	return &shard.Committee{
		ShardID: shardID,
		Slots:   slots,
	}
}

func makeHmySlot(seed int, shardID uint32) shard.Slot {
	addr := common.BigToAddress(new(big.Int).SetInt64(int64(seed) + int64(shardID*1000000)))
	var blsKey bls.SerializedPublicKey
	copy(blsKey[:], bls.RandPrivateKey().GetPublicKey().Serialize())

	return shard.Slot{
		EcdsaAddress: addr,
		BLSPublicKey: blsKey,
	}
}

const testStake = int64(100000000000)

// makeTestMixedCommittee makes a committee with both harmony nodes and user nodes
func makeTestMixedCommittee(numHmySlots, numUserSlots int, shardID uint32) *shard.Committee {
	slots := make(shard.SlotList, 0, numHmySlots+numUserSlots)
	for i := 0; i != numHmySlots; i++ {
		slots = append(slots, makeHmySlot(i, shardID))
	}
	for i := numHmySlots; i != numHmySlots+numUserSlots; i++ {
		slots = append(slots, makeUserSlot(i, shardID))
	}
	return &shard.Committee{
		ShardID: shardID,
		Slots:   slots,
	}
}

func makeUserSlot(seed int, shardID uint32) shard.Slot {
	slot := makeHmySlot(seed, shardID)
	stake := numeric.NewDec(testStake)
	slot.EffectiveStake = &stake
	return slot
}

// indexesToBitMap convert the indexes to bitmap. The conversion follows the little-
// endian order.
func indexesToBitMap(idxs []int, n int) ([]byte, error) {
	bSize := (n + 7) >> 3
	res := make([]byte, bSize)
	for _, idx := range idxs {
		byt := idx >> 3
		if byt >= bSize {
			return nil, fmt.Errorf("overflow index when converting to bitmap: %v/%v", byt, bSize)
		}
		msk := byte(1) << uint(idx&7)
		res[byt] ^= msk
	}
	return res, nil
}

func makeTestWrapper(addr common.Address, numSigned, numToSign int64) staking.ValidatorWrapper {
	var val staking.ValidatorWrapper
	val.Address = addr
	val.Counters.NumBlocksToSign = new(big.Int).SetInt64(numToSign)
	val.Counters.NumBlocksSigned = new(big.Int).SetInt64(numSigned)
	return val
}
