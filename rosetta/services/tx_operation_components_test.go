package services

import (
	"math/big"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/crypto/bls"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/rosetta/common"
)

func TestGetContractCreationOperationComponents(t *testing.T) {
	refAmount := &types.Amount{
		Value:    "-12000",
		Currency: &common.NativeCurrency,
	}
	refKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	// test valid operations
	refOperation := &types.Operation{
		Type:    common.ContractCreationOperation,
		Amount:  refAmount,
		Account: refFrom,
	}
	testComponents, rosettaError := getContractCreationOperationComponents(refOperation)
	if rosettaError != nil {
		t.Error(rosettaError)
	}
	if testComponents.Type != refOperation.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expect same sender")
	}
	if testComponents.Amount.Cmp(big.NewInt(12000)) != 0 {
		t.Error("expected amount to be absolute value of reference amount")
	}

	// test nil amount
	_, rosettaError = getContractCreationOperationComponents(&types.Operation{
		Type:    common.ContractCreationOperation,
		Amount:  nil,
		Account: refFrom,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test positive amount
	_, rosettaError = getContractCreationOperationComponents(&types.Operation{
		Type: common.ContractCreationOperation,
		Amount: &types.Amount{
			Value:    "12000",
			Currency: &common.NativeCurrency,
		},
		Account: refFrom,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test different/unsupported currency
	_, rosettaError = getContractCreationOperationComponents(&types.Operation{
		Type: common.ContractCreationOperation,
		Amount: &types.Amount{
			Value: "-12000",
			Currency: &types.Currency{
				Symbol:   "bad",
				Decimals: 9,
			},
		},
		Account: refFrom,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil account
	_, rosettaError = getContractCreationOperationComponents(&types.Operation{
		Type:    common.ContractCreationOperation,
		Amount:  refAmount,
		Account: nil,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getContractCreationOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestGetCrossShardOperationComponents(t *testing.T) {
	refAmount := &types.Amount{
		Value:    "-12000",
		Currency: &common.NativeCurrency,
	}
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refToKey := internalCommon.MustGeneratePrivateKey()
	refTo, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refToKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refMetadata := common.CrossShardTransactionOperationMetadata{
		From: refFrom,
		To:   refTo,
	}
	refMetadataMap, err := types.MarshalMap(refMetadata)
	if err != nil {
		t.Fatal(err)
	}

	// test valid operations
	refOperation := &types.Operation{
		Type:     common.NativeCrossShardTransferOperation,
		Amount:   refAmount,
		Account:  refFrom,
		Metadata: refMetadataMap,
	}
	testComponents, rosettaError := getCrossShardOperationComponents(refOperation)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperation.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expect same sender")
	}
	if testComponents.To == nil || types.Hash(testComponents.To) != types.Hash(refTo) {
		t.Error("expected same sender")
	}
	if testComponents.Amount.Cmp(big.NewInt(12000)) != 0 {
		t.Error("expected amount to be absolute value of reference amount")
	}

	// test nil amount
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type:     common.NativeCrossShardTransferOperation,
		Amount:   nil,
		Account:  refFrom,
		Metadata: refMetadataMap,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test positive amount
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type: common.NativeCrossShardTransferOperation,
		Amount: &types.Amount{
			Value:    "12000",
			Currency: &common.NativeCurrency,
		},
		Account:  refFrom,
		Metadata: refMetadataMap,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test different/unsupported currency
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type: common.NativeCrossShardTransferOperation,
		Amount: &types.Amount{
			Value: "-12000",
			Currency: &types.Currency{
				Symbol:   "bad",
				Decimals: 9,
			},
		},
		Account:  refFrom,
		Metadata: refMetadataMap,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil account
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type:     common.NativeCrossShardTransferOperation,
		Amount:   refAmount,
		Account:  nil,
		Metadata: refMetadataMap,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test no metadata
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type:    common.NativeCrossShardTransferOperation,
		Amount:  refAmount,
		Account: refFrom,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test bad metadata
	randomKey := internalCommon.MustGeneratePrivateKey()
	randomID, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(randomKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	badMetadata := common.CrossShardTransactionOperationMetadata{
		From: randomID,
		To:   refTo,
	}
	badMetadataMap, err := types.MarshalMap(badMetadata)
	if err != nil {
		t.Fatal(err)
	}
	_, rosettaError = getCrossShardOperationComponents(&types.Operation{
		Type:     common.NativeCrossShardTransferOperation,
		Amount:   refAmount,
		Account:  refFrom,
		Metadata: badMetadataMap,
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getCrossShardOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestGetTransferOperationComponents(t *testing.T) {
	refFromAmount := &types.Amount{
		Value:    "-12000",
		Currency: &common.NativeCurrency,
	}
	refToAmount := &types.Amount{
		Value:    "12000",
		Currency: &common.NativeCurrency,
	}
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refToKey := internalCommon.MustGeneratePrivateKey()
	refTo, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refToKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	// test valid operations
	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0,
			},
			Type:    common.NativeTransferOperation,
			Amount:  refFromAmount,
			Account: refFrom,
		},
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:    common.NativeTransferOperation,
			Amount:  refToAmount,
			Account: refTo,
		},
	}
	testComponents, rosettaError := getTransferOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations[0].Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expect same sender")
	}
	if testComponents.To == nil || types.Hash(testComponents.To) != types.Hash(refTo) {
		t.Error("expected same sender")
	}
	if testComponents.Amount.Cmp(big.NewInt(12000)) != 0 {
		t.Error("expected amount to be absolute value of reference amount")
	}

	// test valid operations flipped
	refOperations[0].Amount = refToAmount
	refOperations[0].Account = refTo
	refOperations[1].Amount = refFromAmount
	refOperations[1].Account = refFrom
	testComponents, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations[0].Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expect same sender")
	}
	if testComponents.To == nil || types.Hash(testComponents.To) != types.Hash(refTo) {
		t.Error("expected same sender")
	}
	if testComponents.Amount.Cmp(big.NewInt(12000)) != 0 {
		t.Error("expected amount to be absolute value of reference amount")
	}

	// test no sender
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = nil
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test no receiver
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = nil
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid operation
	refOperations[0].Type = common.ExpendGasOperation
	refOperations[1].Type = common.NativeTransferOperation
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid operation sender
	refOperations[0].Type = common.NativeTransferOperation
	refOperations[1].Type = common.ExpendGasOperation
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
	refOperations[1].Type = common.NativeTransferOperation

	// test nil amount
	refOperations[0].Amount = nil
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil amount sender
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = refFrom
	refOperations[1].Amount = nil
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test uneven amount
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = refFrom
	refOperations[1].Amount = &types.Amount{
		Value:    "0",
		Currency: &common.NativeCurrency,
	}
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test uneven amount sender
	refOperations[0].Amount = &types.Amount{
		Value:    "0",
		Currency: &common.NativeCurrency,
	}
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil amount
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = refFrom
	refOperations[1].Amount = nil
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil amount sender
	refOperations[0].Amount = nil
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid currency
	refOperations[0].Amount = refFromAmount
	refOperations[0].Amount.Currency = &types.Currency{
		Symbol:   "bad",
		Decimals: 9,
	}
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
	refOperations[0].Amount.Currency = &common.NativeCurrency

	// test invalid currency sender
	refOperations[0].Amount = refFromAmount
	refOperations[0].Account = refFrom
	refOperations[1].Amount = refToAmount
	refOperations[1].Amount.Currency = &types.Currency{
		Symbol:   "bad",
		Decimals: 9,
	}
	refOperations[1].Account = refTo
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
	refOperations[1].Amount.Currency = &common.NativeCurrency

	// test invalid related operation
	refOperations[1].RelatedOperations[0].Index = 2
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
	refOperations[1].RelatedOperations[0].Index = 0

	// test cyclic related operation
	refOperations[0].RelatedOperations = []*types.OperationIdentifier{
		{
			Index: 1,
		},
	}
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// Test invalid related operation sender
	refOperations[1].RelatedOperations = nil
	refOperations[0].RelatedOperations[0].Index = 3
	_, rosettaError = getTransferOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// Test nil operations
	_, rosettaError = getTransferOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestCreateValidatorOperationComponents(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	validatorKey := internalCommon.MustGeneratePrivateKey()
	validatorAddr := crypto.PubkeyToAddress(validatorKey.PublicKey)
	validatorBech32Addr, _ := internalCommon.AddressToBech32(validatorAddr)
	blsKey := bls.RandPrivateKey()
	var serializedPubKey bls.SerializedPublicKey
	copy(serializedPubKey[:], blsKey.GetPublicKey().Serialize())

	// test valid operations
	refOperations := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.CreateValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorBech32Addr,
			"commissionRate":     new(big.Int).SetInt64(10),
			"maxCommissionRate":  new(big.Int).SetInt64(90),
			"maxChangeRate":      new(big.Int).SetInt64(2),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"amount":             new(big.Int).SetInt64(100000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	testComponents, rosettaError := getCreateValidatorOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expected same sender")
	}
	if testComponents.Amount != nil {
		t.Error("expected nil amount")
	}
	if testComponents.To != nil {
		t.Error("expected nil to")
	}

	// test invalid operation

	// test nil account
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.CreateValidatorOperation,
		Account: nil,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorAddr,
			"commissionRate":     new(big.Int).SetInt64(10),
			"maxCommissionRate":  new(big.Int).SetInt64(90),
			"maxChangeRate":      new(big.Int).SetInt64(2),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"amount":             new(big.Int).SetInt64(100000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	_, rosettaError = getCreateValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getCreateValidatorOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid validator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.CreateValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"commissionRate":     new(big.Int).SetInt64(10),
			"maxCommissionRate":  new(big.Int).SetInt64(90),
			"maxChangeRate":      new(big.Int).SetInt64(2),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"amount":             new(big.Int).SetInt64(100000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}
	_, rosettaError = getCreateValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid commission rate
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.CreateValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorAddr,
			"maxCommissionRate":  new(big.Int).SetInt64(90),
			"maxChangeRate":      new(big.Int).SetInt64(2),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"amount":             new(big.Int).SetInt64(100000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	_, rosettaError = getCreateValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestEditValidatorOperationComponents(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	validatorKey := internalCommon.MustGeneratePrivateKey()
	validatorAddr := crypto.PubkeyToAddress(validatorKey.PublicKey)
	validatorBech32Addr, _ := internalCommon.AddressToBech32(validatorAddr)

	// test valid operations
	refOperations := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.EditValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorBech32Addr,
			"commissionRate":     new(big.Int).SetInt64(10),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	testComponents, rosettaError := getEditValidatorOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expected same sender")
	}
	if testComponents.Amount != nil {
		t.Error("expected nil amount")
	}
	if testComponents.To != nil {
		t.Error("expected nil to")
	}

	// test invalid operation

	// test nil account
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.EditValidatorOperation,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorAddr,
			"commissionRate":     new(big.Int).SetInt64(10),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	_, rosettaError = getEditValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getEditValidatorOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid validator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.EditValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"commissionRate":     new(big.Int).SetInt64(10),
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}

	_, rosettaError = getEditValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid commission rate
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.EditValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"validatorAddress":   validatorAddr,
			"minSelfDelegation":  new(big.Int).SetInt64(10000),
			"maxTotalDelegation": new(big.Int).SetInt64(10000000),
			"name":               "Test validator",
			"website":            "https://test.website.com",
			"identity":           "test identity",
			"securityContact":    "security contact",
			"details":            "test detail",
		},
	}
	_, rosettaError = getEditValidatorOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestDelegateOperationComponents(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	delegatorAddr := crypto.PubkeyToAddress(refFromKey.PublicKey)
	delegatorBech32Addr, _ := internalCommon.AddressToBech32(delegatorAddr)
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	validatorKey := internalCommon.MustGeneratePrivateKey()
	validatorAddr, _ := internalCommon.AddressToBech32(crypto.PubkeyToAddress(validatorKey.PublicKey))

	// test valid operations
	refOperations := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.DelegateOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorBech32Addr,
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	testComponents, rosettaError := getDelegateOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expected same sender")
	}
	if testComponents.Amount != nil {
		t.Error("expected nil amount")
	}
	if testComponents.To != nil {
		t.Error("expected nil to")
	}

	// test nil account
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.DelegateOperation,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getDelegateOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getDelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid delegator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.DelegateOperation,
		Metadata: map[string]interface{}{
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getDelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid validator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.DelegateOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getDelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid amount
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.DelegateOperation,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"validatorAddress": validatorAddr,
		},
	}

	_, rosettaError = getDelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

}

func TestUndelegateOperationComponents(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	delegatorAddr := crypto.PubkeyToAddress(refFromKey.PublicKey)
	delegatorBech32Addr, _ := internalCommon.AddressToBech32(delegatorAddr)
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	validatorKey := internalCommon.MustGeneratePrivateKey()
	validatorAddr, _ := internalCommon.AddressToBech32(crypto.PubkeyToAddress(validatorKey.PublicKey))

	// test valid operations
	refOperations := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.UndelegateOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorBech32Addr,
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	testComponents, rosettaError := getUndelegateOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expected same sender")
	}
	if testComponents.Amount != nil {
		t.Error("expected nil amount")
	}
	if testComponents.To != nil {
		t.Error("expected nil to")
	}

	// test nil account
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.UndelegateOperation,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getUndelegateOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = getUndelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid delegator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.UndelegateOperation,
		Metadata: map[string]interface{}{
			"validatorAddress": validatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getUndelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid validator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.UndelegateOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"amount":           new(big.Int).SetInt64(100),
		},
	}

	_, rosettaError = getUndelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid amount
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.UndelegateOperation,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorAddr,
			"validatorAddress": validatorAddr,
		},
	}

	_, rosettaError = getUndelegateOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}

}

func TestCollectRewardsOperationComponents(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	validatorAddr := crypto.PubkeyToAddress(refFromKey.PublicKey)
	refFrom, rosettaError := newAccountIdentifier(validatorAddr)
	delegatorBech32Addr, _ := internalCommon.AddressToBech32(validatorAddr)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	// test valid operations
	refOperations := &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:    common.EditValidatorOperation,
		Account: refFrom,
		Metadata: map[string]interface{}{
			"delegatorAddress": delegatorBech32Addr,
		},
	}

	testComponents, rosettaError := getCollectRewardsOperationComponents(refOperations)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if testComponents.Type != refOperations.Type {
		t.Error("expected same operation")
	}
	if testComponents.From == nil || types.Hash(testComponents.From) != types.Hash(refFrom) {
		t.Error("expected same sender")
	}
	if testComponents.Amount != nil {
		t.Error("expected nil amount")
	}
	if testComponents.To != nil {
		t.Error("expected nil to")
	}

	// test invalid operation

	// test nil operation
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type: common.EditValidatorOperation,
		Metadata: map[string]interface{}{
			"delegatorAddress": validatorAddr,
		},
	}

	_, rosettaError = getCollectRewardsOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid delegator
	refOperations = &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: 0,
		},
		Type:     common.EditValidatorOperation,
		Account:  refFrom,
		Metadata: map[string]interface{}{},
	}

	_, rosettaError = getCollectRewardsOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestGetOperationComponents(t *testing.T) {
	refFromAmount := &types.Amount{
		Value:    "-12000",
		Currency: &common.NativeCurrency,
	}
	refToAmount := &types.Amount{
		Value:    "12000",
		Currency: &common.NativeCurrency,
	}
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refToKey := internalCommon.MustGeneratePrivateKey()
	refTo, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refToKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	// test valid transaction operation
	// Detailed test in TestGetTransferOperationComponents
	_, rosettaError = GetOperationComponents([]*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0,
			},
			Type:    common.NativeTransferOperation,
			Amount:  refFromAmount,
			Account: refFrom,
		},
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				{
					Index: 0,
				},
			},
			Type:    common.NativeTransferOperation,
			Amount:  refToAmount,
			Account: refTo,
		},
	})
	if rosettaError != nil {
		t.Error(rosettaError)
	}

	// test valid cross-shard transaction operation
	// Detailed test in TestGetCrossShardOperationComponents
	refMetadata := common.CrossShardTransactionOperationMetadata{
		From: refFrom,
		To:   refTo,
	}
	refMetadataMap, err := types.MarshalMap(refMetadata)
	if err != nil {
		t.Fatal(err)
	}
	_, rosettaError = GetOperationComponents([]*types.Operation{
		{
			Type:     common.NativeCrossShardTransferOperation,
			Amount:   refFromAmount,
			Account:  refFrom,
			Metadata: refMetadataMap,
		},
	})
	if rosettaError != nil {
		t.Error(rosettaError)
	}

	// test valid contract creation operation
	// Detailed test in TestGetContractCreationOperationComponents
	_, rosettaError = GetOperationComponents([]*types.Operation{
		{
			Type:    common.ContractCreationOperation,
			Amount:  refFromAmount,
			Account: refFrom,
		},
	})
	if rosettaError != nil {
		t.Error(rosettaError)
	}

	// test invalid number of operations
	refOperations := []*types.Operation{}
	_, rosettaError = GetOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid number of operations pas max number of operations
	for i := 0; i <= maxNumOfConstructionOps+1; i++ {
		refOperations = append(refOperations, &types.Operation{})
	}
	_, rosettaError = GetOperationComponents(refOperations)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid operation
	_, rosettaError = GetOperationComponents([]*types.Operation{
		{
			Type: common.ExpendGasOperation,
		},
	})
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil operation
	_, rosettaError = GetOperationComponents(nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}
