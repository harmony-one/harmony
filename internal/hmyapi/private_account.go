package hmyapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/hmy"
	"github.com/harmony-one/harmony/internal/utils"
)

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	am        *accounts.Manager
	nonceLock *AddrLocker
	b         *hmy.APIBackend
}

// NewAccount will create a new account and returns the address for the new account.
func (s *PrivateAccountAPI) NewAccount(password string) (common.Address, error) {
	// TODO: port
	return common.Address{}, nil
}

// SendTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails.
func (s *PrivateAccountAPI) SendTransaction(ctx context.Context, args SendTxArgs, passwd string) (common.Hash, error) {
	if args.Nonce == nil {
		// Hold the addresse's mutex around signing to prevent concurrent assignment of
		// the same nonce to multiple accounts.
		s.nonceLock.LockAddr(args.From)
		defer s.nonceLock.UnlockAddr(args.From)
	}
	signed, err := s.signTransaction(ctx, &args, passwd)
	if err != nil {
		utils.Logger().Warn().
			Str("from", args.From.Hex()).
			Str("to", args.To.Hex()).
			Uint64("value", args.Value.ToInt().Uint64()).
			AnErr("err", err).
			Msg("Failed transaction send attempt")
		return common.Hash{}, err
	}
	return SubmitTransaction(ctx, s.b, signed)
}

// signTransaction sets defaults and signs the given transaction
// NOTE: the caller needs to ensure that the nonceLock is held, if applicable,
// and release it after the transaction has been submitted to the tx pool
func (s *PrivateAccountAPI) signTransaction(ctx context.Context, args *SendTxArgs, passwd string) (*types.Transaction, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}
	wallet, err := s.am.Find(account)
	if err != nil {
		return nil, err
	}
	// Set some sanity defaults and terminate on failure
	if err := args.setDefaults(ctx, s.b); err != nil {
		return nil, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.toTransaction()

	return wallet.SignTxWithPassphrase(account, passwd, tx, s.b.ChainConfig().ChainID)
}
