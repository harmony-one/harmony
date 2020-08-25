package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	rpcV2 "github.com/harmony-one/harmony/rpc/v2"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// BlockAPI implements the server.BlockAPIServicer interface.
type BlockAPI struct {
	hmy *hmy.Harmony
}

// NewBlockAPI creates a new instance of a BlockAPI.
func NewBlockAPI(hmy *hmy.Harmony) server.BlockAPIServicer {
	return &BlockAPI{
		hmy: hmy,
	}
}

// Block implements the /block endpoint
func (s *BlockAPI) Block(
	ctx context.Context, request *types.BlockRequest,
) (response *types.BlockResponse, rosettaError *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	var block *hmytypes.Block
	var currBlockID, prevBlockID *types.BlockIdentifier
	if block, rosettaError = s.getBlock(ctx, request); rosettaError != nil {
		return nil, rosettaError
	}
	currBlockID = &types.BlockIdentifier{
		Index: block.Number().Int64(),
		Hash:  block.Hash().String(),
	}

	otherTransactions := []*types.TransactionIdentifier{}
	if block.Number().Int64() > 0 {
		prevBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(block.Number().Int64()-1).EthBlockNumber())
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		prevBlockID = &types.BlockIdentifier{
			Index: prevBlock.Number().Int64(),
			Hash:  prevBlock.Hash().String(),
		}
	} else {
		prevBlockID = currBlockID
		genesisSpec := getGenesisSpec(block.ShardID())
		for _, tx := range getPseudoTransactionForGenesis(genesisSpec) {
			otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
				Hash: tx.To().String(), // use hex address as tx identifiers for genesis block only
			})
		}
	}

	responseBlock := &types.Block{
		BlockIdentifier:       currBlockID,
		ParentBlockIdentifier: prevBlockID,
		Timestamp:             block.Time().Int64() * 1e3, // Timestamp must be in ms.
		Transactions:          []*types.Transaction{},     // Do not return tx details as it is optional.
	}

	for _, tx := range block.Transactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	for _, tx := range block.StakingTransactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	for _, cxReceipts := range block.IncomingReceipts() {
		for _, cxReceipt := range cxReceipts.Receipts {
			otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
				Hash: cxReceipt.TxHash.String(),
			})
		}
	}

	return &types.BlockResponse{
		Block:             responseBlock,
		OtherTransactions: otherTransactions,
	}, nil
}

// BlockTransaction implements the /block/transaction endpoint
func (s *BlockAPI) BlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (response *types.BlockTransactionResponse, rosettaError *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	// Special case for genesis block
	if request.BlockIdentifier.Index == 0 {
		txs, rosettaError := formatGenesisTransaction(request.TransactionIdentifier, s.hmy.ShardID)
		if rosettaError != nil {
			return nil, rosettaError
		}
		return &types.BlockTransactionResponse{Transaction: txs}, nil
	}

	blockHash := ethcommon.HexToHash(request.BlockIdentifier.Hash)
	txHash := ethcommon.HexToHash(request.TransactionIdentifier.Hash)
	txInfo, rosettaError := s.getTransactionInfo(ctx, blockHash, txHash)
	if rosettaError != nil {
		return nil, rosettaError
	}

	var transaction *types.Transaction
	if txInfo.tx != nil && txInfo.receipt != nil {
		transaction, rosettaError = formatTransaction(txInfo.tx, txInfo.receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else if txInfo.cxReceipt != nil {
		transaction, rosettaError = formatCrossShardReceiverTransaction(txInfo.cxReceipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else {
		return nil, &common.TransactionNotFoundError
	}
	return &types.BlockTransactionResponse{Transaction: transaction}, nil
}

func (s *BlockAPI) getBlock(
	ctx context.Context, request *types.BlockRequest,
) (block *hmytypes.Block, rosettaError *types.Error) {
	var err error
	if request.BlockIdentifier.Hash != nil {
		requestBlockHash := ethcommon.HexToHash(*request.BlockIdentifier.Hash)
		block, err = s.hmy.GetBlock(ctx, requestBlockHash)
	} else if request.BlockIdentifier.Index != nil {
		block, err = s.hmy.BlockByNumber(ctx, rpc.BlockNumber(*request.BlockIdentifier.Index).EthBlockNumber())
	} else {
		return nil, &common.BlockNotFoundError
	}
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	return block, nil
}

// transactionInfo stores all related information for any transaction on the Harmony chain
// Note that some elements can be nil if not applicable
type transactionInfo struct {
	tx        hmytypes.PoolTransaction
	txIndex   uint64
	receipt   *hmytypes.Receipt
	cxReceipt *hmytypes.CXReceipt
}

// getTransactionInfo given the block hash and transaction hash
func (s *BlockAPI) getTransactionInfo(
	ctx context.Context, blockHash, txHash ethcommon.Hash,
) (txInfo *transactionInfo, rosettaError *types.Error) {
	// Look for all of the possible transactions
	var index uint64
	var plainTx *hmytypes.Transaction
	var stakingTx *stakingTypes.StakingTransaction
	plainTx, _, _, index = rawdb.ReadTransaction(s.hmy.ChainDb(), txHash)
	if plainTx == nil {
		stakingTx, _, _, index = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), txHash)
	}
	cxReceipt, _, _, _ := rawdb.ReadCXReceipt(s.hmy.ChainDb(), txHash)

	if plainTx == nil && stakingTx == nil && cxReceipt == nil {
		return nil, &common.TransactionNotFoundError
	}

	var receipt *hmytypes.Receipt
	receipts, err := s.hmy.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if int(index) < len(receipts) {
		receipt = receipts[index]
	}

	// Use pool transaction for concise formatting
	var tx hmytypes.PoolTransaction
	if stakingTx != nil {
		tx = stakingTx
	} else if plainTx != nil {
		tx = plainTx
	}

	return &transactionInfo{
		tx:        tx,
		txIndex:   index,
		receipt:   receipt,
		cxReceipt: cxReceipt,
	}, nil
}

func (s *BlockAPI) getTransactionReceiptFromIndex(
	ctx context.Context, blockHash ethcommon.Hash, index uint64,
) (*hmytypes.Receipt, *types.Error) {
	receipts, err := s.hmy.GetReceipts(ctx, blockHash)
	if err != nil || len(receipts) <= int(index) {
		message := fmt.Sprintf("Transasction receipt not found")
		if err != nil {
			message = fmt.Sprintf("Transasction receipt not found: %v", err.Error())
		}
		return nil, common.NewError(common.ReceiptNotFoundError, map[string]interface{}{
			"message": message,
		})
	}
	return receipts[index], nil
}

// getPseudoTransactionForGenesis to create unsigned transaction that contain genesis funds.
// Note that this is for internal usage only. Genesis funds are not transactions.
func getPseudoTransactionForGenesis(spec *core.Genesis) []*hmytypes.Transaction {
	txs := []*hmytypes.Transaction{}
	for acc, bal := range spec.Alloc {
		txs = append(txs, hmytypes.NewTransaction(
			0, acc, spec.ShardID, bal.Balance, 0, big.NewInt(0), spec.ExtraData,
		))
	}
	return txs
}

// TransactionMetadata ..
type TransactionMetadata struct {
	CrossShardIdentifier *types.TransactionIdentifier `json:"cross_shard_transaction_identifier,omitempty"`
	ToShardID            uint32                       `json:"to_shard,omitempty"`
	FromShardID          uint32                       `json:"from_shard,omitempty"`
	Data                 string                       `json:"data,omitempty"`
}

// formatCrossShardReceiverTransaction for cross-shard payouts on destination shard
func formatCrossShardReceiverTransaction(
	cxReceipt *hmytypes.CXReceipt,
) (txs *types.Transaction, rosettaError *types.Error) {
	ctxID := &types.TransactionIdentifier{Hash: cxReceipt.TxHash.String()}
	metadata, err := rpc.NewStructuredResponse(TransactionMetadata{
		CrossShardIdentifier: ctxID,
		ToShardID:            cxReceipt.ToShardID,
		FromShardID:          cxReceipt.ShardID,
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	senderAccountID, rosettaError := newAccountIdentifier(cxReceipt.From)
	if rosettaError != nil {
		return nil, rosettaError
	}
	receiverAccountID, rosettaError := newAccountIdentifier(*cxReceipt.To)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return &types.Transaction{
		TransactionIdentifier: ctxID,
		Metadata:              metadata,
		Operations: []*types.Operation{
			{
				OperationIdentifier: &types.OperationIdentifier{
					Index: 0, // There is no gas expenditure for cross-shard transaction payout
				},
				Type:    common.CrossShardTransferOperation,
				Status:  common.SuccessOperationStatus.Status,
				Account: receiverAccountID,
				Amount: &types.Amount{
					Value:    fmt.Sprintf("%v", cxReceipt.Amount.Uint64()),
					Currency: &common.Currency,
				},
				Metadata: map[string]interface{}{"from_account": senderAccountID},
			},
		},
	}, nil
}

// formatGenesisTransaction for genesis block's initial balances
func formatGenesisTransaction(
	txID *types.TransactionIdentifier, shardID uint32,
) (fmtTx *types.Transaction, rosettaError *types.Error) {
	genesisSpec := getGenesisSpec(shardID)
	for _, tx := range getPseudoTransactionForGenesis(genesisSpec) {
		if tx.To().String() == txID.Hash {
			accID, rosettaError := newAccountIdentifier(*tx.To())
			if rosettaError != nil {
				return nil, rosettaError
			}
			return &types.Transaction{
				TransactionIdentifier: txID,
				Operations: []*types.Operation{
					{
						OperationIdentifier: &types.OperationIdentifier{
							Index: 0,
						},
						Type:    common.TransferOperation,
						Status:  common.SuccessOperationStatus.Status,
						Account: accID,
						Amount: &types.Amount{
							Value:    fmt.Sprintf("%v", tx.Value()),
							Currency: &common.Currency,
						},
						Metadata: map[string]interface{}{
							"type": "genesis funds",
						},
					},
				},
			}, nil
		}
	}
	return nil, &common.TransactionNotFoundError
}

// formatTransaction for staking, cross-shard sender, and plain transactions
func formatTransaction(
	tx hmytypes.PoolTransaction, receipt *hmytypes.Receipt,
) (fmtTx *types.Transaction, rosettaError *types.Error) {
	var operations []*types.Operation
	var isCrossShard bool
	var toShard uint32

	// Fetch correct operations depending on transaction type
	stakingTx, isStaking := tx.(*stakingTypes.StakingTransaction)
	if !isStaking {
		plainTx, ok := tx.(*hmytypes.Transaction)
		if !ok {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": "unknown transaction type",
			})
		}
		operations, rosettaError = getOperations(plainTx, receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard = plainTx.ShardID() != plainTx.ToShardID()
		toShard = plainTx.ToShardID()
	} else {
		operations, rosettaError = getStakingOperations(stakingTx, receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard = false
		toShard = stakingTx.ShardID()
	}
	txID := &types.TransactionIdentifier{Hash: tx.Hash().String()}

	// Set transaction metadata if needed
	var txMetadata TransactionMetadata
	var err error
	if isCrossShard {
		txMetadata.CrossShardIdentifier = txID
		txMetadata.ToShardID = toShard
		txMetadata.FromShardID = tx.ShardID()
	}
	if len(tx.Data()) > 0 && !isStaking {
		txMetadata.Data = hex.EncodeToString(tx.Data())
	}
	metadata, err := rpc.NewStructuredResponse(txMetadata)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	return &types.Transaction{
		TransactionIdentifier: txID,
		Operations:            operations,
		Metadata:              metadata,
	}, nil
}

// getOperations for correct one of the following transactions:
// contract creation, cross-shard sender, same-shard transfer
func getOperations(
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	accountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// All operations excepts for cross-shard tx payout expend gas
	gasExpended := receipt.GasUsed * tx.GasPrice().Uint64()
	gasOperations := newOperations(gasExpended, accountID)

	// Handle different cases of plain transactions
	var txOperations []*types.Operation
	if tx.To() == nil {
		txOperations, rosettaError = newContractCreationOperations(
			gasOperations[0].OperationIdentifier, tx, receipt,
		)
	} else if tx.ShardID() != tx.ToShardID() {
		txOperations, rosettaError = newCrossShardSenderTransferOperations(
			gasOperations[0].OperationIdentifier, tx,
		)
	} else {
		txOperations, rosettaError = newTransferOperations(
			gasOperations[0].OperationIdentifier, tx, receipt,
		)
	}
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(gasOperations, txOperations...), nil
}

// getStakingOperations for all staking directives
func getStakingOperations(
	tx *stakingTypes.StakingTransaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	// Sender address should have errored prior to call this function
	senderAddress, _ := tx.SenderAddress()
	accountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// All operations excepts for cross-shard tx payout expend gas
	gasExpended := receipt.GasUsed * tx.GasPrice().Uint64()
	gasOperations := newOperations(gasExpended, accountID)

	// Format staking message for metadata
	rpcStakingTx, err := rpcV2.NewStakingTransaction(tx, ethcommon.Hash{}, 0, 0, 0)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	metadata, err := rpc.NewStructuredResponse(rpcStakingTx.Msg)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	// Set correct amount depending on staking message directive
	var amount *types.Amount
	switch tx.StakingType() {
	case stakingTypes.DirectiveCreateValidator:
		if amount, rosettaError = getAmountFromCreateValidatorMessage(tx.Data()); rosettaError != nil {
			return nil, rosettaError
		}
	case stakingTypes.DirectiveDelegate:
		if amount, rosettaError = getAmountFromDelegateMessage(tx.Data()); rosettaError != nil {
			return nil, rosettaError
		}
	case stakingTypes.DirectiveUndelegate:
		if amount, rosettaError = getAmountFromUndelegateMessage(tx.Data()); rosettaError != nil {
			return nil, rosettaError
		}
	case stakingTypes.DirectiveCollectRewards:
		if amount, rosettaError = getAmountFromCollectRewards(receipt, senderAddress); rosettaError != nil {
			return nil, rosettaError
		}
	default:
		amount = &types.Amount{
			Value:    fmt.Sprintf("-%v", tx.Value()),
			Currency: &common.Currency,
		}
	}

	return append(gasOperations, &types.Operation{
		OperationIdentifier: &types.OperationIdentifier{
			Index: gasOperations[0].OperationIdentifier.Index + 1,
		},
		RelatedOperations: []*types.OperationIdentifier{
			gasOperations[0].OperationIdentifier,
		},
		Type:     tx.StakingType().String(),
		Status:   common.SuccessOperationStatus.Status,
		Account:  accountID,
		Amount:   amount,
		Metadata: metadata,
	}), nil
}

func getAmountFromCreateValidatorMessage(data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveCreateValidator)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.CreateValidator)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for create validator tx",
		})
	}
	return &types.Amount{
		Value:    fmt.Sprintf("-%v", stkMsg.Amount.Uint64()),
		Currency: &common.Currency,
	}, nil
}

func getAmountFromDelegateMessage(data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveDelegate)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.Delegate)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for delegate tx",
		})
	}
	return &types.Amount{
		Value:    fmt.Sprintf("-%v", stkMsg.Amount.Uint64()),
		Currency: &common.Currency,
	}, nil
}

func getAmountFromUndelegateMessage(data []byte) (*types.Amount, *types.Error) {
	msg, err := stakingTypes.RLPDecodeStakeMsg(data, stakingTypes.DirectiveUndelegate)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	stkMsg, ok := msg.(*stakingTypes.Undelegate)
	if !ok {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unable to parse staking message for undelegate tx",
		})
	}
	return &types.Amount{
		Value:    fmt.Sprintf("%v", stkMsg.Amount.Uint64()),
		Currency: &common.Currency,
	}, nil
}

func getAmountFromCollectRewards(
	receipt *hmytypes.Receipt, senderAddress ethcommon.Address,
) (*types.Amount, *types.Error) {
	var amount *types.Amount
	logs := findLogsWithTopic(receipt, staking.CollectRewardsTopic)
	for _, log := range logs {
		if log.Address == senderAddress {
			amount = &types.Amount{
				Value:    fmt.Sprintf("%v", big.NewInt(0).SetBytes(log.Data).Uint64()),
				Currency: &common.Currency,
			}
			break
		}
	}
	if amount == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": fmt.Sprintf("collect rewards amount not found for %v", senderAddress),
		})
	}
	return amount, nil
}

// newTransferOperations extracts & formats the operation(s) for plain transaction,
// including contract transactions.
func newTransferOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	// Sender address should have errored prior to call this function
	senderAddress, _ := tx.SenderAddress()
	receiverAddress := *tx.To()

	// Common elements
	opType := common.TransferOperation
	opStatus := common.SuccessOperationStatus.Status
	if receipt.Status == hmytypes.ReceiptStatusFailed {
		if len(tx.Data()) > 0 {
			opStatus = common.ContractFailureOperationStatus.Status
		} else {
			// Should never see a failed non-contract related transaction on chain
			opStatus = common.FailureOperationStatus.Status
			utils.Logger().Warn().Msgf("Failed transaction on chain: %v", tx.Hash().String())
		}
	}

	// Subtraction operation elements
	subOperationID := &types.OperationIdentifier{
		Index: startingOperationID.Index + 1,
	}
	subRelatedID := []*types.OperationIdentifier{
		startingOperationID,
	}
	subAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	subAmount := &types.Amount{
		Value:    fmt.Sprintf("-%v", tx.Value()),
		Currency: &common.Currency,
	}

	// Addition operation elements
	addOperationID := &types.OperationIdentifier{
		Index: subOperationID.Index + 1,
	}
	addRelatedID := []*types.OperationIdentifier{
		subOperationID,
	}
	addAccountID, rosettaError := newAccountIdentifier(receiverAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	addAmount := &types.Amount{
		Value:    fmt.Sprintf("%v", tx.Value()),
		Currency: &common.Currency,
	}

	return []*types.Operation{
		{
			OperationIdentifier: subOperationID,
			RelatedOperations:   subRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             subAccountID,
			Amount:              subAmount,
			Metadata:            map[string]interface{}{},
		},
		{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             addAccountID,
			Amount:              addAmount,
			Metadata:            map[string]interface{}{},
		},
	}, nil
}

// newCrossShardSenderTransferOperations extracts & formats the operation(s) for cross-shard-tx
// on the sender's shard.
func newCrossShardSenderTransferOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction,
) ([]*types.Operation, *types.Error) {
	// Sender address should have errored prior to call this function
	senderAddress, _ := tx.SenderAddress()
	receiverAddress := *tx.To()
	senderAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}
	receiverAccountID, rosettaError := newAccountIdentifier(receiverAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOperationID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOperationID,
			},
			Type:    common.CrossShardTransferOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: senderAccountID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("-%v", tx.Value()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{
				"to_account": receiverAccountID,
			},
		},
	}, nil
}

// newContractCreationOperations extracts & formats the operation(s) for a contract creation tx
func newContractCreationOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, txReceipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	// Sender address should have errored prior to call this function
	senderAddress, _ := tx.SenderAddress()
	senderAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	// Set execution status as necessary
	status := common.SuccessOperationStatus.Status
	if txReceipt.Status == hmytypes.ReceiptStatusFailed {
		status = common.ContractFailureOperationStatus.Status
	}

	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: startingOperationID.Index + 1,
			},
			RelatedOperations: []*types.OperationIdentifier{
				startingOperationID,
			},
			Type:    common.ContractCreationOperation,
			Status:  status,
			Account: senderAccountID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("-%v", tx.Value()),
				Currency: &common.Currency,
			},
			Metadata: map[string]interface{}{
				"contract_address": txReceipt.ContractAddress.String(),
			},
		},
	}, nil
}

// AccountMetadata used for account identifiers
type AccountMetadata struct {
	Address string `json:"hex_address"`
}

// newAccountIdentifier ..
func newAccountIdentifier(
	address ethcommon.Address,
) (*types.AccountIdentifier, *types.Error) {
	b32Address, err := internalCommon.AddressToBech32(address)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	metadata, err := rpc.NewStructuredResponse(AccountMetadata{Address: address.String()})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	return &types.AccountIdentifier{
		Address:  b32Address,
		Metadata: metadata,
	}, nil
}

// newOperations creates a new operation with the gas fee as the first operation.
// Note: the gas fee is gasPrice * gasUsed.
func newOperations(
	gasFeeInATTO uint64, accountID *types.AccountIdentifier,
) []*types.Operation {
	return []*types.Operation{
		{
			OperationIdentifier: &types.OperationIdentifier{
				Index: 0, // gas operation is always first
			},
			Type:    common.ExpendGasOperation,
			Status:  common.SuccessOperationStatus.Status,
			Account: accountID,
			Amount: &types.Amount{
				Value:    fmt.Sprintf("-%v", gasFeeInATTO),
				Currency: &common.Currency,
			},
		},
	}
}

// findLogsWithTopic returns all the logs that contain the given receipt
func findLogsWithTopic(
	receipt *hmytypes.Receipt, targetTopic ethcommon.Hash,
) []*hmytypes.Log {
	logs := []*hmytypes.Log{}
	for _, log := range receipt.Logs {
		for _, topic := range log.Topics {
			if topic == targetTopic {
				logs = append(logs, log)
				break
			}
		}
	}
	return logs
}

// getGenesisSpec ..
func getGenesisSpec(shardID uint32) *core.Genesis {
	if shard.Schedule.GetNetworkID() == shardingconfig.MainNet {
		return core.NewGenesisSpec(nodeconfig.Mainnet, shardID)
	}
	return core.NewGenesisSpec(nodeconfig.Testnet, shardID)
}
