package services

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/test/helpers"
)

// Invariant: A transaction can only contain 1 type of native operation(s) other than gas expenditure.
func assertNativeOperationTypeUniquenessInvariant(operations []*types.Operation) error {
	foundType := ""
	for _, op := range operations {
		if op.Type == common.ExpendGasOperation {
			continue
		}
		if foundType == "" {
			foundType = op.Type
		}
		if op.Type != foundType {
			return fmt.Errorf("found more than 1 type in given set of operations")
		}
	}
	return nil
}

// Note that this test only checks the general format of each type transaction on Harmony.
// The detailed operation checks for each type of transaction is done in separate unit tests.
func TestFormatTransactionIntegration(t *testing.T) {
	gasLimit := uint64(1e18)
	gasUsed := uint64(1e5)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	receiverKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}

	testFormatStakingTransaction(t, gasLimit, gasUsed, senderKey, receiverKey)
	testFormatPlainTransaction(t, gasLimit, gasUsed, senderKey, receiverKey)
	// Note that cross-shard receiver operations/transactions are formatted via
	// FormatCrossShardReceiverTransaction, thus, it is not tested here -- but tested on its own.
	testFormatCrossShardSenderTransaction(t, gasLimit, gasUsed, senderKey, receiverKey)
}

func testFormatStakingTransaction(
	t *testing.T, gasLimit, gasUsed uint64, senderKey, receiverKey *ecdsa.PrivateKey,
) {
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	receiverAddr := crypto.PubkeyToAddress(receiverKey.PublicKey)
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveDelegate, stakingTypes.Delegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: receiverAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful,
		GasUsed: gasUsed,
	}
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, []byte{})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	if len(rosettaTx.Operations) != 2 {
		t.Error("Expected 2 operations")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(rosettaTx.Operations); err != nil {
		t.Error(err)
	}
	if rosettaTx.TransactionIdentifier.Hash != tx.Hash().String() {
		t.Error("Invalid transaction")
	}
	if rosettaTx.Operations[0].Type != common.ExpendGasOperation {
		t.Error("Expected 1st operation to be gas type")
	}
	if rosettaTx.Operations[1].Type != tx.StakingType().String() {
		t.Error("Expected 2nd operation to be staking type")
	}
	if reflect.DeepEqual(rosettaTx.Operations[1].Metadata, map[string]interface{}{}) {
		t.Error("Expected staking operation to have some metadata")
	}
	if !reflect.DeepEqual(rosettaTx.Metadata, map[string]interface{}{}) {
		t.Error("Expected transaction to have no metadata")
	}
	if !reflect.DeepEqual(rosettaTx.Operations[0].Account, senderAccID) {
		t.Error("Expected sender to pay gas fee")
	}
}

func testFormatPlainTransaction(
	t *testing.T, gasLimit, gasUsed uint64, senderKey, receiverKey *ecdsa.PrivateKey,
) {
	// Note that post EIP-155 epoch singer is tested in detailed tests.
	signer := hmytypes.HomesteadSigner{}
	tx, err := helpers.CreateTestTransaction(
		signer, 0, 0, 0, 1e18, gasPrice, big.NewInt(1), []byte("test"),
	)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAddr, err := tx.SenderAddress()
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful,
		GasUsed: gasUsed,
	}
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, []byte{})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if len(rosettaTx.Operations) != 3 {
		t.Error("Expected 3 operations")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(rosettaTx.Operations); err != nil {
		t.Error(err)
	}
	if rosettaTx.TransactionIdentifier.Hash != tx.Hash().String() {
		t.Error("Invalid transaction")
	}
	if rosettaTx.Operations[0].Type != common.ExpendGasOperation {
		t.Error("Expected 1st operation to be gas")
	}
	if rosettaTx.Operations[1].Type != common.NativeTransferOperation {
		t.Error("Expected 2nd operation to transfer related")
	}
	if rosettaTx.Operations[1].Metadata != nil {
		t.Error("Expected 1st operation to have no metadata")
	}
	if rosettaTx.Operations[2].Metadata != nil {
		t.Error("Expected 2nd operation to have no metadata")
	}
	if reflect.DeepEqual(rosettaTx.Metadata, map[string]interface{}{}) {
		t.Error("Expected transaction to have some metadata")
	}
	if !reflect.DeepEqual(rosettaTx.Operations[0].Account, senderAccID) {
		t.Error("Expected sender to pay gas fee")
	}
}

func TestFormatGenesisTransaction(t *testing.T) {
	genesisSpec := getGenesisSpec(0)
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	for acc := range genesisSpec.Alloc {
		txID := getSpecialCaseTransactionIdentifier(testBlkHash, acc, SpecialGenesisTxID)
		tx, rosettaError := FormatGenesisTransaction(txID, acc, 0)
		if rosettaError != nil {
			t.Fatal(rosettaError)
		}
		if !reflect.DeepEqual(txID, tx.TransactionIdentifier) {
			t.Error("expected transaction ID of formatted tx to be same as requested")
		}
		if len(tx.Operations) != 1 {
			t.Error("expected exactly 1 operation")
		}
		if err := assertNativeOperationTypeUniquenessInvariant(tx.Operations); err != nil {
			t.Error(err)
		}
		if tx.Operations[0].OperationIdentifier.Index != 0 {
			t.Error("expected operational ID to be 0")
		}
		if tx.Operations[0].Type != common.GenesisFundsOperation {
			t.Error("expected operation to be genesis funds operations")
		}
		if tx.Operations[0].Status != common.SuccessOperationStatus.Status {
			t.Error("expected successful operation status")
		}
	}
}

func TestFormatPreStakingRewardTransactionSuccess(t *testing.T) {
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	testRewards := hmy.PreStakingBlockRewards{
		testAddr: big.NewInt(1),
	}
	refTxID := getSpecialCaseTransactionIdentifier(testBlkHash, testAddr, SpecialPreStakingRewardTxID)
	tx, rosettaError := FormatPreStakingRewardTransaction(refTxID, testRewards, testAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	if !reflect.DeepEqual(tx.TransactionIdentifier, refTxID) {
		t.Errorf("Expected TxID %v got %v", refTxID, tx.TransactionIdentifier)
	}
	if len(tx.Operations) != 1 {
		t.Fatal("Expected exactly 1 operation")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(tx.Operations); err != nil {
		t.Error(err)
	}
	if tx.Operations[0].OperationIdentifier.Index != 0 {
		t.Error("expected operational ID to be 0")
	}
	if tx.Operations[0].Type != common.PreStakingBlockRewardOperation {
		t.Error("expected operation type to be pre-staking era block rewards")
	}
	if tx.Operations[0].Status != common.SuccessOperationStatus.Status {
		t.Error("expected successful operation status")
	}
}

func TestFormatPreStakingRewardTransactionFail(t *testing.T) {
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	testRewards := hmy.PreStakingBlockRewards{
		FormatDefaultSenderAddress: big.NewInt(1),
	}
	testTxID := getSpecialCaseTransactionIdentifier(testBlkHash, testAddr, SpecialPreStakingRewardTxID)
	_, rosettaError := FormatPreStakingRewardTransaction(testTxID, testRewards, testAddr)
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if common.TransactionNotFoundError.Code != rosettaError.Code {
		t.Error("expected transaction not found error")
	}
}

func TestFormatUndelegationPayoutTransaction(t *testing.T) {
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	testPayout := big.NewInt(1e10)
	testDelegatorPayouts := hmy.UndelegationPayouts{
		testAddr: testPayout,
	}
	testBlockHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	testTxID := getSpecialCaseTransactionIdentifier(testBlockHash, testAddr, SpecialUndelegationPayoutTxID)

	tx, rosettaError := FormatUndelegationPayoutTransaction(testTxID, testDelegatorPayouts, testAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if len(tx.Operations) != 1 {
		t.Fatal("expected tx operations to be of length 1")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(tx.Operations); err != nil {
		t.Error(err)
	}
	if tx.Operations[0].OperationIdentifier.Index != 0 {
		t.Error("Expect first operation to be index 0")
	}
	if tx.Operations[0].Type != common.UndelegationPayoutOperation {
		t.Errorf("Expect operation type to be: %v", common.UndelegationPayoutOperation)
	}
	if tx.Operations[0].Status != common.SuccessOperationStatus.Status {
		t.Error("expected successful operation status")
	}
	if tx.Operations[0].Amount.Value != fmt.Sprintf("%v", testPayout) {
		t.Errorf("expect payout to be %v", testPayout)
	}

	_, rosettaError = FormatUndelegationPayoutTransaction(testTxID, hmy.UndelegationPayouts{}, testAddr)
	if rosettaError == nil {
		t.Fatal("Expect error for no payouts found")
	}
	if rosettaError.Code != common.TransactionNotFoundError.Code {
		t.Errorf("expect error code %v", common.TransactionNotFoundError.Code)
	}
}

func testFormatCrossShardSenderTransaction(
	t *testing.T, gasLimit, gasUsed uint64, senderKey, receiverKey *ecdsa.PrivateKey,
) {
	// Note that post EIP-155 epoch singer is tested in detailed tests.
	signer := hmytypes.HomesteadSigner{}
	tx, err := helpers.CreateTestTransaction(
		signer, 0, 1, 0, 1e18, gasPrice, big.NewInt(1), []byte("test"),
	)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAddr, err := tx.SenderAddress()
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful,
		GasUsed: gasUsed,
	}
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, []byte{})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if len(rosettaTx.Operations) != 2 {
		t.Error("Expected 2 operations")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(rosettaTx.Operations); err != nil {
		t.Error(err)
	}
	if rosettaTx.TransactionIdentifier.Hash != tx.Hash().String() {
		t.Error("Invalid transaction")
	}
	if rosettaTx.Operations[0].Type != common.ExpendGasOperation {
		t.Error("Expected 1st operation to be gas")
	}
	if rosettaTx.Operations[1].Type != common.NativeCrossShardTransferOperation {
		t.Error("Expected 2nd operation to cross-shard transfer related")
	}
	if reflect.DeepEqual(rosettaTx.Operations[1].Metadata, map[string]interface{}{}) {
		t.Error("Expected 1st operation to have metadata")
	}
	if reflect.DeepEqual(rosettaTx.Metadata, map[string]interface{}{}) {
		t.Error("Expected transaction to have some metadata")
	}
	if !reflect.DeepEqual(rosettaTx.Operations[0].Account, senderAccID) {
		t.Error("Expected sender to pay gas fee")
	}
}

func TestFormatCrossShardReceiverTransaction(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	tx, err := helpers.CreateTestTransaction(
		signer, 0, 1, 0, 1e18, gasPrice, big.NewInt(1), []byte{},
	)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAddr, err := tx.SenderAddress()
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	receiverAccID, rosettaError := newAccountIdentifier(*tx.To())
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	cxReceipt := &hmytypes.CXReceipt{
		TxHash:    tx.Hash(),
		From:      senderAddr,
		To:        tx.To(),
		ShardID:   0,
		ToShardID: 1,
		Amount:    tx.Value(),
	}
	opMetadata, err := types.MarshalMap(common.CrossShardTransactionOperationMetadata{
		From: senderAccID,
		To:   receiverAccID,
	})
	if err != nil {
		t.Error(err)
	}

	refCxID := &types.TransactionIdentifier{Hash: tx.Hash().String()}
	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0, // There is no gas expenditure for cross-shard payout
			},
			Type:    common.NativeCrossShardTransferOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: receiverAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("%v", tx.Value().Uint64()),
				Currency: &common.NativeCurrency,
			},
			Metadata: opMetadata,
		},
	}
	to := tx.ToShardID()
	from := tx.ShardID()
	refMetadata, err := types.MarshalMap(TransactionMetadata{
		CrossShardIdentifier: refCxID,
		ToShardID:            &to,
		FromShardID:          &from,
	})
	refRosettaTx := &types.Transaction{
		TransactionIdentifier: refCxID,
		Operations:            refOperations,
		Metadata:              refMetadata,
	}
	rosettaTx, rosettaError := FormatCrossShardReceiverTransaction(cxReceipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(rosettaTx, refRosettaTx) {
		t.Errorf("Expected transaction to be %v not %v", refRosettaTx, rosettaTx)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(rosettaTx.Operations); err != nil {
		t.Error(err)
	}
}
