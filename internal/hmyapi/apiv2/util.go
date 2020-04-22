package apiv2

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	common2 "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

// defaultPageSize is to have default pagination.
const (
	defaultPageSize = uint32(100)
)

// ReturnWithPagination returns result with pagination (offset, page in TxHistoryArgs).
func ReturnWithPagination(hashes []common.Hash, pageIndex uint32, pageSize uint32) []common.Hash {
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

// SubmitTransaction is a helper function that submits tx to txPool and logs a message.
func SubmitTransaction(
	ctx context.Context, b Backend, tx *types.Transaction,
) (common.Hash, error) {
	if err := b.SendTx(ctx, tx); err != nil {
		return common.Hash{}, err
	}
	to, err := tx.To()
	if err != nil {
		return common.Hash{}, err
	}
	if tx.Type() == types.Contract {
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
			Str("recipient", to.Hex()).
			Msg("Submitted transaction")
	}
	return tx.Hash(), nil
}
