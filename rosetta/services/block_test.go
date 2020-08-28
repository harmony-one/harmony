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

	"github.com/harmony-one/harmony/core"
	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/bls"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	rpcV2 "github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/staking"
	stakingNetwork "github.com/harmony-one/harmony/staking/network"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

var (
	oneBig     = big.NewInt(1e18)
	tenOnes    = new(big.Int).Mul(big.NewInt(10), oneBig)
	twelveOnes = new(big.Int).Mul(big.NewInt(12), oneBig)
	gasPrice   = big.NewInt(10000)
)

func createTestStakingTransaction(
	payloadMaker func() (stakingTypes.Directive, interface{}), key *ecdsa.PrivateKey, nonce, gasLimit uint64,
) (*stakingTypes.StakingTransaction, error) {
	tx, err := stakingTypes.NewStakingTransaction(nonce, gasLimit, gasPrice, payloadMaker)
	if err != nil {
		return nil, err
	}
	if key == nil {
		key, err = crypto.GenerateKey()
		if err != nil {
			return nil, err
		}
	}
	// Staking transactions are always post EIP155 epoch
	return stakingTypes.Sign(tx, stakingTypes.NewEIP155Signer(tx.ChainID()), key)
}

func getMessageFromStakingTx(tx *stakingTypes.StakingTransaction) (map[string]interface{}, error) {
	rpcStakingTx, err := rpcV2.NewStakingTransaction(tx, ethcommon.Hash{}, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	return rpc.NewStructuredResponse(rpcStakingTx.Msg)
}

func createTestTransaction(
	signer hmytypes.Signer, fromShard, toShard uint32, nonce, gasLimit uint64, amount *big.Int, data []byte,
) (*hmytypes.Transaction, error) {
	fromKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	toKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	toAddr := crypto.PubkeyToAddress(toKey.PublicKey)
	var tx *hmytypes.Transaction
	if fromShard != toShard {
		tx = hmytypes.NewCrossShardTransaction(
			nonce, &toAddr, fromShard, toShard, amount, gasLimit, gasPrice, data,
		)
	} else {
		tx = hmytypes.NewTransaction(
			nonce, toAddr, fromShard, amount, gasLimit, gasPrice, data,
		)
	}
	return hmytypes.SignTx(tx, signer, fromKey)
}

func createTestContractCreationTransaction(
	signer hmytypes.Signer, shard uint32, nonce, gasLimit uint64, data []byte,
) (*hmytypes.Transaction, error) {
	fromKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	tx := hmytypes.NewContractCreation(nonce, shard, big.NewInt(0), gasLimit, gasPrice, data)
	return hmytypes.SignTx(tx, signer, fromKey)
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
	// formatCrossShardReceiverTransaction, thus, it is not tested here -- but tested on its own.
	testFormatCrossShardSenderTransaction(t, gasLimit, gasUsed, senderKey, receiverKey)
}

func testFormatStakingTransaction(
	t *testing.T, gasLimit, gasUsed uint64, senderKey, receiverKey *ecdsa.PrivateKey,
) {
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	receiverAddr := crypto.PubkeyToAddress(receiverKey.PublicKey)
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveDelegate, stakingTypes.Delegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: receiverAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit)
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
	rosettaTx, rosettaError := formatTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	if len(rosettaTx.Operations) != 2 {
		t.Error("Expected 2 operations")
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
	tx, err := createTestTransaction(
		signer, 0, 0, 0, 1e18, big.NewInt(1), []byte("test"),
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
	rosettaTx, rosettaError := formatTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if len(rosettaTx.Operations) != 3 {
		t.Error("Expected 3 operations")
	}
	if rosettaTx.TransactionIdentifier.Hash != tx.Hash().String() {
		t.Error("Invalid transaction")
	}
	if rosettaTx.Operations[0].Type != common.ExpendGasOperation {
		t.Error("Expected 1st operation to be gas")
	}
	if rosettaTx.Operations[1].Type != common.TransferOperation {
		t.Error("Expected 2nd operation to transfer related")
	}
	if !reflect.DeepEqual(rosettaTx.Operations[1].Metadata, map[string]interface{}{}) {
		t.Error("Expected 1st operation to have no metadata")
	}
	if !reflect.DeepEqual(rosettaTx.Operations[2].Metadata, map[string]interface{}{}) {
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
		b32Addr, err := internalCommon.AddressToBech32(acc)
		if err != nil {
			t.Fatal(err)
		}
		txID := getSpecialCaseTransactionIdentifier(testBlkHash, b32Addr)
		tx, rosettaError := formatGenesisTransaction(txID, b32Addr, 0)
		if rosettaError != nil {
			t.Fatal(rosettaError)
		}
		if !reflect.DeepEqual(txID, tx.TransactionIdentifier) {
			t.Error("expected transaction ID of formatted tx to be same as requested")
		}
		if len(tx.Operations) != 1 {
			t.Error("expected exactly 1 operation")
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

func TestFormatPreStakingBlockRewardsTransactionSuccess(t *testing.T) {
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	testB32Addr, err := internalCommon.AddressToBech32(testAddr)
	if err != nil {
		t.Fatal(err)
	}
	testBlockSigInfo := &blockSignerInfo{
		signers: map[ethcommon.Address][]bls.SerializedPublicKey{
			testAddr: { // Only care about length for this test
				bls.SerializedPublicKey{},
				bls.SerializedPublicKey{},
			},
		},
		totalKeysSigned: 150,
		blockHash:       ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238"),
	}
	refTxID := getSpecialCaseTransactionIdentifier(testBlockSigInfo.blockHash, testB32Addr)
	tx, rosettaError := formatPreStakingBlockRewardsTransaction(testB32Addr, testBlockSigInfo)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}

	if !reflect.DeepEqual(tx.TransactionIdentifier, refTxID) {
		t.Errorf("Expected TxID %v got %v", refTxID, tx.TransactionIdentifier)
	}
	if len(tx.Operations) != 1 {
		t.Fatal("Expected exactly 1 operation")
	}
	if tx.Operations[0].OperationIdentifier.Index != 0 {
		t.Error("expected operational ID to be 0")
	}
	if tx.Operations[0].Type != common.PreStakingEraBlockRewardOperation {
		t.Error("expected operation type to be pre staking era block rewards")
	}
	if tx.Operations[0].Status != common.SuccessOperationStatus.Status {
		t.Error("expected successful operation status")
	}

	// Expect: myNumberOfSigForBlock * (totalAmountOfRewardsPerBlock / numOfSigsForBlock) to be my block reward amount
	refAmount := new(big.Int).Mul(new(big.Int).Quo(stakingNetwork.BlockReward, big.NewInt(150)), big.NewInt(2))
	fmtRefAmount := fmt.Sprintf("%v", refAmount)
	if tx.Operations[0].Amount.Value != fmtRefAmount {
		t.Errorf("expected operation amount to be %v not %v", fmtRefAmount, tx.Operations[0].Amount.Value)
	}
}

func TestFormatPreStakingBlockRewardsTransactionFail(t *testing.T) {
	testKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	testAddr := crypto.PubkeyToAddress(testKey.PublicKey)
	testB32Addr, err := internalCommon.AddressToBech32(testAddr)
	if err != nil {
		t.Fatal(err)
	}
	testBlockSigInfo := &blockSignerInfo{
		signers: map[ethcommon.Address][]bls.SerializedPublicKey{
			testAddr: {},
		},
		totalKeysSigned: 150,
		blockHash:       ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238"),
	}
	_, rosettaError := formatPreStakingBlockRewardsTransaction(testB32Addr, testBlockSigInfo)
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if !reflect.DeepEqual(&common.TransactionNotFoundError, rosettaError) {
		t.Error("expected transaction not found error")
	}

	testBlockSigInfo = &blockSignerInfo{
		signers:         map[ethcommon.Address][]bls.SerializedPublicKey{},
		totalKeysSigned: 150,
		blockHash:       ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238"),
	}
	_, rosettaError = formatPreStakingBlockRewardsTransaction(testB32Addr, testBlockSigInfo)
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if !reflect.DeepEqual(&common.TransactionNotFoundError, rosettaError) {
		t.Error("expected transaction not found error")
	}
}

func testFormatCrossShardSenderTransaction(
	t *testing.T, gasLimit, gasUsed uint64, senderKey, receiverKey *ecdsa.PrivateKey,
) {
	// Note that post EIP-155 epoch singer is tested in detailed tests.
	signer := hmytypes.HomesteadSigner{}
	tx, err := createTestTransaction(
		signer, 0, 1, 0, 1e18, big.NewInt(1), []byte("test"),
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
	rosettaTx, rosettaError := formatTransaction(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if len(rosettaTx.Operations) != 2 {
		t.Error("Expected 2 operations")
	}
	if rosettaTx.TransactionIdentifier.Hash != tx.Hash().String() {
		t.Error("Invalid transaction")
	}
	if rosettaTx.Operations[0].Type != common.ExpendGasOperation {
		t.Error("Expected 1st operation to be gas")
	}
	if rosettaTx.Operations[1].Type != common.CrossShardTransferOperation {
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

func TestGetStakingOperationsFromCreateValidator(t *testing.T) {
	gasLimit := uint64(1e18)
	createValidatorTxDescription := stakingTypes.Description{
		Name:            "SuperHero",
		Identity:        "YouWouldNotKnow",
		Website:         "Secret Website",
		SecurityContact: "LicenseToKill",
		Details:         "blah blah blah",
	}
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		fromKey, _ := crypto.GenerateKey()
		return stakingTypes.DirectiveCreateValidator, stakingTypes.CreateValidator{
			Description:        createValidatorTxDescription,
			MinSelfDelegation:  tenOnes,
			MaxTotalDelegation: twelveOnes,
			ValidatorAddress:   crypto.PubkeyToAddress(fromKey.PublicKey),
			Amount:             tenOnes,
		}
	}, nil, 0, gasLimit)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := getMessageFromStakingTx(tx)
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
	refOperations := newOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("-%v", tenOnes.Uint64()),
			Currency: &common.Currency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := getStakingOperations(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
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
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveDelegate, stakingTypes.Delegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: validatorAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := getMessageFromStakingTx(tx)
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
	refOperations := newOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("-%v", tenOnes.Uint64()),
			Currency: &common.Currency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := getStakingOperations(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
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
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveUndelegate, stakingTypes.Undelegate{
			DelegatorAddress: senderAddr,
			ValidatorAddress: validatorAddr,
			Amount:           tenOnes,
		}
	}, senderKey, 0, gasLimit)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := getMessageFromStakingTx(tx)
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
	refOperations := newOperations(gasFee, senderAccID)
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
			Currency: &common.Currency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := getStakingOperations(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestGetStakingOperationsFromCollectRewards(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveCollectRewards, stakingTypes.CollectRewards{
			DelegatorAddress: senderAddr,
		}
	}, senderKey, 0, gasLimit)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := getMessageFromStakingTx(tx)
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
	refOperations := newOperations(gasFee, senderAccID)
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
			Currency: &common.Currency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := getStakingOperations(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestGetStakingOperationsFromEditValidator(t *testing.T) {
	gasLimit := uint64(1e18)
	senderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	senderAddr := crypto.PubkeyToAddress(senderKey.PublicKey)
	tx, err := createTestStakingTransaction(func() (stakingTypes.Directive, interface{}) {
		return stakingTypes.DirectiveEditValidator, stakingTypes.EditValidator{
			ValidatorAddress: senderAddr,
		}
	}, senderKey, 0, gasLimit)
	if err != nil {
		t.Fatal(err.Error())
	}
	metadata, err := getMessageFromStakingTx(tx)
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
	refOperations := newOperations(gasFee, senderAccID)
	refOperations = append(refOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{Index: 1},
		RelatedOperations: []*types.OperationIdentifier{
			{Index: 0},
		},
		Type:    tx.StakingType().String(),
		Status:  common.SuccessOperationStatus.Status,
		Account: senderAccID,
		Amount: &types.Amount{
			Value:    fmt.Sprintf("-%v", 0),
			Currency: &common.Currency,
		},
		Metadata: metadata,
	})
	operations, rosettaError := getStakingOperations(tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestNewTransferOperations(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	tx, err := createTestTransaction(
		signer, 0, 0, 0, 1e18, big.NewInt(1), []byte("test"),
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
			Type:    common.TransferOperation,
			Status:  common.ContractFailureOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{},
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
			Type:    common.TransferOperation,
			Status:  common.ContractFailureOperationStatus.Status,
			Account: receiverAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("%v", tx.Value().Uint64()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{},
		},
	}
	receipt := &hmytypes.Receipt{
		Status: hmytypes.ReceiptStatusFailed,
	}
	operations, rosettaError := newTransferOperations(startingOpID, tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}

	// Test successful plain / contract transaction
	refOperations[0].Status = common.SuccessOperationStatus.Status
	refOperations[1].Status = common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful
	operations, rosettaError = newTransferOperations(startingOpID, tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestNewCrossShardSenderTransferOperations(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	tx, err := createTestTransaction(
		signer, 0, 1, 0, 1e18, big.NewInt(1), []byte("data-does-nothing"),
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

	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOpID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOpID,
			},
			Type:    common.CrossShardTransferOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: senderAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{
				"to_account": receiverAccID,
			},
		},
	}
	operations, rosettaError := newCrossShardSenderTransferOperations(startingOpID, tx)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestFormatCrossShardReceiverTransaction(t *testing.T) {
	signer := hmytypes.NewEIP155Signer(params.TestChainConfig.ChainID)
	tx, err := createTestTransaction(
		signer, 0, 1, 0, 1e18, big.NewInt(1), []byte{},
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

	refCxID := &types.TransactionIdentifier{Hash: tx.Hash().String()}
	refOperations := []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0, // There is no gas expenditure for cross-shard payout
			},
			Type:    common.CrossShardTransferOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: receiverAccID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("%v", tx.Value().Uint64()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{
				"from_account": senderAccID,
			},
		},
	}
	refMetadata, err := rpc.NewStructuredResponse(TransactionMetadata{
		CrossShardIdentifier: refCxID,
		ToShardID:            tx.ToShardID(),
		FromShardID:          tx.ShardID(),
	})
	refRosettaTx := &types.Transaction{
		TransactionIdentifier: refCxID,
		Operations:            refOperations,
		Metadata:              refMetadata,
	}
	rosettaTx, rosettaError := formatCrossShardReceiverTransaction(cxReceipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(rosettaTx, refRosettaTx) {
		t.Errorf("Expected transaction to be %v not %v", refRosettaTx, rosettaTx)
	}
}

func TestNewContractCreationOperations(t *testing.T) {
	dummyContractKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	chainID := params.TestChainConfig.ChainID
	signer := hmytypes.NewEIP155Signer(chainID)
	tx, err := createTestContractCreationTransaction(
		signer, 0, 0, 1e18, []byte("test"),
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
				Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{
				"contract_address": contractAddr.String(),
			},
		},
	}
	receipt := &hmytypes.Receipt{
		Status:          hmytypes.ReceiptStatusFailed,
		ContractAddress: contractAddr,
	}
	operations, rosettaError := newContractCreationOperations(startingOpID, tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}

	// Test successful contract creation
	refOperations[0].Status = common.SuccessOperationStatus.Status
	receipt.Status = hmytypes.ReceiptStatusSuccessful // Indicate successful tx
	operations, rosettaError = newContractCreationOperations(startingOpID, tx, receipt)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if !reflect.DeepEqual(operations, refOperations) {
		t.Errorf("Expected operations to be %v not %v", refOperations, operations)
	}
}

func TestNewAccountIdentifier(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf(err.Error())
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)
	b32Addr, err := internalCommon.AddressToBech32(addr)
	if err != nil {
		t.Fatalf(err.Error())
	}
	metadata, err := rpc.NewStructuredResponse(AccountMetadata{Address: addr.String()})
	if err != nil {
		t.Fatalf(err.Error())
	}

	referenceAccID := &types.AccountIdentifier{
		Address:  b32Addr,
		Metadata: metadata,
	}
	testAccID, rosettaError := newAccountIdentifier(addr)
	if rosettaError != nil {
		t.Fatalf("unexpected rosetta error: %v", rosettaError)
	}
	if !reflect.DeepEqual(referenceAccID, testAccID) {
		t.Errorf("reference ID %v != testID %v", referenceAccID, testAccID)
	}
}

func TestNewOperations(t *testing.T) {
	accountID := &types.AccountIdentifier{
		Address: "test-address",
	}
	gasFee := big.NewInt(int64(1e18))
	amount := &types.Amount{
		Value:    fmt.Sprintf("-%v", gasFee),
		Currency: &common.Currency,
	}

	ops := newOperations(gasFee, accountID)
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

func TestFindLogsWithTopic(t *testing.T) {
	tests := []struct {
		receipt          *hmytypes.Receipt
		topic            ethcommon.Hash
		expectedResponse []*hmytypes.Log
	}{
		// test 0
		{
			receipt: &hmytypes.Receipt{
				Logs: []*hmytypes.Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
							staking.IsValidator,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic: staking.IsValidatorKey,
			expectedResponse: []*hmytypes.Log{
				{
					Topics: []ethcommon.Hash{
						staking.IsValidatorKey,
						staking.IsValidator,
					},
				},
			},
		},
		// test 1
		{
			receipt: &hmytypes.Receipt{
				Logs: []*hmytypes.Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
							staking.IsValidator,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic: staking.CollectRewardsTopic,
			expectedResponse: []*hmytypes.Log{
				{
					Topics: []ethcommon.Hash{
						staking.CollectRewardsTopic,
					},
				},
			},
		},
		// test 2
		{
			receipt: &hmytypes.Receipt{
				Logs: []*hmytypes.Log{
					{
						Topics: []ethcommon.Hash{
							staking.IsValidatorKey,
						},
					},
					{
						Topics: []ethcommon.Hash{
							crypto.Keccak256Hash([]byte("test")),
						},
					},
					{
						Topics: []ethcommon.Hash{
							staking.CollectRewardsTopic,
						},
					},
				},
			},
			topic:            staking.IsValidator,
			expectedResponse: []*hmytypes.Log{},
		},
	}

	for i, test := range tests {
		response := findLogsWithTopic(test.receipt, test.topic)
		if !reflect.DeepEqual(test.expectedResponse, response) {
			t.Errorf("Failed test %v, expected %v, got %v", i, test.expectedResponse, response)
		}
	}
}

func TestGetPseudoTransactionForGenesis(t *testing.T) {
	genesisSpec := core.NewGenesisSpec(nodeconfig.Testnet, 0)
	txs := getPseudoTransactionForGenesis(genesisSpec)
	for acc := range genesisSpec.Alloc {
		found := false
		for _, tx := range txs {
			if acc == *tx.To() {
				found = true
				break
			}
		}
		if !found {
			t.Error("unable to find genesis account in generated pseudo transactions")
		}
	}
}

func TestSpecialCaseTransactionIdentifier(t *testing.T) {
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	testB32Address := "one10g7kfque6ew2jjfxxa6agkdwk4wlyjuncp6gwz"
	refTxID := &types.TransactionIdentifier{
		Hash: fmt.Sprintf("%v_%v", testBlkHash.String(), testB32Address),
	}
	specialTxID := getSpecialCaseTransactionIdentifier(testBlkHash, testB32Address)
	if !reflect.DeepEqual(refTxID, specialTxID) {
		t.Fatal("invalid for mate for special case TxID")
	}
	unpackedBlkHash, unpackedB32Address, rosettaError := unpackSpecialCaseTransactionIdentifier(specialTxID)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if unpackedB32Address != testB32Address {
		t.Errorf("expected unpacked address to be %v not %v", testB32Address, unpackedB32Address)
	}
	if unpackedBlkHash.String() != testBlkHash.String() {
		t.Errorf("expected blk hash to be %v not %v", unpackedBlkHash.String(), testBlkHash.String())
	}

	_, _, rosettaError = unpackSpecialCaseTransactionIdentifier(
		&types.TransactionIdentifier{Hash: ""},
	)
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if rosettaError.Code != common.CatchAllError.Code {
		t.Error("expected error code to be catch call error")
	}
}
