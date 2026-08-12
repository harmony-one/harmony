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

// TestIsSpentIgnoresMutatedMerkleProofIdentity guards against replaying a
// genuine, already-applied CXReceiptsProof by mutating the unauthenticated
// MerkleProof.ShardID/BlockNum while keeping the same signed Header: the
// spent-marker must be keyed off the Header, which cannot be altered without
// invalidating the commit signature, not off MerkleProof fields that
// ValidateCXReceiptsProof only binds to the Header from
// IsCXMerkleProofReplayFixEpoch onward.
func TestIsSpentIgnoresMutatedMerkleProofIdentity(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, header, database := getTestEnvironment(*key)

	header = header.With().ShardID(1).Number(big.NewInt(42)).Header()

	original := &types.CXReceiptsProof{
		Header: header,
		MerkleProof: &types.CXMerkleProof{
			ShardID:  1,
			BlockNum: big.NewInt(42),
		},
	}

	batch := database.NewBatch()
	if err := chain.WriteCXReceiptsProofSpent(batch, []*types.CXReceiptsProof{original}); err != nil {
		t.Fatalf("WriteCXReceiptsProofSpent failed: %v", err)
	}
	if err := batch.Write(); err != nil {
		t.Fatalf("batch.Write failed: %v", err)
	}

	if !chain.IsSpent(original) {
		t.Fatal("expected original proof to be marked spent")
	}

	replay := &types.CXReceiptsProof{
		Header: header, // same genuine, signed header
		MerkleProof: &types.CXMerkleProof{
			ShardID:  99,               // mutated, unauthenticated
			BlockNum: big.NewInt(9999), // mutated, unauthenticated
		},
	}
	if !chain.IsSpent(replay) {
		t.Fatal("expected replay with mutated MerkleProof identity to be detected as already spent")
	}
}

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
