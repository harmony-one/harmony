package core

import (
	"crypto/ecdsa"
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
