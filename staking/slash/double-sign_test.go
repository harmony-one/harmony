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
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/numeric"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
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
	// the minimial by protocol, 1 ONE
	minSelfDelgation   = 1_000_000_000_000_000_000
	maxTotalDelegation = 13_000_000_000_000_000_000

	// delegation creation parameters
	delegationSnapshotI1 = 2_000_000_000_000_000_000
	delegationSnapshotI2 = 3_000_000_000_000_000_000
	delegationCurrentI1  = 1_000_000_000_000_000_000
	delegationCurrentI2  = 500_000_000_000_000_000
	undelegateI1         = delegationSnapshotI1 - delegationCurrentI1
	undelegateI2         = delegationSnapshotI2 - delegationCurrentI2

	// Remember to change these in tandum
	slashRate  = 0.02
	slashRateS = "0.02"

	// Should be 4e+17
	slashMagnitudeI1 = delegationSnapshotI1 * slashRate
	slashMagnitudeI2 = delegationSnapshotI2 * slashRate

	// Should be 1.6e+18
	expectedBalancePostSlashI1 = delegationSnapshotI1 - slashMagnitudeI1
	expectedBalancePostSlashI2 = delegationSnapshotI2 - slashMagnitudeI2

	// expected slashing results
	expectTotalSlashMagnitude = slashMagnitudeI1 + slashMagnitudeI2
	expectSnitch              = expectTotalSlashMagnitude / 2.0

	slashAppliedToCurrentBalanceI1 = delegationCurrentI1 - slashMagnitudeI1
	slashAppliedToCurrentBalanceI2 = delegationCurrentI2 - slashMagnitudeI2
)

var (
	signerA, signerB           = &bls.PublicKey{}, &bls.PublicKey{}
	hashA, hashB               = common.Hash{}, common.Hash{}
	signatureA, signatureB     = &bls.Sign{}, &bls.Sign{}
	reporterAddr, offenderAddr = common.Address{}, common.Address{}
	randoDel                   = common.Address{}
	header                     = block.Header{}
	subCommittee               = []shard.BlsPublicKey{}

	unit = func() interface{} {

		if expectedBalancePostSlashI1+slashMagnitudeI1 != delegationSnapshotI1 {
			panic("bad constant time values for computation on slash - delegation 1")
		}

		if expectedBalancePostSlashI2+slashMagnitudeI2 != delegationSnapshotI2 {
			panic("bad constant time values for computation on slash - delegation 2")
		}

		fmt.Println("expected values1",
			delegationSnapshotI1,
			delegationCurrentI1,
			expectedBalancePostSlashI1,
			slashMagnitudeI1,
			slashAppliedToCurrentBalanceI1,
		)

		fmt.Println("expected values2",
			delegationSnapshotI2,
			delegationCurrentI2,
			expectedBalancePostSlashI2,
			slashMagnitudeI2,
			slashAppliedToCurrentBalanceI2,
		)

		// if expectedSlashI1 > delegationCurrentI1 {
		// 	fmt.Printf(
		// 		"self delegation debt => slash %v, current amt %v find debt %v\n",
		// 		expectedSlashI1, delegationCurrentI1, delegationCurrentI1-expectedSlashI1,
		// 	)
		// }

		// if expectedSlashI2 > delegationCurrentI2 {
		// 	fmt.Printf(
		// 		"external delegation => slash %v, current amt %v find debt %v\n",
		// 		expectedSlashI2, delegationCurrentI2, delegationCurrentI2-expectedSlashI2,
		// 	)
		// }

		// Ballot A setup
		signerA.DeserializeHexStr(signerABLSPublicHex)
		headerHashA, _ := hex.DecodeString(signerAHeaderHashHex)
		hashA = common.BytesToHash(headerHashA)
		signatureA.DeserializeHexStr(signerABLSSignature)

		// Ballot B setup
		signerB.DeserializeHexStr(signerBBLSPublicHex)
		headerHashB, _ := hex.DecodeString(signerBHeaderHashHex)
		hashB = common.BytesToHash(headerHashB)
		signatureB.DeserializeHexStr(signerBBLSSignature)

		reporterAddr, _ = common2.Bech32ToAddress(reporterBech32)
		offenderAddr, _ = common2.Bech32ToAddress(offenderBech32)
		randoDel, _ = common2.Bech32ToAddress(randoDelegatorBech32)
		// TODO Do rlp decode on bytes for this header
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
		// fmt.Println("header from rlp!", header.String())
		return nil
	}()

	blsWrapA, blsWrapB = *shard.FromLibBLSPublicKeyUnsafe(signerA),
		*shard.FromLibBLSPublicKeyUnsafe(signerB)

	commonCommission = staking.Commission{
		CommissionRates: staking.CommissionRates{
			Rate:          numeric.MustNewDecFromStr("0.167983520183826780"),
			MaxRate:       numeric.MustNewDecFromStr("0.179184469782137200"),
			MaxChangeRate: numeric.MustNewDecFromStr("0.152212761523253600"),
		},
		UpdateHeight: big.NewInt(10),
	}

	commonDescr = staking.Description{
		Name:            "someone",
		Identity:        "someone",
		Website:         "someone",
		SecurityContact: "someone",
		Details:         "someone",
	}

	shouldBeTotalSlashed      = big.NewInt(int64(expectTotalSlashMagnitude))
	shouldBeTotalSnitchReward = big.NewInt(int64(expectSnitch))
)

func validatorPair(delegationsSnapshot, delegationsCurrent staking.Delegations) (
	validatorSnapshot, validatorCurrent staking.ValidatorWrapper,
) {
	minSelf, maxDel :=
		big.NewInt(minSelfDelgation), big.NewInt(0).SetUint64(maxTotalDelegation)

	validatorSnapshot = staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(lastEpochInComm),
			MinSelfDelegation:    minSelf,
			MaxTotalDelegation:   maxDel,
			Active:               true,
			Commission:           commonCommission,
			Description:          commonDescr,
			CreationHeight:       big.NewInt(creationHeight),
			Banned:               false,
		},
		Delegations: delegationsSnapshot,
	}

	validatorCurrent = staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(lastEpochInComm + 1),
			MinSelfDelegation:    minSelf,
			MaxTotalDelegation:   maxDel,
			Active:               true,
			Commission:           commonCommission,
			Description:          commonDescr,
			CreationHeight:       big.NewInt(creationHeight),
			Banned:               false,
		},
		Delegations: delegationsCurrent,
	}
	return
}

func delegationPair() (staking.Delegations, staking.Delegations) {
	delegationsSnapshot := staking.Delegations{
		// NOTE  delegation is the validator themselves
		staking.Delegation{
			DelegatorAddress: offenderAddr,
			Amount:           big.NewInt(0).SetUint64(delegationSnapshotI1),
			Reward:           big.NewInt(0),
			Undelegations:    staking.Undelegations{},
		},
		staking.Delegation{
			DelegatorAddress: randoDel,
			Amount:           big.NewInt(0).SetUint64(delegationSnapshotI2),
			Reward:           big.NewInt(0),
			Undelegations:    staking.Undelegations{},
		},
	}

	delegationsCurrent := staking.Delegations{
		staking.Delegation{
			DelegatorAddress: offenderAddr,
			Amount:           big.NewInt(0).SetUint64(delegationCurrentI1),
			Reward:           big.NewInt(0),
			Undelegations: staking.Undelegations{
				staking.Undelegation{
					Amount: big.NewInt(undelegateI1),
					Epoch:  big.NewInt(doubleSignEpoch + 2),
				},
			},
		},
		// some external delegator
		staking.Delegation{
			DelegatorAddress: randoDel,
			Amount:           big.NewInt(delegationCurrentI2),
			Reward:           big.NewInt(0),
			Undelegations: staking.Undelegations{
				staking.Undelegation{
					Amount: big.NewInt(undelegateI2),
					Epoch:  big.NewInt(doubleSignEpoch + 2),
				},
			},
		},
	}

	// fmt.Println("the delegations", delegationsSnapshot.String(), "\n", delegationsCurrent.String())

	return delegationsSnapshot, delegationsCurrent
}

func exampleSlashRecords() Records {
	return Records{
		Record{
			ConflictingBallots: ConflictingBallots{
				AlreadyCastBallot: votepower.Ballot{
					SignerPubKey:    blsWrapA,
					BlockHeaderHash: hashA,
					Signature:       signatureA,
				},
				DoubleSignedBallot: votepower.Ballot{
					SignerPubKey:    blsWrapB,
					BlockHeaderHash: hashB,
					Signature:       signatureB,
				},
			},
			Evidence: Evidence{
				Moment: Moment{
					Epoch:        big.NewInt(doubleSignEpoch),
					Height:       big.NewInt(doubleSignBlockNumber),
					TimeUnixNano: big.NewInt(doubleSignUnixNano),
					ViewID:       doubleSignViewID,
					ShardID:      doubleSignShardID,
				},
				ProposalHeader: &header,
			},
			Reporter: reporterAddr,
			Offender: offenderAddr,
		},
	}

}

type mockOutSnapshotReader struct {
	snapshot staking.ValidatorWrapper
}

func (m mockOutSnapshotReader) ReadValidatorSnapshot(common.Address) (*staking.ValidatorWrapper, error) {
	return &m.snapshot, nil
}

func TestVerify(t *testing.T) {
	//
}

func TestApply(t *testing.T) {
	st := ethdb.NewMemDatabase()
	stateHandle, _ := state.New(common.Hash{}, state.NewDatabase(st))
	slashes := exampleSlashRecords()
	validatorSnapshot, validatorCurrent := validatorPair(delegationPair())

	for _, addr := range []common.Address{reporterAddr, offenderAddr, randoDel} {
		stateHandle.CreateAccount(addr)
	}

	stateHandle.SetBalance(offenderAddr, big.NewInt(0).SetUint64(1994680320000000000))
	stateHandle.SetBalance(randoDel, big.NewInt(0).SetUint64(1999975592000000000))

	if err := stateHandle.UpdateStakingInfo(
		offenderAddr, &validatorSnapshot,
	); err != nil {
		t.Fatalf("creation of validator failed %s", err.Error())
	}

	stateHandle.IntermediateRoot(false)
	stateHandle.Commit(false)

	dump := stateHandle.Dump()

	if err := stateHandle.UpdateStakingInfo(
		offenderAddr, &validatorCurrent,
	); err != nil {
		t.Fatalf("update of validator failed %s", err.Error())
	}

	stateHandle.IntermediateRoot(false)
	stateHandle.Commit(false)

	// NOTE See dump.json to see what account
	// state looks like as of this point

	// fmt.Println("Before slash apply", stateHandle.Dump())
	dump = stateHandle.Dump()
	if len(dump) > 0 {
		//
	}

	slashRateH := numeric.MustNewDecFromStr(slashRateS)

	slashResult, err := Apply(
		mockOutSnapshotReader{validatorSnapshot}, stateHandle, slashes, slashRateH,
	)

	// fmt.Println("After slash apply", stateHandle.Dump())

	if err != nil {
		t.Fatalf("slash application failed %s", err.Error())
	}

	fmt.Println(slashResult.String())

	if sn := slashResult.TotalSlashed; sn.Cmp(shouldBeTotalSlashed) != 0 {
		t.Errorf(
			"total slash incorrect have %v want %v", sn, shouldBeTotalSlashed,
		)
	}

	// if sn := slashResult.TotalSnitchReward; sn.Cmp(shouldBeTotalSnitchReward) != 0 {
	// 	t.Errorf(
	// 		"total snitch incorrect have %v want %v", sn, shouldBeTotalSnitchReward,
	// 	)
	// }
}
