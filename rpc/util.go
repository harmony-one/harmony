package rpc

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	staking "github.com/harmony-one/harmony/staking/types"
)

// SubmitTransaction is a helper function that submits tx to txPool and logs a message.
func SubmitTransaction(
	ctx context.Context, hmy *hmy.Harmony, tx *types.Transaction,
) (common.Hash, error) {
	if err := hmy.SendTx(ctx, tx); err != nil {
		utils.Logger().Warn().Err(err).Msg("Could not submit transaction")
		return tx.Hash(), err
	}
	if tx.To() == nil {
		signer := types.MakeSigner(hmy.ChainConfig(), hmy.CurrentBlock().Epoch())
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
	ctx context.Context, hmy *hmy.Harmony, tx *staking.StakingTransaction,
) (common.Hash, error) {
	if err := hmy.SendStakingTx(ctx, tx); err != nil {
		utils.Logger().Warn().Err(err).Msg("Could not submit staking transaction")
		return tx.Hash(), err
	}
	utils.Logger().Info().
		Str("fullhash", tx.Hash().Hex()).
		Msg("Submitted Staking transaction")
	return tx.Hash(), nil
}
