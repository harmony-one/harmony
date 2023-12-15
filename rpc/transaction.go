package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/accounts/abi"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	eth "github.com/harmony-one/harmony/rpc/eth"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	defaultPageSize = uint32(100)
)

// PublicTransactionService provides an API to access Harmony's transaction service.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicTransactionService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicTransactionAPI creates a new API for the RPC interface
func NewPublicTransactionAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicTransactionService{hmy, version},
		Public:    true,
	}
}

// GetAccountNonce returns the nonce value of the given address for the given block number
func (s *PublicTransactionService) GetAccountNonce(
	ctx context.Context, address string, blockNumber BlockNumber,
) (uint64, error) {
	timer := DoMetricRPCRequest(GetAccountNonce)
	defer DoRPCRequestDuration(GetAccountNonce, timer)

	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Response output is the same for all versions
	addr, err := internal_common.ParseAddr(address)
	if err != nil {
		return 0, err
	}
	return s.hmy.GetAccountNonce(ctx, addr, blockNum)
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number.
// Legacy for apiv1. For apiv2, please use getAccountNonce/getPoolNonce/getTransactionsCount/getStakingTransactionsCount apis for
// more granular transaction counts queries
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetTransactionCount(
	ctx context.Context, addr string, blockNrOrHash rpc.BlockNumberOrHash,
) (response interface{}, err error) {
	timer := DoMetricRPCRequest(GetTransactionCount)
	defer DoRPCRequestDuration(GetTransactionCount, timer)

	address, err := internal_common.ParseAddr(addr)
	if err != nil {
		return nil, err
	}

	// Fetch transaction count
	var nonce uint64
	if blockNr, ok := blockNrOrHash.Number(); ok && blockNr == rpc.PendingBlockNumber {
		// Ask transaction pool for the nonce which includes pending transactions
		nonce, err = s.hmy.GetPoolNonce(ctx, address)
		if err != nil {
			return nil, err
		}
	} else {
		// Resolve block number and use its state to ask for the nonce
		state, _, err := s.hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
		if err != nil {
			return nil, err
		}
		if state == nil {
			return nil, fmt.Errorf("state not found")
		}
		if state.Error() != nil {
			return nil, state.Error()
		}
		nonce = state.GetNonce(address)
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return (hexutil.Uint64)(nonce), nil
	case V2:
		return nonce, nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionsCount returns the number of regular transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionService) GetTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	timer := DoMetricRPCRequest(GetTransactionsCount)
	defer DoRPCRequestDuration(GetTransactionsCount, timer)

	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr, err := internal_common.ParseAddr(address)
		if err != nil {
			return 0, err
		}
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.hmy.GetTransactionsCount(address, txType)
}

// GetStakingTransactionsCount returns the number of staking transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionService) GetStakingTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	timer := DoMetricRPCRequest(GetStakingTransactionsCount)
	defer DoRPCRequestDuration(GetStakingTransactionsCount, timer)

	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr, err := internal_common.ParseAddr(address)
		if err != nil {
			return 0, err
		}
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.hmy.GetStakingTransactionsCount(address, txType)
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicTransactionService) EstimateGas(
	ctx context.Context, args CallArgs, blockNrOrHash *rpc.BlockNumberOrHash,
) (hexutil.Uint64, error) {
	timer := DoMetricRPCRequest(RpcEstimateGas)
	defer DoRPCRequestDuration(RpcEstimateGas, timer)
	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	gas, err := EstimateGas(ctx, s.hmy, args, bNrOrHash, nil)
	if err != nil {
		return 0, err
	}

	// Response output is the same for all versions
	return (hexutil.Uint64)(gas), nil
}

// GetTransactionByHash returns the plain transaction for the given hash
func (s *PublicTransactionService) GetTransactionByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetTransactionByHash)
	defer DoRPCRequestDuration(GetTransactionByHash, timer)
	// Try to return an already finalized transaction
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.hmy.ChainDb(), hash)
	if tx == nil {
		// Try to return a pending transaction
		if tx := s.hmy.TxPool.Get(hash); tx != nil {
			if plainTx, ok := tx.(*types.Transaction); ok {
				return s.newRPCTransaction(plainTx, common.Hash{}, 0, 0, 0)
			}
		}

		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		DoMetricRPCQueryInfo(GetTransactionByHash, FailedNumber)
		return nil, nil
	}
	block, err := s.hmy.GetHeader(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		DoMetricRPCQueryInfo(GetTransactionByHash, FailedNumber)
		return nil, nil
	}

	return s.newRPCTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
}

func (s *PublicTransactionService) newRPCTransaction(tx *types.Transaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64) (StructuredResponse, error) {

	// Format the response according to the version
	switch s.version {
	case V1:
		tx, err := v1.NewTransaction(tx, blockHash, blockNumber, timestamp, index)
		if err != nil {
			DoMetricRPCQueryInfo(GetTransactionByHash, FailedNumber)
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransaction(tx, blockHash, blockNumber, timestamp, index)
		if err != nil {
			DoMetricRPCQueryInfo(GetTransactionByHash, FailedNumber)
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransaction(tx.ConvertToEth(), blockHash, blockNumber, timestamp, index)
		if err != nil {
			DoMetricRPCQueryInfo(GetTransactionByHash, FailedNumber)
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByHash returns the staking transaction for the given hash
func (s *PublicTransactionService) GetStakingTransactionByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetStakingTransactionByHash)
	defer DoRPCRequestDuration(GetStakingTransactionByHash, timer)

	// Try to return an already finalized transaction
	stx, blockHash, blockNumber, index := rawdb.ReadStakingTransaction(s.hmy.ChainDb(), hash)
	if stx == nil {
		// Try to return a pending transaction
		if tx := s.hmy.TxPool.Get(hash); tx != nil {
			if stx, ok := tx.(*staking.StakingTransaction); ok {
				return s.newRPCStakingTransaction(stx, common.Hash{}, 0, 0, 0)
			}
		}

		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		DoMetricRPCQueryInfo(GetStakingTransactionByHash, FailedNumber)
		return nil, nil
	}
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		DoMetricRPCQueryInfo(GetStakingTransactionByHash, FailedNumber)
		return nil, nil
	}

	return s.newRPCStakingTransaction(stx, blockHash, blockNumber, block.Time().Uint64(), index)
}

func (s *PublicTransactionService) newRPCStakingTransaction(stx *staking.StakingTransaction, blockHash common.Hash,
	blockNumber uint64, timestamp uint64, index uint64) (StructuredResponse, error) {

	switch s.version {
	case V1:
		tx, err := v1.NewStakingTransaction(stx, blockHash, blockNumber, timestamp, index)
		if err != nil {
			DoMetricRPCQueryInfo(GetStakingTransactionByHash, FailedNumber)
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransaction(stx, blockHash, blockNumber, timestamp, index, true)
		if err != nil {
			DoMetricRPCQueryInfo(GetStakingTransactionByHash, FailedNumber)
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionService) GetTransactionsHistory(
	ctx context.Context, args TxHistoryArgs,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetTransactionsHistory)
	defer DoRPCRequestDuration(GetTransactionsHistory, timer)
	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr, err := internal_common.ParseAddr(args.Address)
		if err != nil {
			DoMetricRPCQueryInfo(GetTransactionsHistory, FailedNumber)
			return nil, err
		}
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			DoMetricRPCQueryInfo(GetTransactionsHistory, FailedNumber)
			return nil, err
		}
	}
	hashes, err := s.hmy.GetTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		DoMetricRPCQueryInfo(GetTransactionsHistory, FailedNumber)
		return nil, err
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return StructuredResponse{"transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []StructuredResponse{}
	for _, hash := range result {
		tx, err := s.GetTransactionByHash(ctx, hash)
		if err == nil {
			txs = append(txs, tx)
		} else {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetTransactionsHistory")
			// Legacy behavior is to not return RPC errors
		}
	}
	return StructuredResponse{"transactions": txs}, nil
}

// GetStakingTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionService) GetStakingTransactionsHistory(
	ctx context.Context, args TxHistoryArgs,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetStakingTransactionsHistory)
	defer DoRPCRequestDuration(GetStakingTransactionsHistory, timer)

	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr, err := internal_common.ParseAddr(args.Address)
		if err != nil {
			DoMetricRPCQueryInfo(GetStakingTransactionsHistory, FailedNumber)
			return nil, err
		}
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
			// Legacy behavior is to not return RPC errors
			DoMetricRPCQueryInfo(GetStakingTransactionsHistory, FailedNumber)
			return nil, nil
		}
	}
	hashes, err := s.hmy.GetStakingTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
		// Legacy behavior is to not return RPC errors
		DoMetricRPCQueryInfo(GetStakingTransactionsHistory, FailedNumber)
		return nil, nil
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return StructuredResponse{"staking_transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []StructuredResponse{}
	for _, hash := range result {
		tx, err := s.GetStakingTransactionByHash(ctx, hash)
		if err == nil {
			txs = append(txs, tx)
		} else {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
			// Legacy behavior is to not return RPC errors
		}
	}
	return StructuredResponse{"staking_transactions": txs}, nil
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockTransactionCountByNumber)
	defer DoRPCRequestDuration(GetBlockTransactionCountByNumber, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	block, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockTransactionCountByNumber")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockTransactionCountByHash)
	defer DoRPCRequestDuration(GetBlockTransactionCountByHash, timer)

	// Fetch block
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockTransactionCountByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionService) GetTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetTransactionByBlockNumberAndIndex)
	defer DoRPCRequestDuration(GetTransactionByBlockNumberAndIndex, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch Block
	block, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByBlockNumberAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionService) GetTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetTransactionByBlockHashAndIndex)
	defer DoRPCRequestDuration(GetTransactionByBlockHashAndIndex, timer)

	// Fetch Block
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByBlockHashAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case Eth:
		tx, err := eth.NewTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockStakingTransactionCountByNumber returns the number of staking transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockStakingTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockStakingTransactionCountByNumber)
	defer DoRPCRequestDuration(GetBlockStakingTransactionCountByNumber, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch block
	block, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockStakingTransactionCountByNumber")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetBlockStakingTransactionCountByHash returns the number of staking transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionService) GetBlockStakingTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
	timer := DoMetricRPCRequest(GetBlockStakingTransactionCountByHash)
	defer DoRPCRequestDuration(GetBlockStakingTransactionCountByHash, timer)

	// Fetch block
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetBlockStakingTransactionCountByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByBlockNumberAndIndex returns the staking transaction for the given block number and index.
func (s *PublicTransactionService) GetStakingTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetStakingTransactionByBlockNumberAndIndex)
	defer DoRPCRequestDuration(GetStakingTransactionByBlockNumberAndIndex, timer)

	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch Block
	block, err := s.hmy.BlockByNumber(ctx, blockNum)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByBlockNumberAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1:
		tx, err := v1.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetStakingTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionService) GetStakingTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetStakingTransactionByBlockHashAndIndex)
	defer DoRPCRequestDuration(GetStakingTransactionByBlockHashAndIndex, timer)

	// Fetch Block
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByBlockHashAndIndex")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format response according to version
	switch s.version {
	case V1, Eth:
		tx, err := v1.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionService) GetTransactionReceipt(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetTransactionReceipt)
	defer DoRPCRequestDuration(GetTransactionReceipt, timer)

	// Fetch receipt for plain & staking transaction
	var tx *types.Transaction
	var stx *staking.StakingTransaction
	var blockHash common.Hash
	var blockNumber, index uint64
	tx, blockHash, blockNumber, index = rawdb.ReadTransaction(s.hmy.ChainDb(), hash)
	if tx == nil {
		stx, blockHash, blockNumber, index = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), hash)
		if stx == nil {
			return nil, nil
		}
		// if there both normal and staking transactions, add to index
		if block, _ := s.hmy.GetBlock(ctx, blockHash); block != nil {
			index = index + uint64(block.Transactions().Len())
		}
	}
	receipts, err := s.hmy.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	// Format response according to version
	var RPCReceipt interface{}
	switch s.version {
	case V1:
		if tx == nil {
			RPCReceipt, err = v1.NewReceipt(stx, blockHash, blockNumber, index, receipt)
		} else {
			RPCReceipt, err = v1.NewReceipt(tx, blockHash, blockNumber, index, receipt)
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(RPCReceipt)
	case V2, Eth:
		if tx == nil {
			RPCReceipt, err = v2.NewReceipt(stx, blockHash, blockNumber, index, receipt, false)
		} else {
			RPCReceipt, err = v2.NewReceipt(tx, blockHash, blockNumber, index, receipt, s.version == Eth)
		}
		if err != nil {
			return nil, err
		}
		return NewStructuredResponse(RPCReceipt)
	default:
		return nil, ErrUnknownRPCVersion
	}
}

// GetCXReceiptByHash returns the transaction for the given hash
func (s *PublicTransactionService) GetCXReceiptByHash(
	ctx context.Context, hash common.Hash,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetCXReceiptByHash)
	defer DoRPCRequestDuration(GetCXReceiptByHash, timer)

	if cx, blockHash, blockNumber, _ := rawdb.ReadCXReceipt(s.hmy.ChainDb(), hash); cx != nil {
		// Format response according to version
		switch s.version {
		case V1, Eth:
			tx, err := v1.NewCxReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return NewStructuredResponse(tx)
		case V2:
			tx, err := v2.NewCxReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return NewStructuredResponse(tx)
		default:
			return nil, ErrUnknownRPCVersion
		}
	}
	utils.Logger().Debug().
		Err(fmt.Errorf("unable to found CX receipt for tx %v", hash.String())).
		Msgf("%v error at %v", LogTag, "GetCXReceiptByHash")
	return nil, nil // Legacy behavior is to not return an error here
}

// ResendCx requests that the egress receipt for the given cross-shard
// transaction be sent to the destination shard for credit.  This is used for
// unblocking a half-complete cross-shard transaction whose fund has been
// withdrawn already from the source shard but not credited yet in the
// destination account due to transient failures.
func (s *PublicTransactionService) ResendCx(ctx context.Context, txID common.Hash) (bool, error) {
	timer := DoMetricRPCRequest(ResendCx)
	defer DoRPCRequestDuration(ResendCx, timer)

	_, success := s.hmy.ResendCx(ctx, txID)

	// Response output is the same for all versions
	return success, nil
}

// returnHashesWithPagination returns result with pagination (offset, page in TxHistoryArgs).
func returnHashesWithPagination(hashes []common.Hash, pageIndex uint32, pageSize uint32) []common.Hash {
	size := defaultPageSize
	if pageSize > 0 {
		size = pageSize
	}
	if uint64(size)*uint64(pageIndex) >= uint64(len(hashes)) {
		return make([]common.Hash, 0)
	}
	if uint64(size)*uint64(pageIndex)+uint64(size) > uint64(len(hashes)) {
		return hashes[size*pageIndex:]
	}
	return hashes[size*pageIndex : size*pageIndex+size]
}

// EstimateGas - estimate gas cost for a given operation
func EstimateGas(ctx context.Context, hmy *hmy.Harmony, args CallArgs, blockNrOrHash rpc.BlockNumberOrHash, gasCap *big.Int) (uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  uint64 = params.TxGas - 1
		hi  uint64
		cap uint64
	)
	// Use zero address if sender unspecified.
	if args.From == nil {
		args.From = new(common.Address)
	}
	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {

		// Retrieve the block to act as the gas ceiling
		blk, err := hmy.BlockByNumberOrHash(ctx, blockNrOrHash)
		if err != nil {
			return 0, err
		}
		hi = blk.GasLimit()
	}
	// Recap the highest gas limit with account's available balance.
	if args.GasPrice != nil && args.GasPrice.ToInt().BitLen() != 0 {
		state, _, err := hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
		if err != nil {
			return 0, err
		}
		balance := state.GetBalance(*args.From) // from can't be nil
		available := new(big.Int).Set(balance)
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available) >= 0 {
				return 0, errors.New("insufficient funds for transfer")
			}
			available.Sub(available, args.Value.ToInt())
		}
		allowance := new(big.Int).Div(available, args.GasPrice.ToInt())

		// If the allowance is larger than maximum uint64, skip checking
		if allowance.IsUint64() && hi > allowance.Uint64() {
			transfer := args.Value
			if transfer == nil {
				transfer = new(hexutil.Big)
			}
			utils.Logger().Warn().Uint64("original", hi).Uint64("balance", balance.Uint64()).Uint64("sent", transfer.ToInt().Uint64()).Uint64("gasprice", args.GasPrice.ToInt().Uint64()).Uint64("fundable", allowance.Uint64()).Msg("Gas estimation capped by limited funds")
			hi = allowance.Uint64()
		}
	}

	// Recap the highest gas allowance with specified gascap.
	if gasCap != nil && hi > gasCap.Uint64() {
		utils.Logger().Warn().Uint64("requested", hi).Uint64("cap", gasCap.Uint64()).Msg("Caller gas above allowance, capping")
		hi = gasCap.Uint64()
	}
	cap = hi

	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (bool, *core.ExecutionResult, error) {
		args.Gas = (*hexutil.Uint64)(&gas)

		result, err := DoEVMCall(ctx, hmy, args, blockNrOrHash, 0)
		if err != nil {
			if errors.Is(err, core.ErrIntrinsicGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return result.Failed(), &result, nil
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		failed, _, err := executable(mid)

		// If the error is not nil(consensus error), it means the provided message
		// call or transaction will never be accepted no matter how much gas it is
		// assigned. Return the error directly, don't struggle any more.
		if err != nil {
			return 0, err
		}
		if failed {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		failed, result, err := executable(hi)
		if err != nil {
			return 0, err
		}
		if failed {
			if result != nil && result.VMErr != vm.ErrOutOfGas {
				if len(result.Revert()) > 0 {
					return 0, newRevertError(result)
				}
				return 0, result.VMErr
			}
			// Otherwise, the specified gas cap is too low
			return 0, fmt.Errorf("gas required exceeds allowance (%d)", cap)
		}
	}
	return hi, nil
}

func newRevertError(result *core.ExecutionResult) *revertError {
	reason, errUnpack := abi.UnpackRevert(result.Revert())
	err := errors.New("execution reverted")
	if errUnpack == nil {
		err = fmt.Errorf("execution reverted: %v", reason)
	}
	return &revertError{
		error:  err,
		reason: hexutil.Encode(result.Revert()),
	}
}

// revertError is an API error that encompassas an EVM revertal with JSON error
// code and a binary data blob.
type revertError struct {
	error
	reason string // revert reason hex encoded
}

// ErrorCode returns the JSON error code for a revertal.
// See: https://github.com/ethereum/wiki/wiki/JSON-RPC-Error-Codes-Improvement-Proposal
func (e *revertError) ErrorCode() int {
	return 3
}

// ErrorData returns the hex encoded revert reason.
func (e *revertError) ErrorData() interface{} {
	return e.reason
}
