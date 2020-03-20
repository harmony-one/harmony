package slash

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/effective"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	commonCommission = staking.Commission{
		CommissionRates: staking.CommissionRates{
			Rate:          numeric.MustNewDecFromStr("0.167983520183826780"),
			MaxRate:       numeric.MustNewDecFromStr("0.179184469782137200"),
			MaxChangeRate: numeric.MustNewDecFromStr("0.152212761523253600"),
		},
		UpdateHeight: big.NewInt(10),
	}

	commonDescr = staking.Description{
		Name:            "someoneA",
		Identity:        "someoneB",
		Website:         "someoneC",
		SecurityContact: "someoneD",
		Details:         "someoneE",
	}

	fiveKOnes            = new(big.Int).Mul(big.NewInt(5000), big.NewInt(1e18))
	tenKOnes             = new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18))
	onePointNineSixKOnes = new(big.Int).Mul(big.NewInt(19600), big.NewInt(1e18))
	twentyKOnes          = new(big.Int).Mul(big.NewInt(20000), big.NewInt(1e18))
	twentyfiveKOnes      = new(big.Int).Mul(big.NewInt(25000), big.NewInt(1e18))
	thirtyKOnes          = new(big.Int).Mul(big.NewInt(30000), big.NewInt(1e18))
	hundredKOnes         = new(big.Int).Mul(big.NewInt(1000000), big.NewInt(1e18))
)

const (
	// ballot A hex values
	signerABLSPublicHex = "be23bc3c93fe14a25f3533" +
		"feee1cff1c60706845a4907" +
		"c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a"
	signerAHeaderHashHex = "0x68bf572c03e36b4b7a4f268797d18" +
		"7027b288bc69725084bab5bfe6214cb8ddf"
	signerABLSSignature = "ab14e519485b70d8af76ae83205" +
		"290792d679a8a11eb9a12d21787cc0b73" +
		"96e340d88b2a0c39888d0c9cec1" +
		"2c4a09b06b2eec3e851f08f3070f3" +
		"804b35fe4a2033725f073623e3870756141ebc" +
		"2a6495478930c428f6e6b25f292dab8552d30c"
	// ballot B hex values
	signerBBLSPublicHex = "be23bc3c93fe14a25f3533" +
		"feee1cff1c60706845a490" +
		"7c5df58bc19f5d1760bfff" +
		"06fe7c9d1f596b18fdf529e0508e0a"
	signerBHeaderHashHex = "0x1dbf572c03e36b4b7a4f268797d187" +
		"027b288bc69725084bab5bfe6214cb8ddf"
	signerBBLSSignature = "0894d55a541a90ada11535866e5a848d9d6a2b5c30" +
		"932a95f48133f886140cefbe4d690eddd0540d246df1fec" +
		"8b4f719ad9de0bc822f0a1bf70e78b321a5e4462ba3e3efd" +
		"cd24c21b9cb24ed6b26f02785a2cdbd168696c5f4a49b6c00f00994"
	// RLP encoded header
	proposalHeaderRLPBytes = "f9039187486d6e79546764827633f90383a080532768867d8c1a96" +
		"6ae9df80d4efc9a9a83559b0e2d13b2aa819a6a7627c7294" +
		"6911b75b2560be9a8f71164a33086be4511fc99aa0eb514ec3bc1" +
		"8e67ad7d9641768b7a9b9655ff78e54970465aeb037c2d81c5987a05" +
		"6e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc0016" +
		"22fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b" +
		"48e01b996cadc001622fb5e363b421a056e81f171bcc55a6" +
		"ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a05" +
		"6e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc0" +
		"01622fb5e363b421b901000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000000" +
		"000000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000000000" +
		"000000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000000000000" +
		"00002383a9235180845e4c602680a00000000000000000000000" +
		"000000000000000000000000000000000000000000230680b860c" +
		"4db93981d5870f0f77bb8487dde189e4579e3866ae3b598fbed0f5" +
		"d87e53d21a039e427d2a206eeca4ff1f1fab2ed0d2171b9c1f12" +
		"1e42c1a1456e6e1ba4c5e2fb3ddda5d34873bcdf01f14d1da1bc5" +
		"0e70dc6f0bdfda3f36f9dafe1e29880e7fb8807b05ee6062476f5a43" +
		"6b29876d726491fb76bfe2cf8f6d9e3a7361e0ca95bd66c4ef4593c" +
		"2ac616c4ca171af74a93f5ef9e4d1b74421e789f07e0a36e4b00b7a9" +
		"13fb6a296e20ee4dcd2d74088ea9711b8b7693af18d3f6ab925d26a0b" +
		"e30d8899f18e4b7f9e8c6937e78863b828fa172e8edef106cca814294" +
		"eee7146eec7018080b88bf889f887a0a1cc8366aa9c8acac219a229b2" +
		"eb88deee3bc9b133baa610bd88af294422ab0020b860e5a234c8bce79" +
		"24c4f934edf7ce9f8f4b10d61d1e72becdda95c5fb3195598cc9ae334" +
		"8a00c493fb77a87a094dbf08130c3f93793337be78167b8b241d7ef9e" +
		"1fab1f8097dda339a6ca2737c291d8a7733c9b5b540ea30544c46" +
		"a0426a34f60d3f010580"
	// trailing bech32 info
	reporterBech32 = "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"
	offenderBech32 = "one1zyxauxquys60dk824p532jjdq753pnsenrgmef"
	// some rando delegator
	randoDelegatorBech32 = "one1nqevvacj3y5ltuef05my4scwy5wuqteur72jk5"
	// rest of the committee
	commK1 = "65f55eb3052f9e9f632b2923be594ba77c55543f5c58ee1454b9cfd658d25e06373b0f7d42a19c84768139ea294f6204"
	commK2 = "02c8ff0b88f313717bc3a627d2f8bb172ba3ad3bb9ba3ecb8eed4b7c878653d3d4faf769876c528b73f343967f74a917"
	commK3 = "e751ec995defe4931273aaebcb2cd14bf37e629c554a57d3f334c37881a34a6188a93e76113c55ef3481da23b7d7ab09"
	commK4 = "2d61379e44a772e5757e27ee2b3874254f56073e6bd226eb8b160371cc3c18b8c4977bd3dcb71fd57dc62bf0e143fd08"
	commK5 = "86dc2fdc2ceec18f6923b99fd86a68405c132e1005cf1df72dca75db0adfaeb53d201d66af37916d61f079f34f21fb96"
	commK6 = "95117937cd8c09acd2dfae847d74041a67834ea88662a7cbed1e170350bc329e53db151e5a0ef3e712e35287ae954818"
	// double signing info
	doubleSignShardID     = 0
	doubleSignEpoch       = 3
	doubleSignBlockNumber = 37
	doubleSignViewID      = 38
	doubleSignUnixNano    = 1582049233802498300
	// validator creation parameters
	lastEpochInComm = 5
	creationHeight  = 33
	// slash rates to test double signing
	// from 0 percent to 100 percent slash rate, 1 percent increment per testSlashRate
	numSlashRatesToTest = 101
)

type scenario struct {
	snapshot, current *staking.ValidatorWrapper
	slashRate         float64
	result            *Application
}

func defaultFundingScenario() *scenario {
	return &scenario{
		snapshot:  nil,
		current:   nil,
		slashRate: 0.02,
		result:    nil,
	}
}

func scenarioRealWorldSample1() *scenario {
	const (
		snapshotBytes = "f90108f8be94110dde181c2434f6d8eaa869154a4d07a910ce19f1b0be23bc3c93fe14a25f3533feee1cff1c60706845a4907c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a06880de0b6b3a764000088b469471f8014000001e0dec9880254cc1f20ad395cc988027c97536ea18970c988021cc4b33cdff1601df83e945f546573745f6b65795f76616c696461746f72308c746573745f6163636f756e748b6861726d6f6e792e6f6e658a44616e69656c2d56444d846e6f6e651d80f842e094110dde181c2434f6d8eaa869154a4d07a910ce19880f43fc2c04ee000080c0e0949832c677128929f5f3297d364ac30e251dc02f3c8814d1120d7b16000080c0c3060180"

		currentBytes = "f90118f8be94110dde181c2434f6d8eaa869154a4d07a910ce19f1b0be23bc3c93fe14a25f3533feee1cff1c60706845a4907c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a06880de0b6b3a764000088b469471f8014000001e0dec9880254cc1f20ad395cc988027c97536ea18970c988021cc4b33cdff1601df83e945f546573745f6b65795f76616c696461746f72308c746573745f6163636f756e748b6861726d6f6e792e6f6e658a44616e69656c2d56444d846e6f6e651d80f852e894110dde181c2434f6d8eaa869154a4d07a910ce19880f43fc2c04ee000088e6ec131ed55ec404c0e8949832c677128929f5f3297d364ac30e251dc02f3c8814d1120d7b16000088d52ac35139f06a2cc0c3070280"
	)

	snapshotData, _ := hex.DecodeString(snapshotBytes)
	currentData, _ := hex.DecodeString(currentBytes)

	var snapshot, current staking.ValidatorWrapper

	if err := rlp.DecodeBytes(snapshotData, &snapshot); err != nil {
		panic("test case has bad input")
	}

	if err := rlp.DecodeBytes(currentData, &current); err != nil {
		panic("test case has bad input")
	}

	return &scenario{
		slashRate: 0.02,
		result: &Application{
			TotalSlashed:      big.NewInt(0.052 * denominations.One),
			TotalSnitchReward: big.NewInt(0.026 * denominations.One),
		},
		snapshot: &snapshot,
		current:  &current,
	}
}

var (
	scenarioTwoPercent    = defaultFundingScenario()
	scenarioEightyPercent = defaultFundingScenario()
	doubleSignScenarios   = make([]*scenario, numSlashRatesToTest)
)

func totalSlashedExpected(slashRate float64) *big.Int {
	t := int64(50000 * slashRate)
	res := new(big.Int).Mul(big.NewInt(t), big.NewInt(denominations.One))
	return res
	//return big.NewInt(int64(slashRate * 50000 * denominations.One)) // 5.0 * denominations.One
}

func totalSnitchRewardExpected(slashRate float64) *big.Int {
	t := int64(25000 * slashRate)
	res := new(big.Int).Mul(big.NewInt(t), big.NewInt(denominations.One))
	return res
	//return big.NewInt(int64(slashRate * 2.5 * denominations.One))
}

func init() {
	{
		s := scenarioTwoPercent
		s.slashRate = 0.02
		s.result = &Application{
			// totalSlashedExpected
			TotalSlashed: totalSlashedExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 5.0 * denominations.One)),
			// totalSnitchRewardExpected
			TotalSnitchReward: totalSnitchRewardExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 2.5 * denominations.One)),
		}
		s.snapshot, s.current = s.defaultValidatorPair(s.defaultDelegationPair())
	}
	{
		s := scenarioEightyPercent
		s.slashRate = 0.80
		s.result = &Application{
			// totalSlashedExpected
			TotalSlashed: totalSlashedExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 5.0 * denominations.One)),
			// totalSnitchRewardExpected
			TotalSnitchReward: totalSnitchRewardExpected(s.slashRate), // big.NewInt(int64(s.slashRate * 2.5 * denominations.One)),
		}
		s.snapshot, s.current = s.defaultValidatorPair(s.defaultDelegationPair())
	}

	// Testing slash rate from 0% to 100% with increment being 100.0 / numSlashRatesToTest.
	// numSlashRatesToTest=100, so it's 1 percent step size in between test slash rates.
	testSlashRateStepSize := numeric.NewDec(1).
		Quo(numeric.NewDec(numSlashRatesToTest - 1)) // -1 since starting from 0% slash rate edge case
	slashRate := numeric.NewDec(0)
	for i := 0; i < numSlashRatesToTest; i++ {
		s := defaultFundingScenario()
		s.slashRate = float64(slashRate.Quo(numeric.NewDec(100)).TruncateInt64())
		s.result = &Application{
			TotalSlashed:      totalSlashedExpected(s.slashRate),
			TotalSnitchReward: totalSnitchRewardExpected(s.slashRate),
		}
		s.snapshot, s.current = s.defaultValidatorPair(s.defaultDelegationPair())
		doubleSignScenarios[i] = s

		slashRate = slashRate.Add(testSlashRateStepSize)
	}
}

var (
	signerA, signerB           = &bls.PublicKey{}, &bls.PublicKey{}
	hashA, hashB               = common.Hash{}, common.Hash{}
	reporterAddr, offenderAddr = common.Address{}, common.Address{}
	randoDel                   = common.Address{}
	header                     = block.Header{}
	subCommittee               = []shard.BlsPublicKey{}
	doubleSignEpochBig         = big.NewInt(doubleSignEpoch)

	unit = func() interface{} {
		// Ballot A setup
		signerA.DeserializeHexStr(signerABLSPublicHex)
		headerHashA, _ := hex.DecodeString(signerAHeaderHashHex)
		hashA = common.BytesToHash(headerHashA)
		// Ballot B setup
		signerB.DeserializeHexStr(signerBBLSPublicHex)
		headerHashB, _ := hex.DecodeString(signerBHeaderHashHex)
		hashB = common.BytesToHash(headerHashB)
		// address setup
		reporterAddr, _ = common2.Bech32ToAddress(reporterBech32)
		offenderAddr, _ = common2.Bech32ToAddress(offenderBech32)
		randoDel, _ = common2.Bech32ToAddress(randoDelegatorBech32)
		headerData, err := hex.DecodeString(proposalHeaderRLPBytes)
		if err != nil {
			panic("test case has bad input")
		}
		if err := rlp.DecodeBytes(headerData, &header); err != nil {
			panic("test case has bad input")
		}
		for _, hexK := range [...]string{
			commK1, commK2, commK3, commK4, commK5, commK6,
		} {
			k := &bls.PublicKey{}
			k.DeserializeHexStr(hexK)
			subCommittee = append(subCommittee, *shard.FromLibBLSPublicKeyUnsafe(k))
		}
		return nil
	}()

	blsWrapA, blsWrapB = *shard.FromLibBLSPublicKeyUnsafe(signerA),
		*shard.FromLibBLSPublicKeyUnsafe(signerB)
)

func (s *scenario) defaultValidatorPair(
	delegationsSnapshot, delegationsCurrent staking.Delegations,
) (
	*staking.ValidatorWrapper, *staking.ValidatorWrapper,
) {

	validatorSnapshot := &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(lastEpochInComm),
			MinSelfDelegation:    tenKOnes,     //new(big.Int).SetUint64(1 * denominations.One),
			MaxTotalDelegation:   hundredKOnes, //new(big.Int).SetUint64(10 * denominations.One),
			Status:               effective.Active,
			Commission:           commonCommission,
			Description:          commonDescr,
			CreationHeight:       big.NewInt(creationHeight),
		},
		Delegations: delegationsSnapshot,
	}

	validatorCurrent := &staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(lastEpochInComm + 1),
			MinSelfDelegation:    tenKOnes,     // new(big.Int).SetUint64(1 * denominations.One),
			MaxTotalDelegation:   hundredKOnes, // new(big.Int).SetUint64(10 * denominations.One),
			Status:               effective.Active,
			Commission:           commonCommission,
			Description:          commonDescr,
			CreationHeight:       big.NewInt(creationHeight),
		},
		Delegations: delegationsCurrent,
	}
	return validatorSnapshot, validatorCurrent
}

func (s *scenario) defaultDelegationPair() (
	staking.Delegations, staking.Delegations,
) {
	delegationsSnapshot := staking.Delegations{
		// NOTE  delegation is the validator themselves
		staking.Delegation{
			DelegatorAddress: offenderAddr,
			Amount:           twentyKOnes, // new(big.Int).SetUint64(2 * denominations.One),
			Reward:           common.Big0,
			Undelegations:    staking.Undelegations{},
		},
		staking.Delegation{
			DelegatorAddress: randoDel,
			Amount:           thirtyKOnes, //new(big.Int).SetUint64(3 * denominations.One),
			Reward:           common.Big0,
			Undelegations:    staking.Undelegations{},
		},
	}

	delegationsCurrent := staking.Delegations{
		staking.Delegation{
			DelegatorAddress: offenderAddr,
			Amount:           onePointNineSixKOnes, //new(big.Int).SetUint64(1.96 * denominations.One),
			Reward:           common.Big0,
			Undelegations: staking.Undelegations{
				staking.Undelegation{
					Amount: tenKOnes, //new(big.Int).SetUint64(1 * denominations.One),
					Epoch:  big.NewInt(doubleSignEpoch + 2),
				},
			},
		},
		// some external delegator
		staking.Delegation{
			DelegatorAddress: randoDel,
			Amount:           fiveKOnes, //new(big.Int).SetUint64(0.5 * denominations.One),
			Reward:           common.Big0,
			Undelegations: staking.Undelegations{
				staking.Undelegation{
					Amount: twentyfiveKOnes, //new(big.Int).SetUint64(2.5 * denominations.One),
					Epoch:  big.NewInt(doubleSignEpoch + 2),
				},
			},
		},
	}

	return delegationsSnapshot, delegationsCurrent
}

func defaultSlashRecord() Record {
	return Record{
		Evidence: Evidence{
			ConflictingBallots: ConflictingBallots{
				AlreadyCastBallot: votepower.Ballot{
					SignerPubKey:    blsWrapA,
					BlockHeaderHash: hashA,
					Signature:       common.Hex2Bytes(signerABLSSignature),
					Height:          doubleSignBlockNumber,
					ViewID:          doubleSignViewID,
				},
				DoubleSignedBallot: votepower.Ballot{
					SignerPubKey:    blsWrapB,
					BlockHeaderHash: hashB,
					Signature:       common.Hex2Bytes(signerBBLSSignature),
					Height:          doubleSignBlockNumber,
					ViewID:          doubleSignViewID,
				},
			},
			Moment: Moment{
				Epoch:        big.NewInt(doubleSignEpoch),
				TimeUnixNano: big.NewInt(doubleSignUnixNano),
				ShardID:      doubleSignShardID,
			},
			ProposalHeader: &header,
		},
		Reporter: reporterAddr,
		Offender: offenderAddr,
	}
}

func exampleSlashRecords() Records {
	return Records{defaultSlashRecord()}
}

type mockOutSnapshotReader struct {
	snapshot staking.ValidatorWrapper
}

func (m mockOutSnapshotReader) ReadValidatorSnapshotAtEpoch(
	epoch *big.Int,
	addr common.Address,
) (*staking.ValidatorWrapper, error) {
	return &m.snapshot, nil
}

type mockOutChainReader struct{}

func (mockOutChainReader) CurrentBlock() *types.Block {
	b := types.Block{}
	b.Header().SetEpoch(doubleSignEpochBig)
	return &b
}

func (mockOutChainReader) ReadShardState(epoch *big.Int) (*shard.State, error) {
	return &shard.State{
		Epoch: doubleSignEpochBig,
		Shards: []shard.Committee{
			shard.Committee{
				ShardID: doubleSignShardID,
				Slots: shard.SlotList{
					shard.Slot{
						EcdsaAddress:   offenderAddr,
						BlsPublicKey:   blsWrapA,
						EffectiveStake: nil,
					},
				},
			},
		},
	}, nil
}

func TestVerify(t *testing.T) {
	stateHandle := defaultStateWithAccountsApplied()

	if err := Verify(
		mockOutChainReader{}, stateHandle, &exampleSlashRecords()[0],
	); err != nil {
		// TODO
		// t.Errorf("could not verify slash %s", err.Error())
	}
}

func testScenario(
	t *testing.T, stateHandle *state.DB, slashes Records, s *scenario,
) {
	if err := stateHandle.UpdateValidatorWrapper(
		offenderAddr, s.snapshot,
	); err != nil {
		t.Fatalf("creation of validator failed %s", err.Error())
	}

	stateHandle.IntermediateRoot(false)
	stateHandle.Commit(false)

	if err := stateHandle.UpdateValidatorWrapper(
		offenderAddr, s.current,
	); err != nil {
		t.Fatalf("update of validator failed %s", err.Error())
	}

	stateHandle.IntermediateRoot(false)
	stateHandle.Commit(false)
	// NOTE See dump.json to see what account
	// state looks like as of this point

	slashResult, err := Apply(
		mockOutSnapshotReader{*s.snapshot},
		stateHandle,
		slashes,
		numeric.MustNewDecFromStr(
			fmt.Sprintf("%f", s.slashRate),
		),
	)

	if err != nil {
		t.Fatalf("rate: %v, slash application failed %s", s.slashRate, err.Error())
	}

	if sn := slashResult.TotalSlashed; sn.Cmp(
		s.result.TotalSlashed,
	) != 0 {
		t.Errorf(
			"total slash incorrect have %v want %v",
			sn,
			s.result.TotalSlashed,
		)
	}

	if sn := slashResult.TotalSnitchReward; sn.Cmp(
		s.result.TotalSnitchReward,
	) != 0 {
		t.Errorf(
			"total snitch incorrect have %v want %v",
			sn,
			s.result.TotalSnitchReward,
		)
	}
}

func defaultStateWithAccountsApplied() *state.DB {
	st := ethdb.NewMemDatabase()
	stateHandle, _ := state.New(common.Hash{}, state.NewDatabase(st))
	for _, addr := range []common.Address{reporterAddr, offenderAddr, randoDel} {
		stateHandle.CreateAccount(addr)
	}
	stateHandle.SetBalance(offenderAddr, big.NewInt(0).SetUint64(1994680320000000000))
	stateHandle.SetBalance(randoDel, big.NewInt(0).SetUint64(1999975592000000000))
	return stateHandle
}

func TestTwoPercentSlashed(t *testing.T) {
	slashes := exampleSlashRecords()
	stateHandle := defaultStateWithAccountsApplied()
	testScenario(t, stateHandle, slashes, scenarioTwoPercent)
}

// func TestEightyPercentSlashed(t *testing.T) {
// 	slashes := exampleSlashRecords()
// 	stateHandle := defaultStateWithAccountsApplied()
// 	testScenario(t, stateHandle, slashes, scenarioEightyPercent)
// }

func TestDoubleSignSlashRates(t *testing.T) {
	for _, scenario := range doubleSignScenarios {
		slashes := exampleSlashRecords()
		stateHandle := defaultStateWithAccountsApplied()
		testScenario(t, stateHandle, slashes, scenario)
	}
}

// Simply testing serialization / deserialization of slash records working correctly
func TestRoundTripSlashRecord(t *testing.T) {
	slashes := exampleSlashRecords()
	serializedA := slashes.String()
	data, err := rlp.EncodeToBytes(slashes)
	if err != nil {
		t.Errorf("encoding slash records failed %s", err.Error())
	}
	roundTrip := Records{}
	if err := rlp.DecodeBytes(data, &roundTrip); err != nil {
		t.Errorf("decoding slash records failed %s", err.Error())
	}
	serializedB := roundTrip.String()
	if serializedA != serializedB {
		t.Error("rlp encode/decode round trip records failed")
	}
}

func TestSetDifference(t *testing.T) {
	setA, setB := exampleSlashRecords(), exampleSlashRecords()
	additionalSlash := defaultSlashRecord()
	additionalSlash.Evidence.Epoch.Add(additionalSlash.Evidence.Epoch, common.Big1)
	setB = append(setB, additionalSlash)
	diff := setA.SetDifference(setB)
	if diff[0].Hash() != additionalSlash.Hash() {
		t.Errorf("did not get set difference of slash")
	}
}

// TODO bytes used for this example are stale, need to update RLP dump
// func TestApply(t *testing.T) {
// 	slashes := exampleSlashRecords()
// {
// 	stateHandle := defaultStateWithAccountsApplied()
// 	testScenario(t, stateHandle, slashes, scenarioRealWorldSample1())
// }
// }
