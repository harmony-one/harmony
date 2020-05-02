package types

import (
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	maxPlainSinkCapacity   = 1024
	maxStakingSinkCapacity = 1024
	logTag                 = "[TransactionErrorSink]"
)

// TransactionError ..
type TransactionError struct {
	TxHashID             string `json:"tx-hash-id"`
	StakingDirective     string `json:"directive-kind,omitempty"`
	TimestampOfRejection int64  `json:"time-at-rejection"`
	ErrMessage           string `json:"error-message"`
}

// TransactionErrors
type TransactionErrors []*TransactionError

// TransactionErrorSink is where all failed transactions get reported.
// Note that the keys of the lru caches are tx-hash strings.
type TransactionErrorSink struct {
	failedPlainTxs   *lru.Cache
	failedStakingTxs *lru.Cache
}

// NewTransactionErrorSink ..
func NewTransactionErrorSink() (*TransactionErrorSink, error) {
	failedPlainTx, err := lru.New(maxPlainSinkCapacity)
	if err != nil {
		return nil, err
	}
	failedStakingTx, err := lru.New(maxStakingSinkCapacity)
	if err != nil {
		return nil, err
	}
	return &TransactionErrorSink{
		failedPlainTxs:   failedPlainTx,
		failedStakingTxs: failedStakingTx,
	}, nil
}

// Add a transaction to the error sink with the given error
func (sink *TransactionErrorSink) Add(tx PoolTransaction, err error) {
	// no-op if no error is provided
	if err == nil {
		return
	}
	if plainTx, ok := tx.(*Transaction); ok {
		hash := plainTx.Hash().String()
		sink.failedPlainTxs.Add(hash, &TransactionError{
			TxHashID:             hash,
			TimestampOfRejection: time.Now().Unix(),
			ErrMessage:           err.Error(),
		})
		utils.Logger().Debug().
			Str("tag", logTag).
			Interface("tx-hash-id", hash).
			Msgf("Added plain transaction error message")
	} else if stakingTx, ok := tx.(*staking.StakingTransaction); ok {
		hash := stakingTx.Hash().String()
		sink.failedStakingTxs.Add(hash, &TransactionError{
			TxHashID:             hash,
			StakingDirective:     stakingTx.StakingType().String(),
			TimestampOfRejection: time.Now().Unix(),
			ErrMessage:           err.Error(),
		})
		utils.Logger().Debug().
			Str("tag", logTag).
			Interface("tx-hash-id", hash).
			Msgf("Added staking transaction error message")
	} else {
		utils.Logger().Error().
			Str("tag", logTag).
			Interface("tx", tx).
			Msg("Attempted to add an unknown transaction type")
	}
}

// Contains checks if there is an error associated with the given hash
// Note that the keys of the lru caches are tx-hash strings.
func (sink *TransactionErrorSink) Contains(hash string) bool {
	return sink.failedPlainTxs.Contains(hash) || sink.failedStakingTxs.Contains(hash)
}

// Remove a transaction's error from the error sink
func (sink *TransactionErrorSink) Remove(tx PoolTransaction) {
	if plainTx, ok := tx.(*Transaction); ok {
		hash := plainTx.Hash().String()
		present := sink.failedPlainTxs.Remove(hash)
		utils.Logger().Debug().
			Str("tag", logTag).
			Interface("tx-hash-id", hash).
			Bool("was-in-error-sink", present).
			Msgf("Removed plain transaction error message")
	} else if stakingTx, ok := tx.(*staking.StakingTransaction); ok {
		hash := stakingTx.Hash().String()
		present := sink.failedStakingTxs.Remove(hash)
		utils.Logger().Debug().
			Str("tag", logTag).
			Interface("tx-hash-id", hash).
			Bool("was-in-error-sink", present).
			Msgf("Removed staking transaction error message")
	} else {
		utils.Logger().Error().
			Str("tag", logTag).
			Interface("tx", tx).
			Msg("Attempted to remove an unknown transaction type")
	}
}

// PlainReport ..
func (sink *TransactionErrorSink) PlainReport() TransactionErrors {
	return reportErrorsFromLruCache(sink.failedPlainTxs)
}

// StakingReport ..
func (sink *TransactionErrorSink) StakingReport() TransactionErrors {
	return reportErrorsFromLruCache(sink.failedStakingTxs)
}

// PlainCount ..
func (sink *TransactionErrorSink) PlainCount() int {
	return sink.failedPlainTxs.Len()
}

// StakingCount ..
func (sink *TransactionErrorSink) StakingCount() int {
	return sink.failedStakingTxs.Len()
}

// reportErrorsFromLruCache is a helper for reporting errors
// from the TransactionErrorSink's lru cache. Do not use this function directly,
// use the respective public methods of TransactionErrorSink.
func reportErrorsFromLruCache(lruCache *lru.Cache) TransactionErrors {
	rpcErrors := TransactionErrors{}
	for _, txHash := range lruCache.Keys() {
		rpcErrorFetch, ok := lruCache.Get(txHash)
		if !ok {
			utils.Logger().Warn().
				Str("tag", logTag).
				Interface("tx-hash-id", txHash).
				Msgf("Error not found in staking sink")
			continue
		}
		rpcError, ok := rpcErrorFetch.(*TransactionError)
		if !ok {
			utils.Logger().Error().
				Str("tag", logTag).
				Interface("tx-hash-id", txHash).
				Msgf("Invalid type in staking sink")
			continue
		}
		rpcErrors = append(rpcErrors, rpcError)
	}
	return rpcErrors
}
