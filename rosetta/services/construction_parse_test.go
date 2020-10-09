package services

import (
	"context"
	"math/big"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/parser"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

var (
	tempTestSigner        = hmytypes.NewEIP155Signer(big.NewInt(0))
	tempTestStakingSigner = stakingTypes.NewEIP155Signer(big.NewInt(0))
)

func TestParseUnsignedTransaction(t *testing.T) {
	refEstGasUsed := big.NewInt(100000)
	testTx, err := createTestTransaction(
		tempTestSigner, 0, 1, 2, refEstGasUsed.Uint64(), big.NewInt(1e18), []byte{},
	)
	if err != nil {
		t.Fatal(err)
	}
	refSender, err := testTx.SenderAddress()
	if err != nil {
		t.Fatal(err)
	}
	refSenderID, rosettaError := newAccountIdentifier(refSender)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refTestReceipt := &hmytypes.Receipt{
		GasUsed: testTx.Gas(),
	}
	refFormattedTx, rosettaError := formatTransaction(testTx, refTestReceipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refUnsignedTx := hmytypes.NewCrossShardTransaction(
		testTx.Nonce(), &refSender, testTx.ShardID(), testTx.ToShardID(), testTx.Value(),
		testTx.Gas(), gasPrice, testTx.Data(),
	)

	// Test valid plain transaction
	wrappedTransaction := &WrappedTransaction{
		// RLP bytes should not be needed
		// Is staking flag should not be needed
		From: refSenderID,
	}
	parsedResponse, rosettaError := parseUnsignedTransaction(
		context.Background(), wrappedTransaction, refUnsignedTx,
	)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	for _, op := range parsedResponse.Operations {
		if op.Status != "" {
			t.Error("expect operation status to be empty for construction")
		}
	}
	p := parser.Parser{}
	if err := p.ExpectedOperations(
		refFormattedTx.Operations, parsedResponse.Operations, false, false,
	); err != nil {
		t.Error(err)
	}

	// test nil
	_, rosettaError = parseUnsignedTransaction(context.Background(), nil, nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}

func TestParseUnsignedTransactionStaking(t *testing.T) {
	// TODO (dm): implement staking test
}

func TestParseSignedTransaction(t *testing.T) {
	refEstGasUsed := big.NewInt(100000)
	testTx, err := createTestTransaction(
		tempTestSigner, 0, 1, 2, refEstGasUsed.Uint64(), big.NewInt(1e18), []byte{},
	)
	if err != nil {
		t.Fatal(err)
	}
	refSender, err := testTx.SenderAddress()
	if err != nil {
		t.Fatal(err)
	}
	refSenderID, rosettaError := newAccountIdentifier(refSender)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refTestReceipt := &hmytypes.Receipt{
		GasUsed: testTx.Gas(),
	}
	refFormattedTx, rosettaError := formatTransaction(testTx, refTestReceipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	// Test valid plain transaction
	wrappedTransaction := &WrappedTransaction{
		// RLP bytes should not be needed
		// Is staking flag should not be needed
		From: refSenderID,
	}
	parsedResponse, rosettaError := parseSignedTransaction(context.Background(), wrappedTransaction, testTx)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	for _, op := range parsedResponse.Operations {
		if op.Status != "" {
			t.Error("expect operation status to be empty for construction")
		}
	}
	p := parser.Parser{}
	if err := p.ExpectedOperations(
		refFormattedTx.Operations, parsedResponse.Operations, false, false,
	); err != nil {
		t.Error(err)
	}

	// Test invalid wrapper
	randomKey := internalCommon.MustGeneratePrivateKey()
	randomAccountID, rosettaError := newAccountIdentifier(crypto.PubkeyToAddress(randomKey.PublicKey))
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	wrappedTransaction = &WrappedTransaction{
		// RLP bytes should not be needed
		// Is staking flag should not be needed
		From: randomAccountID,
	}
	_, rosettaError = parseSignedTransaction(context.Background(), wrappedTransaction, testTx)
	if rosettaError == nil {
		t.Error("expected error")
	}

	// test nil
	_, rosettaError = parseSignedTransaction(context.Background(), nil, nil)
	if rosettaError == nil {
		t.Error("expected error")
	}
}
