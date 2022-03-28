package core

import (
	"bytes"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
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

type FakeBlockValidator struct{}

func (v *FakeBlockValidator) ValidateBody(block *types.Block) error {
	return nil
}
func (v *FakeBlockValidator) ValidateState(block *types.Block, statedb *state.DB, receipts types.Receipts, cxReceipts types.CXReceipts, usedGas uint64) error {
	return nil
}
func (v *FakeBlockValidator) ValidateHeader(block *types.Block, seal bool) error {
	return nil
}
func (v *FakeBlockValidator) ValidateHeaders(chain []*types.Block) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(chain))
	return abort, results
}
func (v *FakeBlockValidator) ValidateCXReceiptsProof(cxp *types.CXReceiptsProof) error {
	return nil
}

func TestClearValidatorWrappers(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, _, _, database := getTestEnvironment(*key)
	// turn off validation for inserted blocks
	chain.SetValidator(&FakeBlockValidator{})
	// move after header epoch so that header coinbase is directly used instead of shard state
	chain.chainConfig.StakingEpoch = big.NewInt(5)
	address := crypto.PubkeyToAddress(key.PublicKey)
	funds := new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(40000))
	gspec := Genesis{
		Config:  chain.Config(),
		Factory: blockfactory.ForTest,
		Alloc:   GenesisAlloc{address: {Balance: funds}},
		ShardID: 0,
	}
	// genesis error
	expectedError := "Cannot clear validator wrappers from genesis block"
	if err := chain.ClearValidatorWrappersAtBlock(0); err == nil || err.Error() != expectedError {
		t.Errorf("Expected %s but got %s", expectedError, err)
	}
	// missing block error
	expectedError = "Block 1 not found"
	if err := chain.ClearValidatorWrappersAtBlock(1); err == nil || err.Error() != expectedError {
		t.Errorf("Expected error %s but got %s", expectedError, err)
	}
	// actual test
	// add 2 blocks
	genesis := gspec.MustCommit(database)
	if err := chain.ResetWithGenesisBlock(genesis); err != nil {
		t.Fatalf("Could not add block 0 due to %s", err)
	}
	// make a new block with number 1
	header := gspec.Factory.NewHeader(genesis.Epoch()).With().
		ParentHash(genesis.Hash()).
		Coinbase(address).
		Number(new(big.Int).Add(genesis.Number(), common.Big1)).
		Time(big.NewInt(10)).
		GasLimit(params.TestGenesisGasLimit).
		Header()
	// add a validator wrapper for demonstration (this is set to nil later)
	stx1 := signedCreateValidatorStakingTxn(key)
	stxs := []*staking.StakingTransaction{stx1}
	block1 := types.NewBlock(header, nil, []*types.Receipt{types.NewReceipt([]byte{}, false, 0)}, nil, nil, stxs)
	if _, err := chain.InsertChain(types.Blocks{
		block1,
	}, false); err != nil {
		t.Fatalf("Could not add block 1 due to %s", err)
	}
	originalRoot := chain.GetBlockByNumber(1).Root()
	if err := chain.ClearValidatorWrappersAtBlock(1); err != nil {
		t.Errorf("Expected no error but got %s", err)
	}
	finalRoot := chain.GetBlockByNumber(1).Root()
	if !bytes.Equal(originalRoot.Bytes(), finalRoot.Bytes()) {
		t.Errorf("Expected root %s but got %s", originalRoot.Hex(), finalRoot.Hex())
	}
	// TODO: make a chain here with CreateValidator transaction
	// and run the test outlined in ClearValidatorWrappersAtBlock
	// when I tried this, it was full of errors so I could not make a chain
	// with a staking transaction
}

func signedCreateValidatorStakingTxn(key *ecdsa.PrivateKey) *staking.StakingTransaction {
	stakePayloadMaker := func() (staking.Directive, interface{}) {
		return staking.DirectiveCreateValidator, sampleCreateValidator(*key)
	}
	stx, _ := staking.NewStakingTransaction(0, 1e7, big.NewInt(1), stakePayloadMaker)
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
