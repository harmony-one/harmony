package rpc

import (
	"context"
	"reflect"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/eth/rpc"
	"github.com/harmony-one/harmony/hmy"
	common2 "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	eth "github.com/harmony-one/harmony/rpc/eth"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
	staking "github.com/harmony-one/harmony/staking/types"
)

// PublicPoolService provides an API to access the Harmony node's transaction pool.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicPoolService struct {
	hmy     *hmy.Harmony
	version Version

	// TEMP SOLUTION to rpc node spamming issue
	limiterPendingTransactions *rate.Limiter
}

// NewPublicPoolAPI creates a new API for the RPC interface
func NewPublicPoolAPI(hmy *hmy.Harmony, version Version, limiterEnable bool, limit int) rpc.API {
	var limiter *rate.Limiter
	if limiterEnable {
		limiter = rate.NewLimiter(rate.Limit(limit), limit)
	}
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicPoolService{hmy, version, limiter},
		Public:    true,
	}
}

func (s *PublicPoolService) wait(limiter *rate.Limiter, ctx context.Context) error {
	if limiter != nil {
		deadlineCtx, cancel := context.WithTimeout(ctx, DefaultRateLimiterWaitTimeout)
		defer cancel()
		if !limiter.Allow() {
			name := reflect.TypeOf(limiter).Elem().Name()
			rpcRateLimitCounterVec.With(prometheus.Labels{
				"limiter_name": name,
			}).Inc()
		}

		return limiter.Wait(deadlineCtx)
	}
	return nil
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicPoolService) SendRawTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	timer := DoMetricRPCRequest(SendRawTransaction)
	defer DoRPCRequestDuration(SendRawTransaction, timer)

	// DOS prevention
	if len(encodedTx) >= types.MaxEncodedPoolTransactionSize {
		err := errors.Wrapf(core.ErrOversizedData, "encoded tx size: %d", len(encodedTx))
		return common.Hash{}, err
	}

	var tx *types.Transaction
	var txHash common.Hash

	if s.version == Eth {
		ethTx := new(types.EthTransaction)
		if err := rlp.DecodeBytes(encodedTx, ethTx); err != nil {
			return common.Hash{}, err
		}
		txHash = ethTx.Hash()
		tx = ethTx.ConvertToHmy()
	} else {
		tx = new(types.Transaction)
		if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
			return common.Hash{}, err
		}
		txHash = tx.Hash()
	}

	// Verify chainID
	if err := s.verifyChainID(tx); err != nil {
		return common.Hash{}, err
	}

	// Submit transaction
	if err := s.hmy.SendTx(ctx, tx); err != nil {
		utils.Logger().Warn().Err(err).Msg("Could not submit transaction")
		return common.Hash{}, err
	}

	// Log submission
	if tx.To() == nil {
		signer := types.MakeSigner(s.hmy.ChainConfig(), s.hmy.CurrentBlock().Epoch())
		ethSigner := types.NewEIP155Signer(s.hmy.ChainConfig().EthCompatibleChainID)

		if tx.IsEthCompatible() {
			signer = ethSigner
		}
		from, err := types.Sender(signer, tx)
		if err != nil {
			return common.Hash{}, err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		utils.Logger().Info().
			Str("fullhash", tx.Hash().Hex()).
			Str("hashByType", tx.HashByType().Hex()).
			Str("contract", common2.MustAddressToBech32(addr)).
			Msg("Submitted contract creation")
	} else {
		utils.Logger().Info().
			Str("fullhash", tx.Hash().Hex()).
			Str("hashByType", tx.HashByType().Hex()).
			Str("recipient", tx.To().Hex()).
			Interface("tx", tx).
			Msg("Submitted transaction")
	}

	// Response output is the same for all versions
	return txHash, nil
}

func (s *PublicPoolService) verifyChainID(tx *types.Transaction) error {
	nodeChainID := s.hmy.ChainConfig().ChainID
	ethChainID := nodeconfig.GetDefaultConfig().GetNetworkType().ChainConfig().EthCompatibleChainID

	if tx.ChainID().Cmp(ethChainID) != 0 && tx.ChainID().Cmp(nodeChainID) != 0 {
		return errors.Wrapf(
			ErrInvalidChainID, "blockchain chain id:%s, given %s", nodeChainID.String(), tx.ChainID().String(),
		)
	}

	return nil
}

// SendRawStakingTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicPoolService) SendRawStakingTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	timer := DoMetricRPCRequest(SendRawStakingTransaction)
	defer DoRPCRequestDuration(SendRawStakingTransaction, timer)

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

	// Submit transaction
	if err := s.hmy.SendStakingTx(ctx, tx); err != nil {
		utils.Logger().Warn().Err(err).Msg("Could not submit staking transaction")
		return tx.Hash(), err
	}

	// Log submission
	utils.Logger().Info().
		Str("fullhash", tx.Hash().Hex()).
		Msg("Submitted Staking transaction")

	// Response output is the same for all versions
	return tx.Hash(), nil
}

// GetPoolStats returns stats for the tx-pool
func (s *PublicPoolService) GetPoolStats(
	ctx context.Context,
) (StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetPoolStats)
	defer DoRPCRequestDuration(GetPoolStats, timer)

	pendingCount, queuedCount := s.hmy.GetPoolStats()

	// Response output is the same for all versions
	return StructuredResponse{
		"executable-count":     pendingCount,
		"non-executable-count": queuedCount,
	}, nil
}

// PendingTransactions returns the plain transactions that are in the transaction pool
func (s *PublicPoolService) PendingTransactions(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(PendingTransactions)
	defer DoRPCRequestDuration(PendingTransactions, timer)

	err := s.wait(s.limiterPendingTransactions, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(PendingTransactions, RateLimitedNumber)
		return nil, err
	}

	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
		DoMetricRPCQueryInfo(PendingTransactions, FailedNumber)
		return nil, err
	}

	// Only format and return plain transactions according to the version
	transactions := []StructuredResponse{}
	for i := range pending {
		if plainTx, ok := pending[i].(*types.Transaction); ok {
			var tx interface{}
			switch s.version {
			case V1:
				tx, err = v1.NewTransaction(plainTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case V2:
				tx, err = v2.NewTransaction(plainTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case Eth:
				from, err := plainTx.SenderAddress()
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
				tx, err = eth.NewTransaction(from, plainTx.ConvertToEth(), common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingTransactions")
					continue // Legacy behavior is to not return error here
				}
			default:
				return nil, ErrUnknownRPCVersion
			}
			rpcTx, err := NewStructuredResponse(tx)
			if err == nil {
				transactions = append(transactions, rpcTx)
			} else {
				// Legacy behavior is to not return error here
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
func (s *PublicPoolService) PendingStakingTransactions(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(PendingStakingTransactions)
	defer DoRPCRequestDuration(PendingStakingTransactions, timer)

	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
		DoMetricRPCQueryInfo(PendingStakingTransactions, FailedNumber)
		return nil, err
	}

	// Only format and return staking transactions according to the version
	transactions := []StructuredResponse{}
	for i := range pending {
		if _, ok := pending[i].(*types.Transaction); ok {
			continue // Do not return plain transactions here
		} else if stakingTx, ok := pending[i].(*staking.StakingTransaction); ok {
			var tx interface{}
			switch s.version {
			case V1:
				tx, err = v1.NewStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case V2:
				tx, err = v2.NewStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0, true)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
					continue // Legacy behavior is to not return error here
				}
			default:
				return nil, ErrUnknownRPCVersion
			}
			rpcTx, err := NewStructuredResponse(tx)
			if err == nil {
				transactions = append(transactions, rpcTx)
			} else {
				// Legacy behavior is to not return error here
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
func (s *PublicPoolService) GetCurrentTransactionErrorSink(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetCurrentTransactionErrorSink)
	defer DoRPCRequestDuration(GetCurrentTransactionErrorSink, timer)

	// For each transaction error in the error sink, format the response (same format for all versions)
	formattedErrors := []StructuredResponse{}
	for _, txError := range s.hmy.GetCurrentTransactionErrorSink() {
		formattedErr, err := NewStructuredResponse(txError)
		if err != nil {
			DoMetricRPCQueryInfo(GetCurrentTransactionErrorSink, FailedNumber)
			return nil, err
		}
		formattedErrors = append(formattedErrors, formattedErr)
	}
	return formattedErrors, nil
}

// GetCurrentStakingErrorSink ..
func (s *PublicPoolService) GetCurrentStakingErrorSink(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetCurrentStakingErrorSink)
	defer DoRPCRequestDuration(GetCurrentStakingErrorSink, timer)

	// For each staking tx error in the error sink, format the response (same format for all versions)
	formattedErrors := []StructuredResponse{}
	for _, txErr := range s.hmy.GetCurrentStakingErrorSink() {
		formattedErr, err := NewStructuredResponse(txErr)
		if err != nil {
			DoMetricRPCQueryInfo(GetCurrentStakingErrorSink, FailedNumber)
			return nil, err
		}
		formattedErrors = append(formattedErrors, formattedErr)
	}
	return formattedErrors, nil
}

// GetPendingCXReceipts ..
func (s *PublicPoolService) GetPendingCXReceipts(
	ctx context.Context,
) ([]StructuredResponse, error) {
	timer := DoMetricRPCRequest(GetPendingCXReceipts)
	defer DoRPCRequestDuration(GetPendingCXReceipts, timer)

	// For each cx receipt, format the response (same format for all versions)
	formattedReceipts := []StructuredResponse{}
	for _, receipts := range s.hmy.GetPendingCXReceipts() {
		formattedReceipt, err := NewStructuredResponse(receipts)
		if err != nil {
			DoMetricRPCQueryInfo(GetPendingCXReceipts, FailedNumber)
			return nil, err
		}
		formattedReceipts = append(formattedReceipts, formattedReceipt)
	}
	return formattedReceipts, nil
}

// GetNumPendingCXReceipts ..
func (s *PublicPoolService) GetNumPendingCXReceipts(
	ctx context.Context,
) (int, error) {
	timer := DoMetricRPCRequest(GetPendingCXReceipts)
	defer DoRPCRequestDuration(GetPendingCXReceipts, timer)

	// For each cx receipt, format the response (same format for all versions)
	formattedReceipts := []StructuredResponse{}
	for _, receipts := range s.hmy.GetPendingCXReceipts() {
		formattedReceipt, err := NewStructuredResponse(receipts)
		if err != nil {
			DoMetricRPCQueryInfo(GetPendingCXReceipts, FailedNumber)
			return 0, err
		}
		formattedReceipts = append(formattedReceipts, formattedReceipt)
	}
	return len(formattedReceipts), nil
}
