package services

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/harmony-one/harmony/hmy/tracers"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy"
	internalCommon "github.com/harmony-one/harmony/internal/common"
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
	refOperations := newNativeOperationsWithGas(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             senderAccID,
		Amount: &types.Amount{
			Value:    negativeBigValue(tenOnes),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetNativeOperationsFromStakingTransaction(tx, receipt, true)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		operationsRaw, _ := json.Marshal(operations)
		refOperationsRaw, _ := json.Marshal(refOperations)
		t.Errorf("Expected operations to be:\n %v\n not\n %v", string(operationsRaw), string(refOperationsRaw))
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}
}

func TestGetSideEffectOperationsFromValueMap(t *testing.T) {
	testAcc1 := crypto.PubkeyToAddress(internalCommon.MustGeneratePrivateKey().PublicKey)
	testAcc2 := crypto.PubkeyToAddress(internalCommon.MustGeneratePrivateKey().PublicKey)
	testAmount1 := big.NewInt(12000)
	testAmount2 := big.NewInt(10000)
	testPayouts := map[ethcommon.Address]*big.Int{
		testAcc1: testAmount1,
		testAcc2: testAmount2,
	}
	testType := common.GenesisFundsOperation
	ops, rosettaError := getSideEffectOperationsFromValueMap(testPayouts, testType, nil)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	for i, op := range ops {
		if int64(i) != op.OperationIdentifier.Index {
			t.Errorf("expected operation %v to have operation index %v", i, i)
		}
		address, err := getAddress(op.Account)
		if err != nil {
			t.Fatal(err)
		}
		if value, ok := testPayouts[address]; !ok {
			t.Errorf("operation %v has address that is not in test map", i)
		} else if value.String() != op.Amount.Value {
			t.Errorf("operation %v has wrong value (%v != %v)", i, value.String(), op.Amount.Value)
		}
		if op.Type != testType {
			t.Errorf("operation %v has wrong type", i)
		}
		if len(op.RelatedOperations) != 0 {
			t.Errorf("operation %v has related operations", i)
		}
		if types.Hash(op.Amount.Currency) != common.NativeCurrencyHash {
			t.Errorf("operation %v has wrong currency", i)
		}
	}
	if err := assertNativeOperationTypeUniquenessInvariant(ops); err != nil {
		t.Error(err)
	}

	testStartingIndex := int64(12)
	ops, rosettaError = getSideEffectOperationsFromValueMap(testPayouts, testType, &testStartingIndex)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	for i, op := range ops {
		if int64(i)+testStartingIndex != op.OperationIdentifier.Index {
			t.Errorf("expected operation %v to have operation index %v", i, int64(i)+testStartingIndex)
		}
		address, err := getAddress(op.Account)
		if err != nil {
			t.Fatal(err)
		}
		if value, ok := testPayouts[address]; !ok {
			t.Errorf("operation %v has address that is not in test map", i)
		} else if value.String() != op.Amount.Value {
			t.Errorf("operation %v has wrong value (%v != %v)", i, value.String(), op.Amount.Value)
		}
		if op.Type != testType {
			t.Errorf("operation %v has wrong type", i)
		}
		if len(op.RelatedOperations) != 0 {
			t.Errorf("operation %v has related operations", i)
		}
		if types.Hash(op.Amount.Currency) != common.NativeCurrencyHash {
			t.Errorf("operation %v has wrong currency", i)
		}
	}
	if err := assertNativeOperationTypeUniquenessInvariant(ops); err != nil {
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

	senderAccIDWithSubAccount, rosettaError := newAccountIdentifierWithSubAccount(senderAddr, validatorAddr,
		map[string]interface{}{SubAccountMetadataKey: Delegation})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperationsWithGas(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             senderAccID,
		Amount: &types.Amount{
			Value:    negativeBigValue(tenOnes),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 2},
		RelatedOperations: []*types.OperationIdentifier{
			{
				Index: 1,
			},
		},
		Type:    tx.StakingType().String(),
		Status:  &common.SuccessOperationStatus.Status,
		Account: senderAccIDWithSubAccount,
		Amount: &types.Amount{
			Value:    tenOnes.String(),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetNativeOperationsFromStakingTransaction(tx, receipt, true)
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

func TestGetSideEffectOperationsFromUndelegationPayouts(t *testing.T) {
	startingOperationIndex := int64(0)
	undelegationPayouts := hmy.NewUndelegationPayouts()
	delegator := ethcommon.HexToAddress("0xB5f440B5c6215eEDc1b2E12b4b964fa31f7afa7d")
	validator := ethcommon.HexToAddress("0x3b8DE43c8F30D3C387840681FED67783f93f1F94")
	undelegationPayouts.SetPayoutByDelegatorAddrAndValidatorAddr(delegator, validator, new(big.Int).SetInt64(4000))
	operations, err := getSideEffectOperationsFromUndelegationPayouts(undelegationPayouts, &startingOperationIndex)
	if err != nil {
		t.Fatal(err)
	}
	receiverAccId, err := newAccountIdentifier(delegator)
	if err != nil {
		t.Fatal(err)
	}
	reveiverSubAccId1, err := newAccountIdentifierWithSubAccount(delegator, validator, map[string]interface{}{
		SubAccountMetadataKey: UnDelegation,
	})
	if err != nil {
		t.Fatal(err)
	}

	refOperations := []*types.Operation{}
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 0},
		Type:                UndelegationPayout,
		Status:              &common.SuccessOperationStatus.Status,
		Account:             receiverAccId,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("9000"),
			Currency: &common.NativeCurrency,
		},
	}, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			&types.OperationIdentifier{
				Index: 0,
			},
		},
		Type:    UndelegationPayout,
		Status:  &common.SuccessOperationStatus.Status,
		Account: reveiverSubAccId1,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("-9000"),
			Currency: &common.NativeCurrency,
		},
	})

	if len(refOperations) != len(operations) {
		t.Errorf("Expected operation to be %d not %d", len(refOperations), len(operations))
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
	senderAccID, rosettaError := newAccountIdentifierWithSubAccount(senderAddr, validatorAddr, map[string]interface{}{
		SubAccountMetadataKey: Delegation,
	})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	receiverAccId, rosettaError := newAccountIdentifierWithSubAccount(senderAddr, validatorAddr, map[string]interface{}{
		SubAccountMetadataKey: UnDelegation,
	})
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	gasUsed := uint64(1e5)
	gasFee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	receipt := &hmytypes.Receipt{
		Status:  hmytypes.ReceiptStatusSuccessful, // Failed staking transaction are never saved on-chain
		GasUsed: gasUsed,
	}
	refOperations := newNativeOperationsWithGas(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("%v", negativeBigValue(tenOnes)),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 2},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             receiverAccId,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("%v", tenOnes.Uint64()),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetNativeOperationsFromStakingTransaction(tx, receipt, true)
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
	refOperations := newNativeOperationsWithGas(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("%v", tenOnes.Uint64()),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetNativeOperationsFromStakingTransaction(tx, receipt, true)
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
	refOperations := newNativeOperationsWithGas(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		Type:                tx.StakingType().String(),
		Status:              &common.SuccessOperationStatus.Status,
		Account:             senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("0"),
			Currency: &common.NativeCurrency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := GetNativeOperationsFromStakingTransaction(tx, receipt, true)
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

func TestGetBasicTransferOperations(t *testing.T) {
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
			Type:    common.NativeTransferOperation,
			Status:  &common.ContractFailureOperationStatus.Status,
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
			Type:    common.NativeTransferOperation,
			Status:  &common.ContractFailureOperationStatus.Status,
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
	opIndex := startingOpID.Index + 1
	operations, rosettaError := getBasicTransferNativeOperations(tx, receipt, senderAddr, tx.To(), &opIndex)
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
	refOperations[0].Status = &common.SuccessOperationStatus.Status
	refOperations[1].Status = &common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful
	operations, rosettaError = getBasicTransferNativeOperations(tx, receipt, senderAddr, tx.To(), &opIndex)
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

func TestGetCrossShardSenderTransferNativeOperations(t *testing.T) {
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

	refReceipt := &hmytypes.Receipt{
		PostState: nil,
		Status:    1,
		GasUsed:   params.TxGas * 3, // somme arb number > TxGas
	}

	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 1,
			},
			Type:    common.NativeCrossShardTransferOperation,
			Status:  &common.SuccessOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    negativeBigValue(tx.Value()),
				Currency: &common.NativeCurrency,
			},
			Metadata: metadata,
		},
	}
	opIndex := startingOpID.Index + 1
	operations, rosettaError := getCrossShardSenderTransferNativeOperations(tx, refReceipt, senderAddr, &opIndex)
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

var (
	getAddressPtr = func(address ethcommon.Address) *ethcommon.Address {
		return &address
	}
	testExecResultForInternalTx = []*tracers.RosettaLogItem{
		{
			IsSuccess: true,
			Reverted:  false,
			OP:        vm.DUP9,
			From: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x2a44f609f860d4ff8835f9ec1d9b1acdae1fd9cb")),
			},
			To: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x4c4fde977fbbe722cddf5719d7edd488510be16a")),
			},
			Value: big.NewInt(1),
		},
		{
			IsSuccess: true,
			Reverted:  false,
			OP:        vm.CALL,
			From: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x2a44f609f860d4ff8835f9ec1d9b1acdae1fd9cb")),
			},
			To: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x4c4fde977fbbe722cddf5719d7edd488510be16a")),
			},
			Value: big.NewInt(1),
		},
		{
			IsSuccess: true,
			Reverted:  false,
			OP:        vm.SWAP4,
			From: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x2a44f609f860d4ff8835f9ec1d9b1acdae1fd9cb")),
			},
			To: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x4c4fde977fbbe722cddf5719d7edd488510be16a")),
			},
			Value: big.NewInt(1),
		},
		{
			IsSuccess: true,
			Reverted:  false,
			OP:        vm.CALLCODE,
			From: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x2a44f609f860d4ff8835f9ec1d9b1acdae1fd9cb")),
			},
			To: &vm.RosettaLogAddressItem{
				Account: getAddressPtr(ethcommon.HexToAddress("0x4c4fde977fbbe722cddf5719d7edd488510be16a")),
			},
			Value: big.NewInt(1),
		},
	}
	testExecResultForInternalTxValueSum = uint64(
		4,
	)
)

func TestGetContractInternalTransferNativeOperations(t *testing.T) {
	refStatus := common.SuccessOperationStatus.Status
	refStartingIndex := int64(23)
	baseValidation := func(ops []*types.Operation, expectedValueSum uint64) {
		prevIndex := int64(-1)
		valueSum := int64(0)
		absValueSum := uint64(0)
		for i, op := range ops {
			if op.OperationIdentifier.Index <= prevIndex {
				t.Errorf("expect prev index (%v) < curr index (%v) for op %v",
					prevIndex, op.OperationIdentifier.Index, i,
				)
			}
			prevIndex = op.OperationIdentifier.Index
			if op.Status == nil || *op.Status != refStatus {
				t.Errorf("wrong status for op %v", i)
			}
			if op.Type != common.NativeTransferOperation {
				t.Errorf("wrong operation type for op %v", i)
			}
			if types.Hash(op.Amount.Currency) != common.NativeCurrencyHash {
				t.Errorf("wrong currency for op %v", i)
			}
			val, err := types.AmountValue(op.Amount)
			if err != nil {
				t.Error(err)
			}
			valueSum += val.Int64()
			absValueSum += val.Abs(val).Uint64()
		}

		if valueSum != 0 {
			t.Errorf("expected sum of all non-gas values to be 0")
		}
		if expectedValueSum*2 != absValueSum {
			t.Errorf("sum of all positive values of operations do not match execpted sum of values")
		}
	}

	testOps, rosettaError := getContractInternalTransferNativeOperations(
		testExecResultForInternalTx, refStatus, &refStartingIndex,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	baseValidation(testOps, testExecResultForInternalTxValueSum)
	if len(testOps) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if testOps[0].OperationIdentifier.Index != refStartingIndex {
		t.Errorf("expected starting index to be %v", refStartingIndex)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}

	testOps, rosettaError = getContractInternalTransferNativeOperations(
		testExecResultForInternalTx, refStatus, nil,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	baseValidation(testOps, testExecResultForInternalTxValueSum)
	if len(testOps) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if testOps[0].OperationIdentifier.Index != 0 {
		t.Errorf("expected starting index to be 0")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}

	testOps, rosettaError = getContractInternalTransferNativeOperations(
		nil, refStatus, nil,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	if len(testOps) != 0 {
		t.Errorf("expected len 0 test operations for nil execution result")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}

	testOps, rosettaError = getContractInternalTransferNativeOperations(
		[]*tracers.RosettaLogItem{}, refStatus, nil,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	if len(testOps) != 0 {
		t.Errorf("expected len 0 test operations for nil struct logs")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}
}

func TestGetContractTransferNativeOperations(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	refTxValue := big.NewInt(1)
	refTx, err := helpers.CreateTestTransaction(
		signer, 0, 0, 0, 1e18, gasPrice, refTxValue, []byte("blah-blah-blah"),
	)
	if err != nil {
		t.Fatal(err.Error())
	}
	refSenderAddr, err := refTx.SenderAddress()
	if err != nil {
		t.Fatal(err.Error())
	}
	refStatus := common.SuccessOperationStatus.Status
	refStartingIndex := int64(23)
	refReceipt := &hmytypes.Receipt{
		PostState: nil,
		Status:    1,
		GasUsed:   params.TxGas * 3, // somme arb number > TxGas
	}
	baseValidation := func(ops []*types.Operation, expectedValueSum uint64) {
		prevIndex := int64(-1)
		valueSum := int64(0)
		absValueSum := uint64(0)
		for i, op := range ops {
			if op.OperationIdentifier.Index <= prevIndex {
				t.Errorf("expect prev index (%v) < curr index (%v) for op %v",
					prevIndex, op.OperationIdentifier.Index, i,
				)
			}
			prevIndex = op.OperationIdentifier.Index
			if op.Status == nil || *op.Status != refStatus {
				t.Errorf("wrong status for op %v", i)
			}
			if types.Hash(op.Amount.Currency) != common.NativeCurrencyHash {
				t.Errorf("wrong currency for op %v", i)
			}
			if op.Type == common.ExpendGasOperation {
				continue
			}
			if op.Type != common.NativeTransferOperation {
				t.Errorf("wrong operation type for op %v", i)
			}
			val, err := types.AmountValue(op.Amount)
			if err != nil {
				t.Error(err)
			}
			valueSum += val.Int64()
			absValueSum += val.Abs(val).Uint64()
		}

		if valueSum != 0 {
			t.Errorf("expected sum of all non-gas values to be 0")
		}
		if expectedValueSum*2 != absValueSum {
			t.Errorf("sum of all positive values of operations do not match execpted sum of values")
		}
	}

	testOps, rosettaError := getContractTransferNativeOperations(
		refTx, refReceipt, refSenderAddr, refTx.To(),
		&ContractInfo{ExecutionResult: testExecResultForInternalTx}, &refStartingIndex,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	baseValidation(testOps, testExecResultForInternalTxValueSum+refTxValue.Uint64())
	if len(testOps) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if testOps[0].OperationIdentifier.Index != refStartingIndex {
		t.Errorf("expected starting index to be %v", refStartingIndex)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}

	testOps, rosettaError = getContractTransferNativeOperations(
		refTx, refReceipt, refSenderAddr, refTx.To(),
		&ContractInfo{ExecutionResult: testExecResultForInternalTx}, nil,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	baseValidation(testOps, testExecResultForInternalTxValueSum+refTxValue.Uint64())
	if len(testOps) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if testOps[0].OperationIdentifier.Index != 0 {
		t.Errorf("expected starting index to be 0")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}

	testOps, rosettaError = getContractTransferNativeOperations(
		refTx, refReceipt, refSenderAddr, refTx.To(),
		&ContractInfo{}, nil,
	)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	baseValidation(testOps, refTxValue.Uint64())
	if len(testOps) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if testOps[0].OperationIdentifier.Index != 0 {
		t.Errorf("expected starting index to be 0")
	}
	if len(testOps) > 3 {
		t.Errorf("expect at most 3 operations for nil ExecutionResult")
	}
	if err := assertNativeOperationTypeUniquenessInvariant(testOps); err != nil {
		t.Error(err)
	}
}

func TestGetContractCreationNativeOperations(t *testing.T) {
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
			Type:    common.ContractCreationOperation,
			Status:  &common.ContractFailureOperationStatus.Status,
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
			Type:    common.ContractCreationOperation,
			Status:  &common.ContractFailureOperationStatus.Status,
			Account: contractAddressID,
			Amount: &types.Amount{
				Value:    tx.Value().String(),
				Currency: &common.NativeCurrency,
			},
		},
	}
	receipt := &hmytypes.Receipt{
		Status:          hmytypes.ReceiptStatusFailed,
		ContractAddress: contractAddr,
	}
	opIndex := startingOpID.Index + 1
	operations, rosettaError := getContractCreationNativeOperations(tx, receipt, senderAddr, &ContractInfo{}, &opIndex)
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
	refOperations[0].Status = &common.SuccessOperationStatus.Status
	refOperations[1].Status = &common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful // Indicate successful tx
	operations, rosettaError = getContractCreationNativeOperations(tx, receipt, senderAddr, &ContractInfo{}, &opIndex)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
	if err := assertNativeOperationTypeUniquenessInvariant(operations); err != nil {
		t.Error(err)
	}

	traceValidation := func(ops []*types.Operation, expectedValueSum uint64) {
		prevIndex := int64(-1)
		valueSum := int64(0)
		absValueSum := uint64(0)
		for i, op := range ops {
			if op.OperationIdentifier.Index <= prevIndex {
				t.Errorf("expect prev index (%v) < curr index (%v) for op %v",
					prevIndex, op.OperationIdentifier.Index, i,
				)
			}
			prevIndex = op.OperationIdentifier.Index
			if *op.Status != *refOperations[0].Status {
				t.Errorf("wrong status for op %v", i)
			}
			if types.Hash(op.Amount.Currency) != common.NativeCurrencyHash {
				t.Errorf("wrong currency for op %v", i)
			}
			if op.Type == common.ExpendGasOperation || op.Type == common.ContractCreationOperation {
				continue
			}
			if op.Type != common.NativeTransferOperation {
				t.Errorf("wrong operation type for op %v", i)
			}
			val, err := types.AmountValue(op.Amount)
			if err != nil {
				t.Error(err)
			}
			valueSum += val.Int64()
			absValueSum += val.Abs(val).Uint64()
		}

		if valueSum != 0 {
			t.Errorf("expected sum of all non-gas values to be 0")
		}
		if expectedValueSum*2 != absValueSum {
			t.Errorf("sum of all positive values of operations do not match execpted sum of values")
		}
	}
	operations, rosettaError = getContractCreationNativeOperations(
		tx, receipt, senderAddr, &ContractInfo{ExecutionResult: testExecResultForInternalTx}, &opIndex,
	)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	traceValidation(operations, testExecResultForInternalTxValueSum)
	if len(operations) == 0 {
		t.Errorf("expect atleast 1 operation")
	}
	if operations[0].OperationIdentifier.Index != opIndex {
		t.Errorf("expect first operation to be %v", opIndex)
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

	ops := newNativeOperationsWithGas(gasFee, accountID)
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
	if ops[0].Status == nil {
		t.Error("Expected operation status should not be nil")
	}
	if *ops[0].Status != common.SuccessOperationStatus.Status {
		t.Errorf("Expected operation status to be %v", common.SuccessOperationStatus.Status)
	}
}
