package services

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/test/helpers"
)

func assertNativeOperationTypeUniquenessInvariant(operations []*types.Operation) error {
	foundType := ""
	for _, op := range operations {
		if _, ok := common.MutuallyExclusiveOperations[op.Type]; !ok {
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
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, &ContractInfo{}, true)
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
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, &ContractInfo{}, true)
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
	rosettaTx, rosettaError := FormatTransaction(tx, receipt, &ContractInfo{}, true)
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
			Status:  &common.SuccessOperationStatus.Status,
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
