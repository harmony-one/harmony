package v2

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/harmony-one/harmony/test/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	oneBig     = big.NewInt(1e18)
	tenOnes    = new(big.Int).Mul(big.NewInt(10), oneBig)
	twelveOnes = new(big.Int).Mul(big.NewInt(12), oneBig)
	gasPrice   = big.NewInt(10000)
)

func TestNewReceipt(t *testing.T) {
	var (
		FaucetPriKey, _                     = crypto.GenerateKey()
		to                                  = common.Address{}
		gasLimit                            = uint64(1e18)
		nonce                        uint64 = 1
		blockHash                           = common.Hash{}
		blockNumber                  uint64 = 1
		blockIndex                   uint64 = 1
		receipt                             = &types.Receipt{}
		r                            any
		err                          error
		createValidatorTxDescription = staking.Description{
			Name:            "SuperHero",
			Identity:        "YouWouldNotKnow",
			Website:         "Secret Website",
			SecurityContact: "LicenseToKill",
			Details:         "blah blah blah",
		}
		payloadMaker = func() (staking.Directive, interface{}) {
			fromKey, _ := crypto.GenerateKey()
			return staking.DirectiveCreateValidator, staking.CreateValidator{
				Description:        createValidatorTxDescription,
				MinSelfDelegation:  tenOnes,
				MaxTotalDelegation: twelveOnes,
				ValidatorAddress:   crypto.PubkeyToAddress(fromKey.PublicKey),
				Amount:             tenOnes,
			}
		}
	)

	t.Run("effectiveGasPrice-transaction", func(t *testing.T) {
		unsigned := types.NewTransaction(nonce, to, 0, big.NewInt(0), 0, big.NewInt(0), nil)
		tx, _ := types.SignTx(unsigned, types.HomesteadSigner{}, FaucetPriKey)
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, receipt)
		require.NoError(t, err)
		require.EqualValues(t, 0, MustReceiptEffectivePrice(r))

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)
		require.EqualValues(t, 1, MustReceiptEffectivePrice(r))
	})
	t.Run("effectiveGasPrice-staking", func(t *testing.T) {
		unsigned, err := helpers.CreateTestStakingTransaction(payloadMaker, nil, 0, gasLimit, gasPrice)
		tx, _ := staking.Sign(unsigned, staking.NewEIP155Signer(unsigned.ChainID()), FaucetPriKey)
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, receipt)
		require.NoError(t, err)
		assert.EqualValues(t, 0, MustReceiptEffectivePrice(r))

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)
		assert.EqualValues(t, 1, MustReceiptEffectivePrice(r))
	})
}
