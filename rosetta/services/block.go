package services

import (
	"context"
	"fmt"
	"time"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	eth_common "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/rawdb"
	hmy_types "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
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

// BlockTransaction implements the /block/transaction endpoint (placeholder)
func (s *BlockAPIService) BlockTransaction(
	ctx context.Context,
	request *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	txHash := eth_common.HexToHash(request.TransactionIdentifier.Hash)
	requestBlockHash := eth_common.HexToHash(request.BlockIdentifier.Hash)

	// Note: Use pool transaction interface to handle both plain & staking transactions
	var tx hmy_types.PoolTransaction
	tx, blockHash, _, _ := rawdb.ReadTransaction(s.hmy.ChainDb(), txHash)
	if tx == nil {
		tx, blockHash, _, _ = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), txHash)
	}

	if tx == nil {
		return nil, &common.TransactionNotFoundError
	}
	if requestBlockHash != blockHash {
		rosettaError := common.SanityCheckError
		rosettaError.Details = map[string]interface{}{
			"message": fmt.Sprintf(
				"Given block hash of %v != transaction block hash %v",
				requestBlockHash.String(), blockHash.String(),
			),
		}
		return nil, &rosettaError
	}

	transaction, rosettaError := formatTransaction(tx)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: transaction}, nil
}

// TransactionMetadata ..
type TransactionMetadata struct {
	CrossShardIdentifier *types.TransactionIdentifier `json:"cross_shard_transaction_identifier"`
	ToShardID            uint32                       `json:"to_shard"`
}

func formatTransaction(
	tx hmy_types.PoolTransaction,
) (txs *types.Transaction, rosettaError *types.Error) {
	var operations []*types.Operation
	var isCrossShard bool
	var toShard uint32

	stakingTx, isStaking := tx.(*staking.StakingTransaction)
	if !isStaking {
		plainTx, ok := tx.(*hmy_types.Transaction)
		if !ok {
			rosettaError := common.CatchAllError
			rosettaError.Details = map[string]interface{}{
				"message": "unknown transaction type",
			}
			return nil, &rosettaError
		}
		operations, rosettaError = getOperationsFromTransaction(plainTx)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard = plainTx.ShardID() != plainTx.ToShardID()
		toShard = plainTx.ToShardID()
	} else {
		operations, rosettaError = getOperationsFromStakingTransaction(stakingTx)
		if rosettaError != nil {
			return nil, rosettaError
		}
		isCrossShard = false
		toShard = stakingTx.ShardID()
	}

	txID := &types.TransactionIdentifier{Hash: tx.Hash().String()}
	var ctxID *types.TransactionIdentifier
	if isCrossShard {
		ctxID = txID
	}

	metadata, err := rpc.NewStructuredResponse(TransactionMetadata{
		CrossShardIdentifier: ctxID,
		ToShardID:            toShard,
	})
	if err != nil {
		rosettaError := common.CatchAllError
		rosettaError.Details = map[string]interface{}{
			"message": err.Error(),
		}
		return nil, &rosettaError
	}

	return &types.Transaction{
		TransactionIdentifier: txID,
		Operations:            operations,
		Metadata:              metadata,
	}, nil
}

func getOperationsFromStakingTransaction(tx *staking.StakingTransaction) ([]*types.Operation, *types.Error) {
	// TODO: implement
}

func getOperationsFromTransaction(tx *hmy_types.Transaction) ([]*types.Operation, *types.Error) {
	// TODO: implement
}
