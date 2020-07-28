package rpc

import (
	"context"
	"fmt"
	"github.com/harmony-one/harmony/internal/utils"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	internal_common "github.com/harmony-one/harmony/internal/common"
	rpc_common "github.com/harmony-one/harmony/rpc/common"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// defaultPageSize is to have default pagination.
const (
	defaultPageSize = uint32(1000)
)

// PublicTransactionPoolService exposes methods for the RPC interface
type PublicTransactionPoolService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicTransactionPoolAPI creates a new API for the RPC interface
func NewPublicTransactionPoolAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicTransactionPoolService{hmy, version},
		Public:    true,
	}
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

// GetTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionPoolService) GetTransactionsHistory(
	ctx context.Context, args rpc_common.TxHistoryArgs,
) (rpc_common.StructuredResponse, error) {
	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return nil, err
		}
	}
	hashes, err := s.hmy.GetTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		return nil, err
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return rpc_common.StructuredResponse{"transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []rpc_common.StructuredResponse{}
	for _, hash := range result {
		tx, err := s.GetTransactionByHash(ctx, hash)
		if err == nil {
			// Legacy behavior is to not return RPC errors
			txs = append(txs, tx)
		} else {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetTransactionsHistory")
		}
	}
	return rpc_common.StructuredResponse{"transactions": txs}, nil
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionPoolService) GetBlockTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
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
	case V1:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionPoolService) GetBlockTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
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
	case V1:
		return hexutil.Uint(len(block.Transactions())), nil
	case V2:
		return len(block.Transactions()), nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolService) GetTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (rpc_common.StructuredResponse, error) {
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
		tx, err := v1.NewRPCTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolService) GetTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (rpc_common.StructuredResponse, error) {
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
		tx, err := v1.NewRPCTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetTransactionByHash returns the plain transaction for the given hash
func (s *PublicTransactionPoolService) GetTransactionByHash(
	ctx context.Context, hash common.Hash,
) (rpc_common.StructuredResponse, error) {
	// Try to return an already finalized transaction
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(s.hmy.ChainDb(), hash)
	if tx == nil {
		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	// Format the response according to the version
	switch s.version {
	case V1:
		tx, err := v1.NewRPCTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCTransaction(tx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetStakingTransactionsHistory returns the list of transactions hashes that involve a particular address.
func (s *PublicTransactionPoolService) GetStakingTransactionsHistory(
	ctx context.Context, args rpc_common.TxHistoryArgs,
) (rpc_common.StructuredResponse, error) {
	// Fetch transaction history
	var address string
	var result []common.Hash
	var err error
	if strings.HasPrefix(args.Address, "one1") {
		address = args.Address
	} else {
		addr := internal_common.ParseAddr(args.Address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			utils.Logger().Debug().
				Err(err).
				Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
			// Legacy behavior is to not return RPC errors
			return nil, nil
		}
	}
	hashes, err := s.hmy.GetStakingTransactionsHistory(address, args.TxType, args.Order)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionsHistory")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	result = returnHashesWithPagination(hashes, args.PageIndex, args.PageSize)

	// Just hashes have same response format for all versions
	if !args.FullTx {
		return rpc_common.StructuredResponse{"staking_transactions": result}, nil
	}

	// Full transactions have different response format
	txs := []rpc_common.StructuredResponse{}
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
	return rpc_common.StructuredResponse{"staking_transactions": txs}, nil
}

// GetBlockStakingTransactionCountByNumber returns the number of staking transactions in the block with the given block number.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionPoolService) GetBlockStakingTransactionCountByNumber(
	ctx context.Context, blockNumber BlockNumber,
) (interface{}, error) {
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
	case V1:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetBlockStakingTransactionCountByHash returns the number of staking transactions in the block with the given hash.
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionPoolService) GetBlockStakingTransactionCountByHash(
	ctx context.Context, blockHash common.Hash,
) (interface{}, error) {
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
	case V1:
		return hexutil.Uint(len(block.StakingTransactions())), nil
	case V2:
		return len(block.StakingTransactions()), nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetStakingTransactionByBlockNumberAndIndex returns the staking transaction for the given block number and index.
func (s *PublicTransactionPoolService) GetStakingTransactionByBlockNumberAndIndex(
	ctx context.Context, blockNumber BlockNumber, index TransactionIndex,
) (rpc_common.StructuredResponse, error) {
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
		tx, err := v1.NewRPCStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetStakingTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolService) GetStakingTransactionByBlockHashAndIndex(
	ctx context.Context, blockHash common.Hash, index TransactionIndex,
) (rpc_common.StructuredResponse, error) {
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
	case V1:
		tx, err := v1.NewRPCStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCStakingTransactionFromBlockIndex(block, uint64(index))
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetStakingTransactionByHash returns the staking transaction for the given hash
func (s *PublicTransactionPoolService) GetStakingTransactionByHash(
	ctx context.Context, hash common.Hash,
) (rpc_common.StructuredResponse, error) {
	// Try to return an already finalized transaction
	stx, blockHash, blockNumber, index := rawdb.ReadStakingTransaction(s.hmy.ChainDb(), hash)
	if stx == nil {
		utils.Logger().Debug().
			Err(errors.Wrapf(ErrTransactionNotFound, "hash %v", hash.String())).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}
	block, err := s.hmy.GetBlock(ctx, blockHash)
	if err != nil {
		utils.Logger().Debug().
			Err(err).
			Msgf("%v error at %v", LogTag, "GetStakingTransactionByHash")
		// Legacy behavior is to not return RPC errors
		return nil, nil
	}

	switch s.version {
	case V1:
		tx, err := v1.NewRPCStakingTransaction(stx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	case V2:
		tx, err := v2.NewRPCStakingTransaction(stx, blockHash, blockNumber, block.Time().Uint64(), index)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(tx)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number.
// Legacy for apiv1. For apiv2, please use getAccountNonce/getPoolNonce/getTransactionsCount/getStakingTransactionsCount apis for
// more granular transaction counts queries
// Note that the return type is an interface to account for the different versions
func (s *PublicTransactionPoolService) GetTransactionCount(
	ctx context.Context, addr string, blockNumber BlockNumber,
) (response interface{}, err error) {
	// Process arguments based on version
	blockNum := blockNumber.EthBlockNumber()
	address := internal_common.ParseAddr(addr)

	// Fetch transaction count
	var nonce uint64
	if blockNum == rpc.PendingBlockNumber {
		// Ask transaction pool for the nonce which includes pending transactions
		nonce, err = s.hmy.GetPoolNonce(ctx, address)
		if err != nil {
			return nil, err
		}
	} else {
		// Resolve block number and use its state to ask for the nonce
		state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNum)
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
	case V1:
		return (hexutil.Uint64)(nonce), nil
	case V2:
		return nonce, nil
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetTransactionsCount returns the number of regular transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionPoolService) GetTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr := internal_common.ParseAddr(address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.hmy.GetTransactionsCount(address, txType)
}

// GetStakingTransactionsCount returns the number of staking transactions from genesis of input type ("SENT", "RECEIVED", "ALL")
func (s *PublicTransactionPoolService) GetStakingTransactionsCount(
	ctx context.Context, address, txType string,
) (count uint64, err error) {
	if !strings.HasPrefix(address, "one1") {
		// Handle hex address
		addr := internal_common.ParseAddr(address)
		address, err = internal_common.AddressToBech32(addr)
		if err != nil {
			return 0, err
		}
	}

	// Response output is the same for all versions
	return s.hmy.GetStakingTransactionsCount(address, txType)
}

// SendRawStakingTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolService) SendRawStakingTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	// DOS prevention
	if len(encodedTx) >= types.MaxEncodedPoolTransactionSize {
		err := errors.Wrapf(core.ErrOversizedData, "encoded tx size: %d", len(encodedTx))
		return common.Hash{}, err
	}

	// Verify staking transaction type & chain
	tx := new(staking.StakingTransaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	c := s.hmy.ChainConfig().ChainID
	if id := tx.ChainID(); id.Cmp(c) != 0 {
		return common.Hash{}, errors.Wrapf(
			ErrInvalidChainID, "blockchain chain id:%s, given %s", c.String(), id.String(),
		)
	}

	// Response output is the same for all versions
	return SubmitStakingTransaction(ctx, s.hmy, tx)
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolService) SendRawTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	// DOS prevention
	if len(encodedTx) >= types.MaxEncodedPoolTransactionSize {
		err := errors.Wrapf(core.ErrOversizedData, "encoded tx size: %d", len(encodedTx))
		return common.Hash{}, err
	}

	// Verify transaction type & chain
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	c := s.hmy.ChainConfig().ChainID
	if id := tx.ChainID(); id.Cmp(c) != 0 {
		return common.Hash{}, errors.Wrapf(
			ErrInvalidChainID, "blockchain chain id:%s, given %s", c.String(), id.String(),
		)
	}

	// Response output is the same for all versions
	return SubmitTransaction(ctx, s.hmy, tx)
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolService) GetTransactionReceipt(
	ctx context.Context, hash common.Hash,
) (rpc_common.StructuredResponse, error) {
	// Fetch receipt for plain & staking transaction
	var tx types.PoolTransaction
	var blockHash common.Hash
	var blockNumber, index uint64
	tx, blockHash, blockNumber, index = rawdb.ReadTransaction(s.hmy.ChainDb(), hash)
	if tx == nil {
		tx, blockHash, blockNumber, index = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), hash)
		if tx == nil {
			// Legacy behavior is to not return error if transaction is not found
			utils.Logger().Debug().
				Err(fmt.Errorf("unable to find plain tx or staking tx with hash %v", hash.String())).
				Msgf("%v error at %v", LogTag, "GetTransactionReceipt")
			return nil, nil
		}
	}
	receipts, err := s.hmy.GetReceipts(ctx, blockHash)
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, fmt.Errorf("index of transaction greater than number of receipts")
	}
	receipt := receipts[index]

	// Format response according to version
	switch s.version {
	case V1:
		RpcReceipt, err := v1.NewRPCReceipt(tx, blockHash, blockNumber, index, receipt)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(RpcReceipt)
	case V2:
		RpcReceipt, err := v2.NewRPCReceipt(tx, blockHash, blockNumber, index, receipt)
		if err != nil {
			return nil, err
		}
		return rpc_common.NewStructuredResponse(RpcReceipt)
	default:
		return nil, ErrUnknownRpcVersion
	}
}

// GetPoolStats returns stats for the tx-pool
func (s *PublicTransactionPoolService) GetPoolStats(
	ctx context.Context,
) (rpc_common.StructuredResponse, error) {
	pendingCount, queuedCount := s.hmy.GetPoolStats()

	// Response output is the same for all versions
	return rpc_common.StructuredResponse{
		"executable-count":     pendingCount,
		"non-executable-count": queuedCount,
	}, nil
}

// PendingTransactions returns the plain transactions that are in the transaction pool
func (s *PublicTransactionPoolService) PendingTransactions(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
		return nil, err
	}

	// Only format and return plain transactions according to the version
	transactions := []rpc_common.StructuredResponse{}
	for i := range pending {
		if plainTx, ok := pending[i].(*types.Transaction); ok {
			var tx interface{}
			switch s.version {
			case V1:
				tx, err = v1.NewRPCTransaction(plainTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case V2:
				tx, err = v2.NewRPCTransaction(plainTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
			default:
				return nil, ErrUnknownRpcVersion
			}
			rpcTx, err := rpc_common.NewStructuredResponse(tx)
			if err == nil {
				transactions = append(transactions, rpcTx) // Legacy behavior is to not return error here
			} else {
				utils.Logger().Debug().
					Err(err).
					Msgf("%v error at %v", LogTag, "PendingTransactions")
			}
		} else if _, ok := pending[i].(*staking.StakingTransaction); ok {
			continue // Do not return staking transactions here.
		} else {
			return nil, types.ErrUnknownPoolTxType
		}
	}
	return transactions, nil
}

// PendingStakingTransactions returns the staking transactions that are in the transaction pool
func (s *PublicTransactionPoolService) PendingStakingTransactions(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
		return nil, err
	}

	// Only format and return staking transactions according to the version
	transactions := []rpc_common.StructuredResponse{}
	for i := range pending {
		if _, ok := pending[i].(*types.Transaction); ok {
			continue // Do not return plain transactions here
		} else if stakingTx, ok := pending[i].(*staking.StakingTransaction); ok {
			var tx interface{}
			switch s.version {
			case V1:
				tx, err = v1.NewRPCStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case V2:
				tx, err = v2.NewRPCStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
					continue // Legacy behavior is to not return error here
				}
			default:
				return nil, ErrUnknownRpcVersion
			}
			rpcTx, err := rpc_common.NewStructuredResponse(tx)
			if err == nil {
				transactions = append(transactions, rpcTx) // Legacy behavior is to not return error here
			} else {
				utils.Logger().Debug().
					Err(err).
					Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
			}
		} else {
			return nil, types.ErrUnknownPoolTxType
		}
	}
	return transactions, nil
}

// GetCurrentTransactionErrorSink ..
func (s *PublicTransactionPoolService) GetCurrentTransactionErrorSink(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// For each transaction error in the error sink, format the response (same format for all versions)
	formattedErrors := []rpc_common.StructuredResponse{}
	for _, err := range s.hmy.GetCurrentTransactionErrorSink() {
		formattedErr, err := rpc_common.NewStructuredResponse(err)
		if err != nil {
			return nil, err
		}
		formattedErrors = append(formattedErrors, formattedErr)
	}
	return formattedErrors, nil
}

// GetCurrentStakingErrorSink ..
func (s *PublicTransactionPoolService) GetCurrentStakingErrorSink(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// For each staking tx error in the error sink, format the response (same format for all versions)
	formattedErrors := []rpc_common.StructuredResponse{}
	for _, err := range s.hmy.GetCurrentStakingErrorSink() {
		formattedErr, err := rpc_common.NewStructuredResponse(err)
		if err != nil {
			return nil, err
		}
		formattedErrors = append(formattedErrors, formattedErr)
	}
	return formattedErrors, nil
}

// GetCXReceiptByHash returns the transaction for the given hash
func (s *PublicTransactionPoolService) GetCXReceiptByHash(
	ctx context.Context, hash common.Hash,
) (rpc_common.StructuredResponse, error) {
	if cx, blockHash, blockNumber, _ := rawdb.ReadCXReceipt(s.hmy.ChainDb(), hash); cx != nil {
		// Format response according to version
		switch s.version {
		case V1:
			tx, err := v1.NewRPCCXReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return rpc_common.NewStructuredResponse(tx)
		case V2:
			tx, err := v2.NewRPCCXReceipt(cx, blockHash, blockNumber)
			if err != nil {
				return nil, err
			}
			return rpc_common.NewStructuredResponse(tx)
		default:
			return nil, ErrUnknownRpcVersion
		}
	}
	utils.Logger().Debug().
		Err(fmt.Errorf("unable to found CX receipt for tx %v", hash.String())).
		Msgf("%v error at %v", LogTag, "GetCXReceiptByHash")
	return nil, nil // Legacy behavior is to not return an error here
}

// GetPendingCXReceipts ..
func (s *PublicTransactionPoolService) GetPendingCXReceipts(
	ctx context.Context,
) ([]rpc_common.StructuredResponse, error) {
	// For each cx receipt, format the response (same format for all versions)
	formattedReceipts := []rpc_common.StructuredResponse{}
	for _, receipts := range s.hmy.GetPendingCXReceipts() {
		formattedReceipt, err := rpc_common.NewStructuredResponse(receipts)
		if err != nil {
			return nil, err
		}
		formattedReceipts = append(formattedReceipts, formattedReceipt)
	}
	return formattedReceipts, nil
}
