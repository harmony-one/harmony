package services

import (
	"math/big"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"

	hmyTypes "github.com/harmony-one/harmony/core/types"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/rosetta/common"
)

func TestConstructPlainTransaction(t *testing.T) {
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
	refDataBytes := []byte{0xEE, 0xEE, 0xEE}
	refData := hexutil.Encode(refDataBytes)
	refComponents := &OperationComponents{
		Type:           common.NativeTransferOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}
	refMetadata := &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}
	refShard := uint32(0)

	// test valid transaction
	generalTx, rosettaError := constructPlainTransaction(refComponents, refMetadata, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok := generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.Nonce() != refMetadata.Nonce {
		t.Error("nonce does not match")
	}
	testToID, rosettaError := newAccountIdentifier(*(tx.To()))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if types.Hash(testToID) != types.Hash(refTo) {
		t.Error("to account ID does not math")
	}
	if tx.ShardID() != refShard {
		t.Error("invalid shard")
	}
	if tx.Value().Cmp(refComponents.Amount) != 0 {
		t.Error("transaction value does not match")
	}
	if tx.GasLimit() != refMetadata.GasLimit {
		t.Error("transaction gas limit does not match")
	}
	if tx.GasPrice().Cmp(refMetadata.GasPrice) != 0 {
		t.Error("transaction gas price does not match")
	}
	if hexutil.Encode(tx.Data()) != refData {
		t.Error("data does not match")
	}

	// test valid empty transaction metadata transaction
	refMetadata.Transaction = &TransactionMetadata{}
	generalTx, rosettaError = constructPlainTransaction(refComponents, refMetadata, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok = generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.Nonce() != refMetadata.Nonce {
		t.Error("nonce does not match")
	}
	testToID, rosettaError = newAccountIdentifier(*(tx.To()))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if types.Hash(testToID) != types.Hash(refTo) {
		t.Error("to account ID does not math")
	}
	if tx.ShardID() != refShard {
		t.Error("invalid shard")
	}
	if tx.Value().Cmp(refComponents.Amount) != 0 {
		t.Error("transaction value does not match")
	}
	if tx.GasLimit() != refMetadata.GasLimit {
		t.Error("transaction gas limit does not match")
	}
	if tx.GasPrice().Cmp(refMetadata.GasPrice) != 0 {
		t.Error("transaction gas price does not match")
	}
	refMetadata.Transaction = &TransactionMetadata{
		Data: &refData,
	}

	// test invalid receiver
	_, rosettaError = constructPlainTransaction(&OperationComponents{
		Type:           common.NativeTransferOperation,
		From:           refFrom,
		To:             nil,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
	_, rosettaError = constructPlainTransaction(&OperationComponents{
		Type: common.NativeTransferOperation,
		From: refFrom,
		To: &types.AccountIdentifier{
			Address: "",
		},
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test valid nil sender
	_, rosettaError = constructPlainTransaction(&OperationComponents{
		Type:           common.NativeTransferOperation,
		From:           nil,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError != nil {
		t.Error(rosettaError)
	}

	// test invalid data
	badData := "this is not a hex string"
	_, rosettaError = constructPlainTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &badData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestConstructCrossShardTransaction(t *testing.T) {
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
	refDataBytes := []byte{0xEE, 0xEE, 0xEE}
	refData := hexutil.Encode(refDataBytes)
	refComponents := &OperationComponents{
		Type:           common.NativeCrossShardTransferOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}
	refShard := uint32(0)
	refToShard := uint32(1)
	refMetadata := &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &refData,
			FromShardID: &refShard,
			ToShardID:   &refToShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}

	// test valid transaction
	generalTx, rosettaError := constructCrossShardTransaction(refComponents, refMetadata, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok := generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.Nonce() != refMetadata.Nonce {
		t.Error("nonce does not match")
	}
	testToID, rosettaError := newAccountIdentifier(*(tx.To()))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if types.Hash(testToID) != types.Hash(refTo) {
		t.Error("to account ID does not math")
	}
	if tx.ShardID() != refShard {
		t.Error("invalid shard")
	}
	if tx.Value().Cmp(refComponents.Amount) != 0 {
		t.Error("transaction value does not match")
	}
	if tx.GasLimit() != refMetadata.GasLimit {
		t.Error("transaction gas limit does not match")
	}
	if tx.GasPrice().Cmp(refMetadata.GasPrice) != 0 {
		t.Error("transaction gas price does not match")
	}
	if hexutil.Encode(tx.Data()) != refData {
		t.Error("data does not match")
	}
	if tx.ToShardID() != refToShard {
		t.Error("invalid destination shard")
	}

	// test invalid receiver
	_, rosettaError = constructCrossShardTransaction(&OperationComponents{
		Type:           common.NativeCrossShardTransferOperation,
		From:           refFrom,
		To:             nil,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
	_, rosettaError = constructCrossShardTransaction(&OperationComponents{
		Type: common.NativeCrossShardTransferOperation,
		From: refFrom,
		To: &types.AccountIdentifier{
			Address: "",
		},
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test valid nil sender
	_, rosettaError = constructCrossShardTransaction(&OperationComponents{
		Type:           common.NativeCrossShardTransferOperation,
		From:           nil,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, refMetadata, refShard)
	if rosettaError != nil {
		t.Error(rosettaError)
	}

	// test invalid data
	badData := "this is not a hex string"
	_, rosettaError = constructCrossShardTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &badData,
			FromShardID: &refShard,
			ToShardID:   &refToShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid from shard
	badShard := refToShard + refShard + 1
	_, rosettaError = constructCrossShardTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &refData,
			FromShardID: &badShard,
			ToShardID:   &refToShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid shards
	_, rosettaError = constructCrossShardTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &refData,
			FromShardID: &refShard,
			ToShardID:   &refShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
	_, rosettaError = constructCrossShardTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestConstructContractCreationTransaction(t *testing.T) {
	refFromKey := internalCommon.MustGeneratePrivateKey()
	refFrom, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(refFromKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refDataBytes := []byte{0xEE, 0xEE, 0xEE}
	refData := hexutil.Encode(refDataBytes)
	refComponents := &OperationComponents{
		Type:           common.ContractCreationOperation,
		From:           refFrom,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}
	refMetadata := &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}
	refShard := uint32(0)

	// test valid transaction
	generalTx, rosettaError := constructContractCreationTransaction(refComponents, refMetadata, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok := generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.Nonce() != refMetadata.Nonce {
		t.Error("nonce does not match")
	}
	if tx.ShardID() != refShard {
		t.Error("invalid shard")
	}
	if tx.Value().Cmp(refComponents.Amount) != 0 {
		t.Error("transaction value does not match")
	}
	if tx.GasLimit() != refMetadata.GasLimit {
		t.Error("transaction gas limit does not match")
	}
	if tx.GasPrice().Cmp(refMetadata.GasPrice) != 0 {
		t.Error("transaction gas price does not match")
	}
	if hexutil.Encode(tx.Data()) != refData {
		t.Error("data does not match")
	}
	if tx.To() != nil {
		t.Error("expect contract creation to NOT have to receiver")
	}

	// test invalid data
	badData := "this is not a hex string"
	_, rosettaError = constructContractCreationTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &badData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil data
	_, rosettaError = constructContractCreationTransaction(refComponents, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: nil,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestConstructTransaction(t *testing.T) {
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
	refDataBytes := []byte{0xEE, 0xEE, 0xEE}
	refData := hexutil.Encode(refDataBytes)
	refShard := uint32(0)
	refToShard := uint32(1)

	// test valid cross-shard transfer (negative test cases are in TestConstructCrossShardTransaction)
	generalTx, rosettaError := ConstructTransaction(&OperationComponents{
		Type:           common.NativeCrossShardTransferOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &refData,
			FromShardID: &refShard,
			ToShardID:   &refToShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok := generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.ToShardID() == tx.ShardID() {
		t.Error("not a cross shard transaction")
	}

	// test valid contract creation (negative test cases are in TestConstructContractCreationTransaction)
	generalTx, rosettaError = ConstructTransaction(&OperationComponents{
		Type:           common.ContractCreationOperation,
		From:           refFrom,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok = generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.To() != nil {
		t.Error("not a contract creation transaction")
	}

	// test valid transfer (negative test cases are in TestConstructPlainTransaction)
	generalTx, rosettaError = ConstructTransaction(&OperationComponents{
		Type:           common.NativeTransferOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	tx, ok = generalTx.(*hmyTypes.Transaction)
	if !ok {
		t.Fatal("invalid transaction")
	}
	if tx.ShardID() != tx.ToShardID() {
		t.Error("not a same shard transaction")
	}

	// test invalid sender shard
	badShard := refShard + refToShard + 1
	_, rosettaError = ConstructTransaction(&OperationComponents{
		Type:           common.NativeTransferOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data:        &refData,
			FromShardID: &badShard,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test invalid operation
	_, rosettaError = ConstructTransaction(&OperationComponents{
		Type:           common.ExpendGasOperation,
		From:           refFrom,
		To:             refTo,
		Amount:         big.NewInt(12000),
		StakingMessage: nil,
	}, &ConstructMetadata{
		Transaction: &TransactionMetadata{
			Data: &refData,
		},
		Nonce:    0,
		GasLimit: 50000,
		GasPrice: big.NewInt(1e18),
	}, refShard)
	if rosettaError == nil {
		t.Error("expected error")
	}
}
