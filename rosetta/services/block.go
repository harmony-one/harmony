package services

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/rawdb"
	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	staking "github.com/harmony-one/harmony/staking/types"
)

// BlockAPIService implements the server.BlockAPIServicer interface.
type BlockAPIService struct {
	hmy *hmy.Harmony
}

// NewBlockAPIService creates a new instance of a BlockAPIService.
func NewBlockAPIService(hmy *hmy.Harmony) server.BlockAPIServicer {
	return &BlockAPIService{
		hmy: hmy,
	}
}

// Block implements the /block endpoint (placeholder)
// FIXME: remove placeholder & implement block endpoint
func (s *BlockAPIService) Block(
	ctx context.Context,
	request *types.BlockRequest,
) (*types.BlockResponse, *types.Error) {
	if *request.BlockIdentifier.Index != 1000 {
		previousBlockIndex := *request.BlockIdentifier.Index - 1
		if previousBlockIndex < 0 {
			previousBlockIndex = 0
		}

		return &types.BlockResponse{
			Block: &types.Block{
				BlockIdentifier: &types.BlockIdentifier{
					Index: *request.BlockIdentifier.Index,
					Hash:  fmt.Sprintf("block %d", *request.BlockIdentifier.Index),
				},
				ParentBlockIdentifier: &types.BlockIdentifier{
					Index: previousBlockIndex,
					Hash:  fmt.Sprintf("block %d", previousBlockIndex),
				},
				Timestamp:    time.Now().UnixNano() / 1000000,
				Transactions: []*types.Transaction{},
			},
		}, nil
	}

	return &types.BlockResponse{
		Block: &types.Block{
			BlockIdentifier: &types.BlockIdentifier{
				Index: 1000,
				Hash:  "block 1000",
			},
			ParentBlockIdentifier: &types.BlockIdentifier{
				Index: 999,
				Hash:  "block 999",
			},
			Timestamp: 1586483189000,
			Transactions: []*types.Transaction{
				{
					TransactionIdentifier: &types.TransactionIdentifier{
						Hash: "transaction 0",
					},
					Operations: []*types.Operation{
						{
							OperationIdentifier: &types.OperationIdentifier{
								Index: 0,
							},
							Type:   "Transfer",
							Status: "Success",
							Account: &types.AccountIdentifier{
								Address: "account 0",
							},
							Amount: &types.Amount{
								Value: "-1000",
								Currency: &types.Currency{
									Symbol:   "ROS",
									Decimals: 2,
								},
							},
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
							Type:   "Transfer",
							Status: "Reverted",
							Account: &types.AccountIdentifier{
								Address: "account 1",
							},
							Amount: &types.Amount{
								Value: "1000",
								Currency: &types.Currency{
									Symbol:   "ROS",
									Decimals: 2,
								},
							},
						},
					},
				},
			},
		},
		OtherTransactions: []*types.TransactionIdentifier{
			{
				Hash: "transaction 1",
			},
		},
	}, nil
}

// BlockTransaction implements the /block/transaction endpoint
func (s *BlockAPIService) BlockTransaction(
	ctx context.Context,
	request *types.BlockTransactionRequest,
) (response *types.BlockTransactionResponse, rosettaError *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	txHash := ethcommon.HexToHash(request.TransactionIdentifier.Hash)
	requestBlockHash := ethcommon.HexToHash(request.BlockIdentifier.Hash)

	// Look for all of the possible transactions
	// Note: Use pool transaction interface to handle both plain & staking transactions
	var tx hmytypes.PoolTransaction
	tx, blockHash, _, index := rawdb.ReadTransaction(s.hmy.ChainDb(), txHash)
	if tx == nil {
		tx, blockHash, _, index = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), txHash)
	}
	cxReceipt, blockHash, _, _ := rawdb.ReadCXReceipt(s.hmy.ChainDb(), txHash)

	if tx == nil && cxReceipt == nil {
		return nil, &common.TransactionNotFoundError
	}
	if requestBlockHash != blockHash {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": fmt.Sprintf(
				"Given block hash of %v != transaction block hash %v",
				requestBlockHash.String(), blockHash.String(),
			),
		})
	}

	var transaction *types.Transaction
	if tx != nil {
		receipt, rosettaError := s.getTransactionReceiptFromIndex(ctx, blockHash, index)
		if rosettaError != nil {
			return nil, rosettaError
		}
		transaction, rosettaError = formatTransaction(tx, receipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else {
		transaction, rosettaError = formatCrossShardReceiverTransaction(cxReceipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	}
	return &types.BlockTransactionResponse{Transaction: transaction}, nil
}

func (s *BlockAPIService) getTransactionReceiptFromIndex(
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

// TransactionMetadata ..
type TransactionMetadata struct {
	CrossShardIdentifier *types.TransactionIdentifier `json:"cross_shard_transaction_identifier,omitempty"`
	ToShardID            uint32                       `json:"to_shard,omitempty"`
	FromShardID          uint32                       `json:"from_shard,omitempty"`
	Data                 string                       `json:"data,omitempty"`
}

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
					Currency: common.Currency,
				},
				Metadata: map[string]interface{}{"account": senderAccountID},
			},
		},
	}, nil
}

func formatTransaction(
	tx hmytypes.PoolTransaction, receipt *hmytypes.Receipt,
) (txs *types.Transaction, rosettaError *types.Error) {
	var operations []*types.Operation
	var isCrossShard bool
	var toShard uint32

	stakingTx, isStaking := tx.(*staking.StakingTransaction)
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

	var ctxID *types.TransactionIdentifier
	var metadata map[string]interface{}
	var err error
	if isCrossShard {
		metadata, err = rpc.NewStructuredResponse(TransactionMetadata{
			CrossShardIdentifier: ctxID,
			ToShardID:            toShard,
			FromShardID:          tx.ShardID(),
		})
	} else if len(tx.Data()) > 0 {
		metadata, err = rpc.NewStructuredResponse(TransactionMetadata{
			Data: hex.EncodeToString(tx.Data()),
		})
	}
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
	operations := newOperations(gasExpended, accountID)

	var txOperations []*types.Operation
	if tx.To() == nil {
		txOperations, rosettaError = newContractCreationOperations(
			operations[0].OperationIdentifier, tx, receipt,
		)
	} else if tx.ShardID() != tx.ToShardID() {
		txOperations, rosettaError = newCrossShardSenderTransferOperations(
			operations[0].OperationIdentifier, tx, receipt,
		)
	} else {
		txOperations, rosettaError = newTransferOperations(
			operations[0].OperationIdentifier, tx, receipt,
		)
	}
	if rosettaError != nil {
		return nil, rosettaError
	}

	return append(operations, txOperations...), nil
}

func newTransferOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	receiverAddress := *tx.To()

	// Common elements
	opType := common.TransferOperation
	opStatus := common.SuccessOperationStatus.Status
	if receipt.Status == hmytypes.ReceiptStatusFailed {
		if len(tx.Data()) > 0 {
			// TODO (dm): Ensure arbitrary data won't return invalid receipt status
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
		Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
		Currency: common.Currency,
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
		Value:    fmt.Sprintf("%v", tx.Value().Uint64()),
		Currency: common.Currency,
	}

	return []*types.Operation{
		{
			OperationIdentifier: subOperationID,
			RelatedOperations:   subRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             subAccountID,
			Amount:              subAmount,
		},
		{
			OperationIdentifier: addOperationID,
			RelatedOperations:   addRelatedID,
			Type:                opType,
			Status:              opStatus,
			Account:             addAccountID,
			Amount:              addAmount,
		},
	}, nil
}

func newCrossShardSenderTransferOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
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
				Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
				Currency: common.Currency,
			},
			Metadata: map[string]interface{}{
				"account": receiverAccountID,
			},
		},
	}, nil
}

func newContractCreationOperations(
	startingOperationID *types.OperationIdentifier,
	tx *hmytypes.Transaction, receipt *hmytypes.Receipt,
) ([]*types.Operation, *types.Error) {
	senderAddress, err := tx.SenderAddress()
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	senderAccountID, rosettaError := newAccountIdentifier(senderAddress)
	if rosettaError != nil {
		return nil, rosettaError
	}

	status := common.SuccessOperationStatus.Status
	if receipt.Status == hmytypes.ReceiptStatusFailed {
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
				Value:    fmt.Sprintf("-%v", tx.Value().Uint64()),
				Currency: common.Currency,
			},
			Metadata: map[string]interface{}{
				"contract_address": receipt.ContractAddress.String(),
			},
		},
	}, nil
}

func getStakingOperations(
	tx *staking.StakingTransaction, receipt *hmytypes.Receipt,
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

	gasExpended := receipt.GasUsed * tx.GasPrice().Uint64()
	oeprations := newOperations(gasExpended, accountID)

	// TODO: finish implementing...

	return []*types.Operation{}, nil
}

// AccountMetadata ..
type AccountMetadata struct {
	Address string `json:"hex_address"`
}

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

func newOperations(
	gasExpended uint64, accountID *types.AccountIdentifier,
) []*types.Operation {
	gasOperationID := &types.OperationIdentifier{
		Index: 0, // gas operation is always first
	}
	gasType := common.ExpendGasOperation
	status := common.SuccessOperationStatus.Status
	amount := &types.Amount{
		Value:    fmt.Sprintf("-%v", gasExpended),
		Currency: common.Currency,
	}
	return []*types.Operation{
		{
			OperationIdentifier: gasOperationID,
			Type:                gasType,
			Status:              status,
			Account:             accountID,
			Amount:              amount,
		},
	}
}
