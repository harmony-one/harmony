package services

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/test/helpers"
)

func TestGetStakingOperationsFromCreateValidator(t *testing.T) {
	gasLimit := uint64(1e18)
	createValidatorTxDescription := stakingTypes.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		fromKey, _ := crypto.GenerateKey()
		return stakingTypes.DirectiveCreateValidator, stakingTypes.CreateValidator{
			Description:        createValidatorTxDescription,
			MinSelfDelegation:  tenOnes,
			MaxTotalDelegation: twelveOnes,
			ValidatorAddress:   crypto.PubkeyToAddress(fromKey.PublicKey),
			Amount:             tenOnes,
		}
	}, nil, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := helpers.GetMessageFromStakingTx(tx)
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

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    negativeBigValue(tenOnes),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetOperationsFromStakingTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestGetStakingOperationsFromDelegate(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	validatorKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	validatorAddr := crypto.PubkeyToAddress(validatorKey.PublicKey)
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveDelegate, stakingTypes.Delegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: validatorAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := helpers.GetMessageFromStakingTx(tx)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    negativeBigValue(tenOnes),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetOperationsFromStakingTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestGetStakingOperationsFromUndelegate(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	validatorKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	validatorAddr := crypto.PubkeyToAddress(validatorKey.PublicKey)
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveUndelegate, stakingTypes.Undelegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: validatorAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := helpers.GetMessageFromStakingTx(tx)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("0"),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetOperationsFromStakingTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestGetStakingOperationsFromCollectRewards(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveCollectRewards, stakingTypes.CollectRewards{
			DelegatorAddress: senderAddr,
		}
	}, senderKey, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := helpers.GetMessageFromStakingTx(tx)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
		Logs: []*hmytypes.Log{
			{
				Address: senderAddr,
				Topics:  []ethcommon.Hash{staking.CollectRewardsTopic},
				Data:    tenOnes.Bytes(),
			},
		},
	}
	refOperations := newNativeOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("%v", tenOnes.Uint64()),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetOperationsFromStakingTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestGetStakingOperationsFromEditValidator(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	tx, err := helpers.CreateTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveEditValidator, stakingTypes.EditValidator{
			ValidatorAddress: senderAddr,
		}
	}, senderKey, 0, gasLimit, gasPrice)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := helpers.GetMessageFromStakingTx(tx)
	if err != nil {
		t.Fatal(err.Error())
	}
	senderAccID, rosettaError := newAccountIdentifier(senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("0"),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetOperationsFromStakingTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestNewTransferNativeOperations(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
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
	receiverAccID, rosettaError := newAccountIdentifier(*tx.To())
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	startingOpID := &types.OperationIdentifier{}

	// Test failed 'contract' transaction
	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				{
					Index: startingOpID.Index,
				},
			},
			Type:    common.TransferNativeOperation,
			Status:  common.ContractFailureOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    negativeBigValue(tx.Value()),
				Currency: &common.NativeCurrency,
			},
		},
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 2,
			},
			RelatedOperations: []*types.OperationIdentifier{
				{
					Index: startingOpID.Index + 1,
				},
			},
			Type:    common.TransferNativeOperation,
			Status:  common.ContractFailureOperationStatus.Status,
			Account: receiverAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("%v", tx.Value().Uint64()),
				Currency: &common.NativeCurrency,
			},
		},
	}
	receipt := &hmytypes.Receipt{
		Status: hmytypes.ReceiptStatusFailed,
	}
	operations, rosettaError := newTransferNativeOperations(startingOpID, tx, receipt, senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}

	// Test successful plain / contract transaction
	refOperations[0].Status = common.SuccessOperationStatus.Status
	refOperations[1].Status = common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful
	operations, rosettaError = newTransferNativeOperations(startingOpID, tx, receipt, senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestNewCrossShardSenderTransferNativeOperations(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	tx, err := helpers.CreateTestTransaction(
		signer, 0, 1, 0, 1e18, gasPrice, big.NewInt(1), []byte("data-does-nothing"),
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
	startingOpID := &types.OperationIdentifier{}
	receiverAccID, rosettaError := newAccountIdentifier(*tx.To())
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	metadata, err := types.MarshalMap(common.CrossShardTransactionOperationMetadata{
		From: senderAccID,
		To:   receiverAccID,
	})
	if err != nil {
		t.Fatal(err)
	}

	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOpID,
			},
			Type:    common.CrossShardTransferNativeOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    negativeBigValue(tx.Value()),
				Currency: &common.NativeCurrency,
			},
			Metadata: metadata,
		},
	}
	operations, rosettaError := newCrossShardSenderTransferNativeOperations(startingOpID, tx, senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestNewContractCreationNativeOperations(t *testing.T) {
	dummyContractKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	chainID := params.TestChainConfig.ChainID
	signer := hmytypes.NewEIP155Signer(chainID)
	tx, err := helpers.CreateTestContractCreationTransaction(
		signer, 0, 0, 1e18, gasPrice, big.NewInt(0), []byte("test"),
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
	startingOpID := &types.OperationIdentifier{}

	// Test failed contract creation
	contractAddr := crypto.PubkeyToAddress(dummyContractKey.PublicKey)
	contractAddressID, rosettaError := newAccountIdentifier(contractAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOpID,
			},
			Type:    common.ContractCreationOperation,
			Status:  common.ContractFailureOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    negativeBigValue(tx.Value()),
				Currency: &common.NativeCurrency,
			},
			Metadata: map[string]interface{}{
				"contract_address": contractAddressID,
			},
		},
	}
	receipt := &hmytypes.Receipt{
		Status:          hmytypes.ReceiptStatusFailed,
		ContractAddress: contractAddr,
	}
	operations, rosettaError := newContractCreationNativeOperations(startingOpID, tx, receipt, senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}

	// Test successful contract creation
	refOperations[0].Status = common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful // Indicate successful tx
	operations, rosettaError = newContractCreationNativeOperations(startingOpID, tx, receipt, senderAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestNewNativeOperations(t *testing.T) {
	accountID := &types.AccountIdentifier{
		Address: "test-address",
	}
	gasFee := big.NewInt(int64(1e18))
	amount := &types.Amount{
		Value:    negativeBigValue(gasFee),
		Currency: &common.NativeCurrency,
	}

	ops := newNativeOperations(gasFee, accountID)
	if len(ops) != 1 {
		t.Fatalf("Expected new operations to be of length 1")
	}
	if !reflect.DeepEqual(ops[0].Account, accountID) {
		t.Errorf("Expected account ID to be %v not %v", accountID, ops[0].OperationIdentifier)
	}
	if !reflect.DeepEqual(ops[0].Amount, amount) {
		t.Errorf("Expected amount to be %v not %v", amount, ops[0].Amount)
	}
	if ops[0].Type != common.ExpendGasOperation {
		t.Errorf("Expected operation to be %v not %v", common.ExpendGasOperation, ops[0].Type)
	}
	if ops[0].OperationIdentifier.Index != 0 {
		t.Errorf("Expected operational ID to be of index 0")
	}
	if ops[0].Status != common.SuccessOperationStatus.Status {
		t.Errorf("Expected operation status to be %v", common.SuccessOperationStatus.Status)
	}
}
