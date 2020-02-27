package apiv1

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

// defaultPageSize is to have default pagination.
const (
	defaultPageSize = uint32(100)
)

// ReturnPagination returns pagination params.
func ReturnPagination(length int, args TxHistoryArgs) (int, TxHistoryArgs) {
	pageSize := defaultPageSize
	pageIndex := args.PageIndex
	if args.PageSize > 0 {
		pageSize = args.PageSize
	}
	if uint64(pageSize)*uint64(pageIndex) >= uint64(length) {
		args.PageSize = 0
		return 0, args
	}
	if uint64(pageSize)*uint64(pageIndex)+uint64(pageSize) > uint64(length) {
		return length, args
	}
	return int(pageSize*pageIndex + pageSize), args
}

// SubmitTransaction is a helper function that submits tx to txPool and logs a message.
func SubmitTransaction(
	ctx context.Context, b Backend, tx *types.Transaction,
) (common.Hash, error) {
	if err := b.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}
	if tx.To() == nil {
		signer := types.MakeSigner(b.ChainConfig(), b.CurrentBlock().Epoch())
		from, err := types.Sender(signer, tx)
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
			Str("recipient", tx.To().Hex()).
			Msg("Submitted transaction")
	}
	return tx.Hash(), nil
}

// SubmitStakingTransaction is a helper function that submits tx to txPool and logs a message.
func SubmitStakingTransaction(
	ctx context.Context, b Backend, tx *staking.StakingTransaction,
) (common.Hash, error) {
	if err := b.SendStakingTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}
	utils.Logger().Info().Str("fullhash", tx.Hash().Hex()).Msg("Submitted Staking transaction")
	return tx.Hash(), nil
}
