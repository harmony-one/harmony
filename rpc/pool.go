package rpc

import (
	"context"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	v1 "github.com/harmony-one/harmony/rpc/v1"
	v2 "github.com/harmony-one/harmony/rpc/v2"
	staking "github.com/harmony-one/harmony/staking/types"
)

// PublicPoolService provides an API to access the Harmony node's transaction pool.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicPoolService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicPoolAPI creates a new API for the RPC interface
func NewPublicPoolAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicPoolService{hmy, version},
		Public:    true,
	}
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicPoolService) SendRawTransaction(
	ctx context.Context, encodedTx hexutil.Bytes,
) (common.Hash, error) {
	// DOS prevention
	if len(encodedTx) >= types.MaxEncodedPoolTransactionSize {
		err := errors.Wrapf(core.ErrOversizedData, "encoded tx size: %d", len(encodedTx))
		return common.Hash{}, err
	}

	var tx types.TransactionInterface
	var txType types.TransactionType

	if s.version == Eth {
		txType = types.EthereumTx
		tx = new(types.EthTransaction)
		if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
			return common.Hash{}, err
		}

		// Verify transaction type & chain
		/*c := s.hmy.ChainConfig().ChainID
		if id := tx.ChainID(); id.Cmp(c) != 0 {
			return common.Hash{}, errors.Wrapf(
				ErrInvalidChainID, "blockchain chain id:%s, given %s", c.String(), id.String(),
			)
		}*/

		// Submit transaction
		if err := s.hmy.SendEthTx(ctx, tx.(*types.EthTransaction)); err != nil {
			utils.Logger().Warn().Err(err).Msg("Could not submit transaction")
			return tx.Hash(), err
		}
	} else {
		txType = types.HarmonyTx
		tx = new(types.Transaction)
		if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
			return common.Hash{}, err
		}

		// Verify transaction type & chain
		c := s.hmy.ChainConfig().ChainID
		if id := tx.ChainID(); id.Cmp(c) != 0 {
			return common.Hash{}, errors.Wrapf(
				ErrInvalidChainID, "blockchain chain id:%s, given %s", c.String(), id.String(),
			)
		}

		// Submit transaction
		if err := s.hmy.SendTx(ctx, tx.(*types.Transaction)); err != nil {
			utils.Logger().Warn().Err(err).Msg("Could not submit transaction")
			return tx.Hash(), err
		}
	}

	// Log submission
	if tx.Recipient() == nil {
		signer := types.MakeSigner(s.hmy.ChainConfig(), s.hmy.CurrentBlock().Epoch())
		from, err := types.Sender(signer, tx, txType)
		if err != nil {
			return common.Hash{}, err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		utils.Logger().Info().
			Str("fullhash", tx.Hash().Hex()).
			Str("contract", common2.MustAddressToBech32(addr)).
			Msg("Submitted contract creation")
	} else {
		utils.Logger().Info().
			Str("fullhash", tx.Hash().Hex()).
			Str("recipient", tx.Recipient().Hex()).
			Msg("Submitted transaction")
	}

	// Response output is the same for all versions
	return tx.Hash(), nil
}

// SendRawStakingTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicPoolService) SendRawStakingTransaction(
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
	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
		return nil, err
	}

	// Only format and return plain transactions according to the version
	transactions := []StructuredResponse{}
	for i := range pending {
		if plainTx, ok := pending[i].(*types.Transaction); ok {
			var tx interface{}
			switch s.version {
			case V1, Eth:
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
	// Fetch all pending transactions (stx & plain tx)
	pending, err := s.hmy.GetPoolTransactions()
	if err != nil {
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
			case V1, Eth:
				tx, err = v1.NewStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0)
				if err != nil {
					utils.Logger().Debug().
						Err(err).
						Msgf("%v error at %v", LogTag, "PendingStakingTransactions")
					continue // Legacy behavior is to not return error here
				}
			case V2:
				tx, err = v2.NewStakingTransaction(stakingTx, common.Hash{}, 0, 0, 0)
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
	// For each transaction error in the error sink, format the response (same format for all versions)
	formattedErrors := []StructuredResponse{}
	for _, txError := range s.hmy.GetCurrentTransactionErrorSink() {
		formattedErr, err := NewStructuredResponse(txError)
		if err != nil {
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
	// For each staking tx error in the error sink, format the response (same format for all versions)
	formattedErrors := []StructuredResponse{}
	for _, txErr := range s.hmy.GetCurrentStakingErrorSink() {
		formattedErr, err := NewStructuredResponse(txErr)
		if err != nil {
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
	// For each cx receipt, format the response (same format for all versions)
	formattedReceipts := []StructuredResponse{}
	for _, receipts := range s.hmy.GetPendingCXReceipts() {
		formattedReceipt, err := NewStructuredResponse(receipts)
		if err != nil {
			return nil, err
		}
		formattedReceipts = append(formattedReceipts, formattedReceipt)
	}
	return formattedReceipts, nil
}
