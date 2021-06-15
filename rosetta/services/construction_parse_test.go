package services

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/coinbase/rosetta-sdk-go/parser"
	"github.com/ethereum/go-ethereum/crypto"

	hmytypes "github.com/harmony-one/harmony/core/types"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/test/helpers"
)

var (
	tempTestSigner        = hmytypes.NewEIP155Signer(big.NewInt(0))
	tempTestStakingSigner = stakingTypes.NewEIP155Signer(big.NewInt(0))
)

func TestParseUnsignedTransaction(t *testing.T) {
	refEstGasUsed := big.NewInt(100000)
	testTx, err := helpers.CreateTestTransaction(
		tempTestSigner, 0, 1, 2, refEstGasUsed.Uint64(), gasPrice, big.NewInt(1e18), []byte{},
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
		GasUsed: testTx.GasLimit(),
	}
	refFormattedTx, rosettaError := FormatTransaction(testTx, refTestReceipt, &ContractInfo{}, false)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	refUnsignedTx := hmytypes.NewCrossShardTransaction(
		testTx.Nonce(), &refSender, testTx.ShardID(), testTx.ToShardID(), testTx.Value(),
		testTx.GasLimit(), gasPrice, testTx.Data(),
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
		if op.Status != nil {
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
	bytes := "{\"rlp_bytes\":\"+DkC65TrzRbowdj0k7oE6ZpWR0Ei2BqcWJTrzRbowdj0k7oE6ZpWR0Ei2BqcWAqAhHc1lACCUgiAgIA=\",\"is_staking\":true,\"contract_code\":\"0x\",\"from\":{\"address\":\"one1a0x3d6xpmr6f8wsyaxd9v36pytvp48zckswvv9\",\"metadata\":{\"hex_address\":\"0xeBCD16e8c1D8f493bA04E99a56474122D81A9c58\"}}}"
	wrappedTransaction, tx, rosettaError := unpackWrappedTransactionFromString(bytes, false)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	_, err := parseUnsignedTransaction(context.Background(), wrappedTransaction, tx)
	if err != nil {
		t.Fatal(err)
	}

	bytes = "{\"rlp_bytes\":\"+HgC65TrzRbowdj0k7oE6ZpWR0Ei2BqcWJTrzRbowdj0k7oE6ZpWR0Ei2BqcWAoBhDuaygCCpBAon6JAUrdiS7lD8XyHhos2yj8gAff6d+If7EVYGijoRVOgKnIb+8ecrVX0wO/R1C0AX/FUL5AJ6jyOh1CtJ3kfV/c=\",\"is_staking\":true,\"contract_code\":\"0x\",\"from\":{\"address\":\"one1snm53h0yk5xrqcxrna0n3g3a4um7peqctwrt30\",\"metadata\":{\"hex_address\":\"0x84f748DDE4B50C3060c39f5f38a23dAF37E0e418\"}}}"
	wrappedTransaction, tx, rosettaError = unpackWrappedTransactionFromString(bytes, true)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	addr, err2 := tx.SenderAddress()
	if err2 != nil {
		t.Fatal(err2)
	}
	if hexutil.Encode(addr[:]) != "0x84f748dde4b50c3060c39f5f38a23daf37e0e418" {
		t.Fatal("sender addr error")
	}
	_, err = parseSignedTransaction(context.Background(), wrappedTransaction, tx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseSignedTransaction(t *testing.T) {
	refEstGasUsed := big.NewInt(100000)
	testTx, err := helpers.CreateTestTransaction(
		tempTestSigner, 0, 1, 2, refEstGasUsed.Uint64(), gasPrice, big.NewInt(1e18), []byte{},
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
		GasUsed: testTx.GasLimit(),
	}
	refFormattedTx, rosettaError := FormatTransaction(testTx, refTestReceipt, &ContractInfo{}, true)
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
		if op.Status != nil {
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
