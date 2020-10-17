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
	internalCommon "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	shardingconfig "github.com/harmony-one/harmony/internal/configs/sharding"
	"github.com/harmony-one/harmony/rosetta/common"
	"github.com/harmony-one/harmony/rpc"
	"github.com/harmony-one/harmony/shard"
)

// SpecialTransactionSuffix enum for all special transactions
type SpecialTransactionSuffix uint

// Special transaction suffixes that are specific to the rosetta package
const (
	SpecialGenesisTxID SpecialTransactionSuffix = iota
	SpecialPreStakingRewardTxID
	SpecialUndelegationPayoutTxID
)

// Length for special case transaction identifiers
const (
	blockHashStrLen  = 64
	bech32AddrStrLen = 42
)

// String ..
func (s SpecialTransactionSuffix) String() string {
	return [...]string{"genesis", "reward", "undelegation"}[s]
}

// getSpecialCaseTransactionIdentifier fetches 'transaction identifiers' for a given block-hash and suffix.
// Special cases include genesis transactions, pre-staking era block rewards, and undelegation payouts.
// Must include block hash to guarantee uniqueness of tx identifiers.
func getSpecialCaseTransactionIdentifier(
	blockHash ethcommon.Hash, address ethcommon.Address, suffix SpecialTransactionSuffix,
) *types.TransactionIdentifier {
	return &types.TransactionIdentifier{
		Hash: fmt.Sprintf("%v_%v_%v",
			blockHash.String(), internalCommon.MustAddressToBech32(address), suffix.String(),
		),
	}
}

// unpackSpecialCaseTransactionIdentifier returns the suffix & blockHash if the txID is formatted correctly.
func unpackSpecialCaseTransactionIdentifier(
	txID *types.TransactionIdentifier, expectedSuffix SpecialTransactionSuffix,
) (ethcommon.Hash, ethcommon.Address, *types.Error) {
	hash := txID.Hash
	hash = strings.TrimPrefix(hash, "0x")
	hash = strings.TrimPrefix(hash, "0X")
	minCharCount := blockHashStrLen + bech32AddrStrLen + 2
	if len(hash) < minCharCount || string(hash[blockHashStrLen]) != "_" ||
		string(hash[minCharCount-1]) != "_" || expectedSuffix.String() != hash[minCharCount:] {
		return ethcommon.Hash{}, ethcommon.Address{}, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": "unknown special case transaction ID format",
		})
	}
	blkHash := ethcommon.HexToHash(hash[:blockHashStrLen])
	addr := internalCommon.MustBech32ToAddress(hash[blockHashStrLen+1 : minCharCount-1])
	return blkHash, addr, nil
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

	otherTransactions := []*types.TransactionIdentifier{}
	// Report initial genesis funds as transactions to fit API.
	for _, tx := range getPseudoTransactionForGenesis(getGenesisSpec(blk.ShardID())) {
		if tx.To() == nil {
			return nil, common.NewError(common.CatchAllError, nil)
		}
		otherTransactions = append(
			otherTransactions, getSpecialCaseTransactionIdentifier(blk.Hash(), *tx.To(), SpecialGenesisTxID),
		)
	}

	return &types.BlockResponse{
		Block:             responseBlock,
		OtherTransactions: otherTransactions,
	}, nil
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

// specialGenesisBlockTransaction is a special handler for genesis block transactions
func (s *BlockAPI) specialGenesisBlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (response *types.BlockTransactionResponse, rosettaError *types.Error) {
	genesisBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(0).EthBlockNumber())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	blkHash, address, rosettaError := unpackSpecialCaseTransactionIdentifier(
		request.TransactionIdentifier, SpecialGenesisTxID,
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	if blkHash.String() != genesisBlock.Hash().String() {
		return nil, &common.TransactionNotFoundError
	}
	txs, rosettaError := FormatGenesisTransaction(request.TransactionIdentifier, address, s.hmy.ShardID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: txs}, nil
}

// getPreStakingRewardTransactionIdentifiers is only used for the /block endpoint
// rewards for signing block n is paid out on block n+1
func (s *BlockAPI) getPreStakingRewardTransactionIdentifiers(
	ctx context.Context, currBlock *hmytypes.Block,
) ([]*types.TransactionIdentifier, *types.Error) {
	if currBlock.Number().Cmp(big.NewInt(1)) != 1 {
		return nil, nil
	}
	blockNumToBeRewarded := currBlock.Number().Uint64() - 1
	rewardedBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNumToBeRewarded).EthBlockNumber())
	if err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	blockSigInfo, err := s.hmy.GetDetailedBlockSignerInfo(ctx, rewardedBlock)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	txIDs := []*types.TransactionIdentifier{}
	for acc, signedBlsKeys := range blockSigInfo.Signers {
		if len(signedBlsKeys) > 0 {
			txIDs = append(txIDs, getSpecialCaseTransactionIdentifier(currBlock.Hash(), acc, SpecialPreStakingRewardTxID))
		}
	}
	return txIDs, nil
}

// specialBlockTransaction is a formatter for special, non-genesis, transactions
func (s *BlockAPI) specialBlockTransaction(
	ctx context.Context, request *types.BlockTransactionRequest,
) (*types.BlockTransactionResponse, *types.Error) {
	// If no transaction info is found, check for special case transactions.
	blk, rosettaError := s.getBlock(ctx, &types.PartialBlockIdentifier{Index: &request.BlockIdentifier.Index})
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
	blkHash, address, rosettaError := unpackSpecialCaseTransactionIdentifier(txID, SpecialPreStakingRewardTxID)
	if rosettaError != nil {
		return nil, rosettaError
	}
	blockNumOfSigsForReward := blk.Number().Uint64() - 1
	signedBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blockNumOfSigsForReward).EthBlockNumber())
	if err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if blkHash.String() != blk.Hash().String() {
		return nil, common.NewError(common.SanityCheckError, map[string]interface{}{
			"message": fmt.Sprintf(
				"block hash %v != requested block hash %v in tx ID", blkHash.String(), blk.Hash().String(),
			),
		})
	}
	blockSignerInfo, err := s.hmy.GetDetailedBlockSignerInfo(ctx, signedBlock)
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	transactions, rosettaError := FormatPreStakingRewardTransaction(txID, blockSignerInfo, address)
	if rosettaError != nil {
		return nil, rosettaError
	}
	return &types.BlockTransactionResponse{Transaction: transactions}, nil
}

// undelegationPayoutBlockTransaction is a special handler for undelegation payout transactions
func (s *BlockAPI) undelegationPayoutBlockTransaction(
	ctx context.Context, txID *types.TransactionIdentifier, blk *hmytypes.Block,
) (*types.BlockTransactionResponse, *types.Error) {
	blkHash, address, rosettaError := unpackSpecialCaseTransactionIdentifier(txID, SpecialUndelegationPayoutTxID)
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
			TransactionIdentifier: getSpecialCaseTransactionIdentifier(
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
