package hmyapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/harmony-one/harmony/core/types"
)

// SubmitTransaction is a helper function that submits tx to txPool and logs a message.
func SubmitTransaction(ctx context.Context, b Backend, tx *types.Transaction) (common.Hash, error) {
	fmt.Println("minhSubmitTransaction-- ", tx.To(), tx.ShardID(), tx.Data())
	if err := b.SendTx(ctx, tx); err != nil {
		log.Info("SubmitTransaction error when sending", "fullhash", tx.Hash().Hex())
		return common.Hash{}, err
	}
	if tx.To() == nil {
		signer := types.MakeSigner(b.ChainConfig(), b.CurrentBlock().Number())
		from, err := types.Sender(signer, tx)
		if err != nil {
			return common.Hash{}, err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		log.Info("SubmitTransaction Submitted contract creation", "fullhash", tx.Hash().Hex(), "contract", addr.Hex())
	} else {
		message, _ := tx.AsMessage(types.HomesteadSigner{})
		log.Info("SubmitTransaction transaction", "fullhash", tx.Hash().Hex(), "recipient", tx.To(), "value", tx.Value(), "from", message.From(), "nonce", message.Nonce())
	}
	return tx.Hash(), nil
}
