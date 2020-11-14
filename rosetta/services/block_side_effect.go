package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	hmytypes "github.com/harmony-one/harmony/core/types"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	"github.com/harmony-one/harmony/shard"
)

// containsSideEffectTransaction checks if the block contains any side effect operations to report.
func (s *BlockAPI) containsSideEffectTransaction(
	ctx context.Context, blk *hmytypes.Block,
) bool {
	if blk == nil {
		return false
	}
	return s.hmy.IsCommitteeSelectionBlock(blk.Header()) || !s.hmy.IsStakingEpoch(blk.Epoch()) || blk.NumberU64() == 0
}

const (
	// SideEffectTransactionSuffix is use in the transaction identifier for each block that contains
	// side-effect operations.
	SideEffectTransactionSuffix = "side_effect"
	blockHashStrLen             = 64
)

// getSideEffectTransactionIdentifier fetches 'transaction identifier' for side effect operations
// for a  given block.
// Side effects are genesis funds, pre-staking era block rewards, and undelegation payouts.
// Must include block hash to guarantee uniqueness of tx identifiers.
func getSideEffectTransactionIdentifier(
	blockHash ethcommon.Hash,
) *types.TransactionIdentifier {
	return &types.TransactionIdentifier{
		Hash: fmt.Sprintf("%v_%v",
			blockHash.String(), SideEffectTransactionSuffix,
		),
	}
}

// unpackSideEffectTransactionIdentifier returns the blockHash if the txID is formatted correctly.
func unpackSideEffectTransactionIdentifier(
	txID *types.TransactionIdentifier,
) (ethcommon.Hash, *types.Error) {
	hash := txID.Hash
	hash = strings.TrimPrefix(hash, "0x")
	hash = strings.TrimPrefix(hash, "0X")
	if len(hash) < blockHashStrLen || string(hash[blockHashStrLen]) != "_" ||
		hash[blockHashStrLen+1:] != SideEffectTransactionSuffix {
		return ethcommon.Hash{}, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unknown special case transaction ID format",
		})
	}
	blkHash := ethcommon.HexToHash(hash[:blockHashStrLen])
	return blkHash, nil
}

// genesisBlock is a special handler for the genesis block.
func (s *BlockAPI) genesisBlock(
	ctx context.Context, request *types.BlockRequest, blk *hmytypes.Block,
) (response *types.BlockResponse, rosettaError *types.Error) {
	var currBlockID, prevBlockID *types.BlockIdentifier
	currBlockID = &types.BlockIdentifier{
		Index: blk.Number().Int64(),
		Hash:  blk.Hash().String(),
	}
	prevBlockID = currBlockID

	metadata, err := types.MarshalMap(BlockMetadata{
		Epoch: blk.Epoch(),
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	responseBlock := &types.Block{
		BlockIdentifier:       currBlockID,
		ParentBlockIdentifier: prevBlockID,
		Timestamp:             blk.Time().Int64() * 1e3, // Timestamp must be in ms.
		Transactions:          []*types.Transaction{},   // Do not return tx details as it is optional.
		Metadata:              metadata,
	}

	// Report initial genesis funds as transactions to fit API.
	otherTransactions := []*types.TransactionIdentifier{
		getSideEffectTransactionIdentifier(blk.Hash()),
	}

	return &types.BlockResponse{
		Block:             responseBlock,
		OtherTransactions: otherTransactions,
	}, nil
}

// genesisBlockTransaction is a special handler for genesis block transactions
func (s *BlockAPI) genesisBlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (response *types.BlockTransactionResponse, rosettaError *types.Error) {
	genesisBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(0).EthBlockNumber())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	blkHash, rosettaError := unpackSideEffectTransactionIdentifier(
		request.TransactionIdentifier,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if blkHash.String() != genesisBlock.Hash().String() {
		return nil, &common.TransactionNotFoundError
	}

	// The only side effect for the genesis block are the initial funds derived from the genesis spec.
	genesisSideEffectOperations, rosettaError := GetSideEffectOperationsFromGenesisSpec(
		getGenesisSpec(s.hmy.ShardID), nil,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}

	return &types.BlockTransactionResponse{Transaction: &types.Transaction{
		TransactionIdentifier: request.TransactionIdentifier,
		Operations:            genesisSideEffectOperations,
	}}, nil
}

// getSideEffectTransaction returns the side effect transaction for a block if said block has one.
// Side effects to reports are: genesis funds, undelegation payouts, permissioned-phase block rewards.
func (s *BlockAPI) getSideEffectTransaction(
	ctx context.Context, blk *hmytypes.Block,
) (*types.Transaction, *types.Error) {
	if !s.containsSideEffectTransaction(ctx, blk) {
		return nil, common.NewError(common.TransactionNotFoundError, map[string]interface{}{
			"message": "no side effect transaction found for given block",
		})
	}

	var startingOpIndex *types.OperationIdentifier
	txOperations := []*types.Operation{}
	updateStartingOpIndex := func(newOperations []*types.Operation) {
		if len(newOperations) > 0 {
			startingOpIndex = &types.OperationIdentifier{
				Index: newOperations[len(newOperations)-1].OperationIdentifier.Index + 1,
			}
		}
		txOperations = append(txOperations, newOperations...)
	}

	// Handle genesis funds
	if blk.NumberU64() == 0 {
		ops, rosettaError := GetSideEffectOperationsFromGenesisSpec(getGenesisSpec(s.hmy.ShardID), startingOpIndex)
		if rosettaError != nil {
			return nil, rosettaError
		}
		updateStartingOpIndex(ops)
	}
	// Handle undelegation payout
	if s.hmy.IsCommitteeSelectionBlock(blk.Header()) && s.hmy.IsPreStakingEpoch(blk.Epoch()) {
		payouts, err := s.hmy.GetUndelegationPayouts(ctx, blk.Epoch())
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		ops, rosettaError := GetSideEffectOperationsFromUndelegationPayouts(payouts, startingOpIndex)
		if rosettaError != nil {
			return nil, rosettaError
		}
		updateStartingOpIndex(ops)
	}
	// Handle block rewards for epoch < staking epoch (permissioned-phase block rewards)
	if !s.hmy.IsStakingEpoch(blk.Epoch()) {
		rewards, err := s.hmy.GetPreStakingBlockRewards(ctx, blk)
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		ops, rosettaError := GetSideEffectOperationsFromPreStakingRewards(rewards, startingOpIndex)
		if rosettaError != nil {
			return nil, rosettaError
		}
		updateStartingOpIndex(ops)
	}

	return &types.Transaction{
		TransactionIdentifier: getSideEffectTransactionIdentifier(blk.Hash()),
		Operations:            txOperations,
	}, nil
}

// specialBlockTransaction is a formatter for special, non-genesis, transactions
func (s *BlockAPI) specialBlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	// If no transaction info is found, check for special case transactions.
	blk, rosettaError := getBlock(ctx, s.hmy, &types.PartialBlockIdentifier{Index: &request.BlockIdentifier.Index})
	if rosettaError != nil {
		return nil, rosettaError
	}
	tx, rosettaError := s.getSideEffectTransaction(ctx, blk)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: tx}, nil
}

// getGenesisSpec ..
func getGenesisSpec(shardID uint32) *core.Genesis {
	if shard.Schedule.GetNetworkID() == shardingconfig.MainNet {
		return core.NewGenesisSpec(nodeconfig.Mainnet, shardID)
	}
	if shard.Schedule.GetNetworkID() == shardingconfig.LocalNet {
		return core.NewGenesisSpec(nodeconfig.Localnet, shardID)
	}
	return core.NewGenesisSpec(nodeconfig.Testnet, shardID)
}
