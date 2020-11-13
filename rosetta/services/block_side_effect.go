package services

import (
	"context"
	"fmt"
	"math/big"
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

// containsSpecialTransaction checks if the block contains any side effect operations to report.
func (s *BlockAPI) containsSpecialTransaction(
	ctx context.Context, blk *hmytypes.Block,
) bool {
	if blk == nil {
		return false
	}
	return !s.hmy.IsStakingEpoch(blk.Epoch()) || s.hmy.IsCommitteeSelectionBlock(blk.Header())
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

// getPreStakingRewardTransactionIdentifiers is only used for the /block endpoint
func (s *BlockAPI) getPreStakingRewardTransactionIdentifiers(
	ctx context.Context, currBlock *hmytypes.Block,
) ([]*types.TransactionIdentifier, *types.Error) {
	if currBlock.Number().Cmp(big.NewInt(1)) != 1 {
		return nil, nil
	}
	rewards, err := s.hmy.GetPreStakingBlockRewards(ctx, currBlock)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	txIDs := []*types.TransactionIdentifier{}
	for addr := range rewards {
		txIDs = append(txIDs, getSideEffectTransactionIdentifier(
			currBlock.Hash(), addr, SpecialPreStakingRewardTxID,
		))
	}
	return txIDs, nil
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
	if s.hmy.IsCommitteeSelectionBlock(blk.Header()) {
		// Note that undelegation payout MUST be checked before reporting error in pre-staking & staking era.
		response, rosettaError := s.undelegationPayoutBlockTransaction(ctx, request.TransactionIdentifier, blk)
		if rosettaError != nil && !s.hmy.IsStakingEpoch(blk.Epoch()) && s.hmy.IsPreStakingEpoch(blk.Epoch()) {
			// Handle edge case special transaction for pre-staking era
			return s.preStakingRewardBlockTransaction(ctx, request.TransactionIdentifier, blk)
		}
		return response, rosettaError
	}
	if !s.hmy.IsStakingEpoch(blk.Epoch()) {
		return s.preStakingRewardBlockTransaction(ctx, request.TransactionIdentifier, blk)
	}
	return nil, &common.TransactionNotFoundError
}

// preStakingRewardBlockTransaction is a special handler for pre-staking era
func (s *BlockAPI) preStakingRewardBlockTransaction(
	ctx context.Context, txID *types.TransactionIdentifier, blk *hmytypes.Block,
) (*types.BlockTransactionResponse, *types.Error) {
	if blk.Number().Cmp(big.NewInt(1)) != 1 {
		return nil, common.NewError(common.TransactionNotFoundError, map[string]interface{}{
			"message": "block does not contain any pre-staking era block rewards",
		})
	}
	blkHash, address, rosettaError := unpackSideEffectTransactionIdentifier(txID, SpecialPreStakingRewardTxID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if blkHash.String() != blk.Hash().String() {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": fmt.Sprintf(
				"block hash %v != requested block hash %v in tx ID", blkHash.String(), blk.Hash().String(),
			),
		})
	}
	rewards, err := s.hmy.GetPreStakingBlockRewards(ctx, blk)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	transactions, rosettaError := FormatPreStakingRewardTransaction(txID, rewards, address)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: transactions}, nil
}

// undelegationPayoutBlockTransaction is a special handler for undelegation payout transactions
func (s *BlockAPI) undelegationPayoutBlockTransaction(
	ctx context.Context, txID *types.TransactionIdentifier, blk *hmytypes.Block,
) (*types.BlockTransactionResponse, *types.Error) {
	blkHash, address, rosettaError := unpackSideEffectTransactionIdentifier(txID, SpecialUndelegationPayoutTxID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if blkHash.String() != blk.Hash().String() {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": fmt.Sprintf(
				"block hash %v != requested block hash %v in tx ID", blkHash.String(), blk.Hash().String(),
			),
		})
	}

	delegatorPayouts, err := s.hmy.GetUndelegationPayouts(ctx, blk.Epoch())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	transactions, rosettaError := FormatUndelegationPayoutTransaction(txID, delegatorPayouts, address)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: transactions}, nil
}

// getAllUndelegationPayoutTransactions is only used for the /block endpoint
func (s *BlockAPI) getAllUndelegationPayoutTransactions(
	ctx context.Context, blk *hmytypes.Block,
) ([]*types.Transaction, *types.Error) {
	if !s.hmy.IsCommitteeSelectionBlock(blk.Header()) {
		return []*types.Transaction{}, nil
	}

	delegatorPayouts, err := s.hmy.GetUndelegationPayouts(ctx, blk.Epoch())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	transactions := []*types.Transaction{}
	for delegator, payout := range delegatorPayouts {
		accID, rosettaError := newAccountIdentifier(delegator)
		if rosettaError != nil {
			return nil, rosettaError
		}
		transactions = append(transactions, &types.Transaction{
			TransactionIdentifier: getSideEffectTransactionIdentifier(
				blk.Hash(), delegator, SpecialUndelegationPayoutTxID,
			),
			Operations: []*types.Operation{
				{
					OperationIdentifier: &types.OperationIdentifier{
						Index: 0, // There is no gas expenditure for undelegation payout
					},
					Type:    common.UndelegationPayoutOperation,
					Status:  common.SuccessOperationStatus.Status,
					Account: accID,
					Amount: &types.Amount{
						Value:    payout.String(),
						Currency: &common.NativeCurrency,
					},
				},
			},
		})
	}
	return transactions, nil
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
