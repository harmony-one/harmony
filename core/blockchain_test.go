package core

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	staking "github.com/harmony-one/harmony/staking/types"
)

func TestPrepareStakingMetadata(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	// fake transaction
	tx := types.NewTransaction(1, common.BytesToAddress([]byte{0x11}), 0, big.NewInt(111), 1111, big.NewInt(11111), []byte{0x11, 0x11, 0x11})
	txs := []*types.Transaction{tx}

	// fake staking transactions
	stx1 := signedCreateValidatorStakingTxn(key)
	stx2 := signedDelegateStakingTxn(key)
	stxs := []*staking.StakingTransaction{stx1, stx2}

	// make a fake block header
	block := types.NewBlock(header, txs, []*types.Receipt{types.NewReceipt([]byte{}, false, 0), types.NewReceipt([]byte{}, false, 0),
		types.NewReceipt([]byte{}, false, 0)}, nil, nil, stxs)
	// run it
	if _, _, err := chain.prepareStakingMetaData(block, []staking.StakeMsg{&staking.Delegate{}}, db); err != nil {
		if err.Error() != "address not present in state" { // when called in test for core/vm
			t.Errorf("Got error %v in prepareStakingMetaData", err)
		}
	} else {
		// when called independently there is no error
	}
}

func TestUpdateDelegationIndices(t *testing.T) {
	key, _ := crypto.GenerateKey()
	bc, _, _, _ := getTestEnvironment(*key)
	batch := bc.db.NewBatch()
	validator1 := makeTestAddr("validator1")
	// self delegation
	if err := bc.writeDelegationsByDelegator(
		batch,
		validator1,
		[]staking.DelegationIndex{
			{
				ValidatorAddress: validator1,
				Index:            uint64(0),
				BlockNum:         big.NewInt(0),
			},
		},
	); err != nil {
		t.Fatalf("Could not write delegationIndex due to %s", err)
	}
	// add 3 more delegations to the same validator
	delegators := [3]common.Address{}
	delegationsToAlter := make(map[common.Address](map[common.Address]uint64), 3)
	for i := 0; i < 3; i++ {
		delegators[i] = makeTestAddr(fmt.Sprintf("delegator%d", i))
		delegationsToAlter[delegators[i]] = make(map[common.Address]uint64)
		if err := bc.writeDelegationsByDelegator(
			batch,
			delegators[i],
			[]staking.DelegationIndex{
				{
					ValidatorAddress: validator1,
					Index:            uint64(i + 1), // 0 is taken already
					BlockNum:         big.NewInt(0),
				},
			},
		); err != nil {
			t.Fatalf("Could not write delegationIndex due to %s", err)
		}
	}
	// remove just the middle guy so we can check
	// if the first guy is unchanged
	// middle guy is missing
	// and last guy has moved
	delegationsToAlter[delegators[1]][validator1] = math.MaxUint64
	delegationsToAlter[delegators[2]][validator1] = uint64(1)
	if err := bc.UpdateDelegationIndices(
		batch,
		delegationsToAlter,
	); err != nil {
		t.Fatalf("Could not update delegations indices due to %s", err)
	}
	expected := [3]uint64{1, math.MaxUint64, 2}
	for i := 0; i < 3; i++ {
		if delegations, err := bc.ReadDelegationsByDelegator(
			delegators[i],
		); err != nil {
			t.Fatalf("Could not read delegations by %s due to %s", delegators[i].Hex(), err)
		} else {
			if len(delegations) == 1 {
				if expected[i] != delegations[0].Index {
					t.Errorf(
						"Unexpected delegation position for delegator %d (expected: %d, actual: %d)",
						i,
						expected[i],
						delegations[0].Index,
					)
				}
			} else if len(delegations) == 0 {
				if expected[i] != math.MaxUint64 {
					t.Errorf(
						"Missing delegationIndex for delegator %d", i,
					)
				}
			} else {
				t.Errorf(
					"Unexpected # of delegations %d for delegator %d",
					len(delegations), i,
				)
			}
		}
	}
}

func signedCreateValidatorStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCreateValidator, sampleCreateValidator(*key)
	}
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedEditValidatorStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveEditValidator, sampleEditValidator(*key)
	}
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedDelegateStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveDelegate, sampleDelegate(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedUndelegateStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveUndelegate, sampleUndelegate(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}

func signedCollectRewardsStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCollectRewards, sampleCollectRewards(*key)
	}
	// nonce, gasLimit uint64, gasPrice *big.Int, f StakeMsgFulfiller
	stx, _ := staking.NewStakingTransaction(0, 1e10, big.NewInt(10000), stakePayloadMaker)
	signed, _ := staking.Sign(stx, staking.NewEIP155Signer(stx.ChainID()), key)
	return signed
}
