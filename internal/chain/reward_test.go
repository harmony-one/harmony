package chain

import (
	"fmt"
	"math/big"
	"testing"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/engine"
	consensus_sig "github.com/harmony-one/harmony/consensus/signature"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
	types2 "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

var (
	EPOCH  = big.NewInt(1000) // epoch where voting power of harmony nodes sums up to 49%
	bigOne = big.NewInt(1e18)

	twentyKOnes      = new(big.Int).Mul(big.NewInt(20_000), bigOne)
	fourtyKOnes      = new(big.Int).Mul(big.NewInt(40_000), bigOne)
	asDecTwentyKOnes = numeric.NewDecFromBigInt(twentyKOnes)
	asDecFourtyKOnes = numeric.NewDecFromBigInt(fourtyKOnes)
	asDecZero        = numeric.ZeroDec()
)

var (
	// Test addresses
	addrA   = makeTestAddress("A")
	addrB   = makeTestAddress("B")
	addrC   = makeTestAddress("C")
	addrD   = makeTestAddress("D")
	addrE   = makeTestAddress("E")
	blsKeys = genKeyPairs(5)

	bc = generateMockBCWithValidators([]Validator{
		{address: addrA},
		{address: addrB},
		{address: addrC},
		{address: addrD},
		{address: addrE},
	})

	slotList = shard.SlotList{
		// External validators
		shard.Slot{
			EcdsaAddress:   addrA,
			BLSPublicKey:   blsKeys[0].Pub(),
			EffectiveStake: &asDecTwentyKOnes,
		},
		shard.Slot{
			EcdsaAddress:   addrB,
			BLSPublicKey:   blsKeys[1].Pub(),
			EffectiveStake: &asDecTwentyKOnes,
		},
		shard.Slot{
			EcdsaAddress:   addrC,
			BLSPublicKey:   blsKeys[2].Pub(),
			EffectiveStake: &asDecFourtyKOnes,
		},
		// Harmony nodes, since they don't contain any effective stake
		shard.Slot{
			EcdsaAddress: addrD,
			BLSPublicKey: blsKeys[3].Pub(),
		},
		shard.Slot{
			EcdsaAddress: addrE,
			BLSPublicKey: blsKeys[4].Pub(),
		},
	}
)

// Note: some testcases are not real-life applicable according to BFT.
// Just testing the reward calculation formula.
func TestCalculateIssuanceRewards(t *testing.T) {
	testcases := []struct {
		signersIdx []int
		expected   numeric.Dec
	}{
		// Issuance based rewards formula: (Overall Percent)^2 * 6
		{
			// Overall: 0.25*0.51 (theirs) + 0.49*0.5 (ours) = 37.25%
			expected:   numeric.MustNewDecFromStr("0.8325375").MulInt(bigOne),
			signersIdx: []int{0, 4},
		},
		{
			// Overall: 0.5*0.51 (theirs) + 0.5*0.49 (ours) = 50%
			expected:   numeric.MustNewDecFromStr("1.5").MulInt(bigOne),
			signersIdx: []int{0, 1, 4},
		},
		{
			// Overall: 0*0.51 (theirs) + 1*0.49 (ours) = 49%
			expected:   numeric.MustNewDecFromStr("1.4406").MulInt(bigOne),
			signersIdx: []int{3, 4},
		},
		{
			// Overall: 1*0.51 (theirs) + 1*0.49 (ours)= 100%
			expected:   numeric.MustNewDecFromStr("6").MulInt(bigOne),
			signersIdx: []int{0, 1, 2, 3, 4},
		},
	}

	cxLink := types.CrossLink{}
	cxLink.EpochF = EPOCH

	for i, testcase := range testcases {
		bitmap, _ := indexesToBitMap(testcase.signersIdx, 3)
		cxLink.BitmapF = bitmap

		res, err := calculateIssuanceRewards(bc, cxLink)
		if err != nil {
			t.Errorf("Issuance rewards calculation failed for testcase: %v. Error: %v/", i, err)
		}

		if !res.Equal(testcase.expected) {
			t.Errorf("Issuance rewards were not calculated properly for tescase %v. Expected/Actual: %v/%v", i, testcase.expected, res)
		}
	}
}

/*
	Test Utilities
*/

//
// Validator setup
//

type Validator struct {
	address     common.Address
	delegations Delegations
}
type Delegations = map[common.Address]*big.Int

func generateValidator(address common.Address, delegations map[common.Address]*big.Int) *staking.ValidatorWrapper {
	v := staking.Validator{}

	ds := staking.Delegations{}
	for key, amount := range delegations {
		ds = append(ds, staking.Delegation{
			DelegatorAddress: key,
			Amount:           amount,
		})
	}

	return &staking.ValidatorWrapper{
		Validator:   v,
		Delegations: ds,
	}
}

//
// BLS keys setup
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
	msg := consensus_sig.ConstructCommitPayload(chain, block.Epoch(), block.Hash(),
		block.Number().Uint64(), block.Header().ViewID().Uint64())
	sig := kp.pri.SignHash(msg)

	return sig.Serialize()
}

//
// Shard state setup
//

func makeTestShardState() *shard.State {
	return &shard.State{
		Epoch: EPOCH,
		Shards: []shard.Committee{
			{ShardID: 0, Slots: slotList},
		},
	}
}

//
// Blockchain setup
//

type fakeBlockChain struct {
	config         params.ChainConfig
	currentBlock   types.Block
	superCommittee shard.State
	snapshots      map[common.Address]*staking.ValidatorWrapper
}

func generateMockBCWithValidators(validators []Validator) *fakeBlockChain {
	snapshots := make(map[common.Address]*staking.ValidatorWrapper)
	for _, validator := range validators {
		snapshots[validator.address] = generateValidator(validator.address, validator.delegations)
	}
	return &fakeBlockChain{
		config:    *params.LocalnetChainConfig,
		snapshots: snapshots,
	}
}

func (bc *fakeBlockChain) CurrentBlock() *types.Block {
	return &bc.currentBlock
}
func (bc *fakeBlockChain) CurrentHeader() *block.Header {
	return bc.currentBlock.Header()
}
func (bc *fakeBlockChain) ReadShardState(epoch *big.Int) (*shard.State, error) {
	return makeTestShardState(), nil
}
func (bc *fakeBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block    { return nil }
func (bc *fakeBlockChain) GetHeader(hash common.Hash, number uint64) *block.Header  { return nil }
func (bc *fakeBlockChain) GetHeaderByHash(hash common.Hash) *block.Header           { return nil }
func (bc *fakeBlockChain) ShardID() uint32                                          { return 0 }
func (bc *fakeBlockChain) WriteCommitSig(blockNum uint64, lastCommits []byte) error { return nil }
func (bc *fakeBlockChain) GetHeaderByNumber(number uint64) *block.Header            { return nil }
func (bc *fakeBlockChain) ReadValidatorList() ([]common.Address, error)             { return nil, nil }
func (bc *fakeBlockChain) ReadCommitSig(blockNum uint64) ([]byte, error)            { return nil, nil }
func (bc *fakeBlockChain) ReadBlockRewardAccumulator(uint64) (*big.Int, error)      { return nil, nil }
func (bc *fakeBlockChain) ValidatorCandidates() []common.Address                    { return nil }
func (cr *fakeBlockChain) ReadValidatorInformationAtState(addr common.Address, state *state.DB) (*staking.ValidatorWrapper, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorSnapshotAtEpoch(epoch *big.Int, addr common.Address) (*types2.ValidatorSnapshot, error) {
	return nil, nil
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
func (bc *fakeBlockChain) ReadValidatorSnapshot(addr common.Address) (*staking.ValidatorSnapshot, error) {
	return &staking.ValidatorSnapshot{
		Epoch:     big.NewInt(1),
		Validator: bc.snapshots[addr],
	}, nil
}
func (bc *fakeBlockChain) SuperCommitteeForNextEpoch(beacon engine.ChainReader, header *block.Header, isVerify bool) (*shard.State, error) {
	return nil, nil
}
func (bc *fakeBlockChain) ReadValidatorStats(addr common.Address) (*staking.ValidatorStats, error) {
	return nil, nil
}

//
// Utilities
//

func makeTestAddress(item interface{}) common.Address {
	s := fmt.Sprintf("harmony.one.%v", item)
	return common.BytesToAddress([]byte(s))
}

func generateSlotList(names []string) shard.SlotList {
	slotList := make(shard.SlotList, len(names))
	for _, name := range names {
		slot := shard.Slot{
			EcdsaAddress: makeTestAddress(name),
			BLSPublicKey: bls.SerializedPublicKey{},
		}
		slotList = append(slotList, slot)
	}

	return slotList
}

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
