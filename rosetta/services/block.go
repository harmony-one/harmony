package services

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/harmony-one/harmony/hmy/tracers"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	coreTypes "github.com/harmony-one/harmony/core/types"

	"github.com/coinbase/rosetta-sdk-go/server"
	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"

	"github.com/harmony-one/harmony/core/rawdb"
	hmytypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/rosetta/common"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

const (
	// txTraceCacheSize is max number of transaction traces to keep cached
	txTraceCacheSize = 1e5
)

// BlockAPI implements the server.BlockAPIServicer interface.
type BlockAPI struct {
	hmy          *hmy.Harmony
	txTraceCache *lru.Cache
}

// NewBlockAPI creates a new instance of a BlockAPI.
func NewBlockAPI(hmy *hmy.Harmony) server.BlockAPIServicer {
	traceCache, _ := lru.New(txTraceCacheSize)
	return &BlockAPI{
		hmy:          hmy,
		txTraceCache: traceCache,
	}
}

// BlockMetadata ..
type BlockMetadata struct {
	Epoch *big.Int `json:"epoch"`
}

// Block implements the /block endpoint
func (s *BlockAPI) Block(
	ctx context.Context, request *types.BlockRequest,
) (response *types.BlockResponse, rosettaError *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	var blk *hmytypes.Block
	var currBlockID, prevBlockID *types.BlockIdentifier
	if blk, rosettaError = getBlock(ctx, s.hmy, request.BlockIdentifier); rosettaError != nil {
		return nil, rosettaError
	}

	currBlockID = &types.BlockIdentifier{
		Index: blk.Number().Int64(),
		Hash:  blk.Hash().String(),
	}

	if blk.NumberU64() == 0 {
		prevBlockID = currBlockID
	} else {
		prevBlock, err := s.hmy.BlockByNumber(ctx, rpc.BlockNumber(blk.Number().Int64()-1))
		if err != nil {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
		}
		prevBlockID = &types.BlockIdentifier{
			Index: prevBlock.Number().Int64(),
			Hash:  prevBlock.Hash().String(),
		}
	}

	// Report any side effect transaction now as it can be computed & cached on block fetch.
	transactions := []*types.Transaction{}
	if s.containsSideEffectTransaction(ctx, blk) {
		tx, rosettaError := s.getSideEffectTransaction(ctx, blk)
		if rosettaError != nil {
			return nil, rosettaError
		}
		transactions = append(transactions, tx)
	}

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
		Transactions:          transactions,
		Metadata:              metadata,
	}

	otherTransactions := []*types.TransactionIdentifier{}
	for _, tx := range blk.Transactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	for _, tx := range blk.StakingTransactions() {
		otherTransactions = append(otherTransactions, &types.TransactionIdentifier{
			Hash: tx.Hash().String(),
		})
	}
	// Report cross-shard transaction payouts.
	for _, cxReceipts := range blk.IncomingReceipts() {
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
) (*types.BlockTransactionResponse, *types.Error) {
	if err := assertValidNetworkIdentifier(request.NetworkIdentifier, s.hmy.ShardID); err != nil {
		return nil, err
	}

	blk, rosettaError := getBlock(
		ctx, s.hmy, &types.PartialBlockIdentifier{Hash: &request.BlockIdentifier.Hash},
	)
	if rosettaError != nil {
		return nil, rosettaError
	}
	txHash := ethcommon.HexToHash(request.TransactionIdentifier.Hash)
	txInfo, rosettaError := s.getTransactionInfo(ctx, blk, txHash)
	if rosettaError != nil {
		// If no transaction info is found, check for side effect case transaction.
		response, rosettaError2 := s.sideEffectBlockTransaction(ctx, request)
		if rosettaError2 != nil && rosettaError2.Code != common.TransactionNotFoundError.Code {
			return nil, common.NewError(common.TransactionNotFoundError, map[string]interface{}{
				"from_error": rosettaError2,
			})
		}
		return response, rosettaError2
	}
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, rpc.BlockNumber(blk.NumberU64()))
	if state == nil || err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": fmt.Sprintf("block state not found for block %v", blk.NumberU64()),
		})
	}

	var transaction *types.Transaction
	if txInfo.tx != nil && txInfo.receipt != nil {
		contractInfo := &ContractInfo{}
		if _, ok := txInfo.tx.(*hmytypes.Transaction); ok {
			// check for contract related operations, if it is a plain transaction.
			if txInfo.tx.To() != nil {
				// possible call to existing contract so fetch relevant data
				contractInfo.ContractCode = state.GetCode(*txInfo.tx.To())
				contractInfo.ContractAddress = txInfo.tx.To()
			} else {
				// contract creation, so address is in receipt
				contractInfo.ContractCode = state.GetCode(txInfo.receipt.ContractAddress)
				contractInfo.ContractAddress = &txInfo.receipt.ContractAddress
			}
			contractInfo.ExecutionResult, rosettaError = s.getTransactionTrace(ctx, blk, txInfo)
			if rosettaError != nil {
				return nil, rosettaError
			}
		}
		transaction, rosettaError = FormatTransaction(txInfo.tx, txInfo.receipt, contractInfo, true)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else if txInfo.cxReceipt != nil {
		transaction, rosettaError = FormatCrossShardReceiverTransaction(txInfo.cxReceipt)
		if rosettaError != nil {
			return nil, rosettaError
		}
	} else {
		return nil, &common.TransactionNotFoundError
	}
	return &types.BlockTransactionResponse{Transaction: transaction}, nil
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
	ctx context.Context, blk *hmytypes.Block, txHash ethcommon.Hash,
) (txInfo *transactionInfo, rosettaError *types.Error) {
	// Look for all of the possible transactions
	var index uint64
	var plainTx *hmytypes.Transaction
	var stakingTx *stakingTypes.StakingTransaction
	plainTx, _, _, index = rawdb.ReadTransaction(s.hmy.ChainDb(), txHash)
	if plainTx == nil {
		stakingTx, _, _, index = rawdb.ReadStakingTransaction(s.hmy.ChainDb(), txHash)
		// if there both normal and staking transactions, correct index offset.
		index = index + uint64(blk.Transactions().Len())
	}
	cxReceipt, _, _, _ := rawdb.ReadCXReceipt(s.hmy.ChainDb(), txHash)

	if plainTx == nil && stakingTx == nil && cxReceipt == nil {
		return nil, &common.TransactionNotFoundError
	}

	var receipt *hmytypes.Receipt
	receipts, err := s.hmy.GetReceipts(ctx, blk.Hash())
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if int(index) < len(receipts) {
		receipt = receipts[index]
		if cxReceipt == nil && receipt.TxHash != txHash {
			return nil, common.NewError(common.CatchAllError, map[string]interface{}{
				"message": "unable to find correct receipt for transaction",
			})
		}
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

var (
	// defaultTraceReExec is the number of blocks the tracer can go back and re-execute to produce historical state.
	// Only 1 block is needed to check for internal transactions
	defaultTraceReExec = uint64(1)
	// defaultTraceTimeout is the amount of time a transaction can execute
	defaultTraceTimeout = (10 * time.Second).String()
	// defaultTraceLogConfig is the log config of all traces
	defaultTraceLogConfig = vm.LogConfig{
		DisableMemory:  false,
		DisableStack:   false,
		DisableStorage: false,
		Debug:          false,
		Limit:          0,
	}
)

var ttLock *lru.Cache

func init() {
	ttLock, _ = lru.New(10000)
}

// getTransactionTrace for the given txInfo.
func (s *BlockAPI) getTransactionTrace(
	ctx context.Context, blk *hmytypes.Block, txInfo *transactionInfo,
) ([]*tracers.RosettaLogItem, *types.Error) {
	cacheKey := blk.Hash().String() + txInfo.tx.Hash().String()
	if value, ok := s.txTraceCache.Get(cacheKey); ok {
		return value.([]*tracers.RosettaLogItem), nil
	}

	lock := &sync.Mutex{}
	if ok, _ := ttLock.ContainsOrAdd(blk.Hash().String(), lock); ok {
		if lockTmp, ok := ttLock.Get(blk.Hash().String()); ok {
			lock = lockTmp.(*sync.Mutex)
		} else {
			lock = nil
		}
	}

	if lock != nil {
		lock.Lock()
		defer lock.Unlock()
	}

	if value, ok := s.txTraceCache.Get(cacheKey); ok {
		return value.([]*tracers.RosettaLogItem), nil
	}

	var blockError *types.Error
	var foundResult []*tracers.RosettaLogItem
	var tracer = "RosettaBlockTracer"
	err := s.hmy.ComputeTxEnvEachBlockWithoutApply(blk, defaultTraceReExec, func(txIndex int, tx *coreTypes.Transaction, msg core.Message, vmctx vm.Context, statedb *state.DB) bool {
		execResultInterface, err := s.hmy.TraceTx(ctx, msg, vmctx, statedb, &hmy.TraceConfig{
			Tracer: &tracer,
			LogConfig: &vm.LogConfig{
				DisableMemory:  true,
				DisableStack:   false,
				DisableStorage: true,
				Debug:          false,
				Limit:          0,
			},
			Timeout: &defaultTraceTimeout,
			Reexec:  &defaultTraceReExec,
		})
		if err != nil {
			blockError = common.NewError(common.CatchAllError, map[string]interface{}{
				"message": err.Error(),
			})
			return false
		}

		execResult, ok := execResultInterface.([]*tracers.RosettaLogItem)
		if !ok {
			blockError = common.NewError(common.CatchAllError, map[string]interface{}{
				"message": "unknown tracer exec result type",
			})
			return false
		}

		if txInfo.tx.Hash().String() == tx.Hash().String() {
			foundResult = execResult
		}

		cacheKey := blk.Hash().String() + tx.Hash().String()
		s.txTraceCache.Add(cacheKey, execResult)
		return true
	})
	if err != nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": err.Error(),
		})
	}

	if blockError != nil {
		return nil, blockError
	}

	if foundResult == nil {
		return nil, common.NewError(common.CatchAllError, map[string]interface{}{
			"message": fmt.Errorf("transaction not found for block %#x", blk.Hash()),
		})
	}

	return foundResult, nil
}

// getBlock ..
func getBlock(
	ctx context.Context, hmy *hmy.Harmony, blockID *types.PartialBlockIdentifier,
) (blk *hmytypes.Block, rosettaError *types.Error) {
	var err error
	if blockID.Hash != nil {
		requestBlockHash := ethcommon.HexToHash(*blockID.Hash)
		blk, err = hmy.GetBlock(ctx, requestBlockHash)
	} else if blockID.Index != nil {
		blk, err = hmy.BlockByNumber(ctx, rpc.BlockNumber(*blockID.Index))
	} else {
		return nil, &common.BlockNotFoundError
	}
	if err != nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": err.Error(),
		})
	}
	if blk == nil {
		return nil, common.NewError(common.BlockNotFoundError, map[string]interface{}{
			"message": "block not found for given block identifier",
		})
	}
	return blk, nil
}
