package slash

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
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
	// trailing bech32 info
	reporterBech32 = "one1pdv9lrdwl0rg5vglh4xtyrv3wjk3wsqket7zxy"
	offenderBech32 = "one1zyxauxquys60dk824p532jjdq753pnsenrgmef"
	// some rando delegator
	randoDelegatorBech32  = "one1nqevvacj3y5ltuef05my4scwy5wuqteur72jk5"
	doubleSignEpoch       = 3
	doubleSignBlockNumber = 37
	doubleSignViewID      = 38
)

var (
	signerA, signerB           = &bls.PublicKey{}, &bls.PublicKey{}
	hashA, hashB               = common.Hash{}, common.Hash{}
	signatureA, signatureB     = &bls.Sign{}, &bls.Sign{}
	reporterAddr, offenderAddr = common.Address{}, common.Address{}
	randoDel                   = common.Address{}
	header                     = &block.Header{}

	unit = func() interface{} {
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

		return nil
	}()

	blsWrapA, blsWrapB = *shard.FromLibBLSPublicKeyUnsafe(signerA),
		*shard.FromLibBLSPublicKeyUnsafe(signerB)

	exampleSlash = Records{
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
					TimeUnixNano: big.NewInt(1582049233802498300),
					ViewID:       doubleSignViewID,
					ShardID:      0,
				},
				ProposalHeader: header,
			},
			Reporter: reporterAddr,
			Offender: offenderAddr,
		},
	}

	validatorSnapshot = staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(2),
			MinSelfDelegation:    nil,
			MaxTotalDelegation:   nil,
			Active:               true,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					// NOTE not relevant for slashing
				},
				UpdateHeight: big.NewInt(10),
			},
			Description: staking.Description{
				// NOTE not relevant for slashing
			},
			CreationHeight: big.NewInt(33),
			Banned:         false,
		},
		Delegations: staking.Delegations{
			// NOTE  delegation is the validator themselves
			staking.Delegation{
				DelegatorAddress: offenderAddr,
				Amount:           big.NewInt(5),
				Reward:           big.NewInt(0),
				Undelegations:    staking.Undelegations{},
			},
			// some external delegator
			staking.Delegation{
				DelegatorAddress: randoDel,
				Amount:           big.NewInt(5),
				Reward:           big.NewInt(0),
				Undelegations:    staking.Undelegations{},
			},
		},
	}

	validatorCurrent = staking.ValidatorWrapper{
		Validator: staking.Validator{
			Address:              offenderAddr,
			SlotPubKeys:          []shard.BlsPublicKey{blsWrapA},
			LastEpochInCommittee: big.NewInt(2),
			MinSelfDelegation:    nil,
			MaxTotalDelegation:   nil,
			Active:               true,
			Commission: staking.Commission{
				CommissionRates: staking.CommissionRates{
					// NOTE not relevant for slashing
				},
				UpdateHeight: big.NewInt(10),
			},
			Description: staking.Description{
				// NOTE not relevant for slashing
			},
			CreationHeight: big.NewInt(33),
			Banned:         false,
		},
		Delegations: staking.Delegations{
			// First delegation is the validator themselves
			staking.Delegation{
				DelegatorAddress: offenderAddr,
				Amount:           big.NewInt(1),
				Reward:           big.NewInt(0),
				Undelegations: staking.Undelegations{
					staking.Undelegation{
						Amount: big.NewInt(4),
						Epoch:  big.NewInt(doubleSignEpoch + 2),
					},
				},
			},
		},
	}
)

type mockOutSnapshotReader struct{}

func (mockOutSnapshotReader) ReadValidatorSnapshot(common.Address) (*staking.ValidatorWrapper, error) {
	return &validatorSnapshot, nil
}

func TestVerify(t *testing.T) {
	//
}

func TestApply(t *testing.T) {
	st := ethdb.NewMemDatabase()
	stateHandle, _ := state.New(common.Hash{}, state.NewDatabase(st))

	fmt.Println("slash->", exampleSlash.String())
	stateHandle.UpdateStakingInfo(offenderAddr, &validatorCurrent)

	t.Log("Unimplemented")
}
