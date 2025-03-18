package v1

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
	staking "github.com/harmony-one/harmony/staking/types"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
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
		nonce                        uint64 = 1
		gasLimit                            = uint64(1e18)
		blockHash                           = common.Hash{}
		blockNumber                  uint64 = 1
		blockIndex                   uint64 = 1
		receipt                             = &types.Receipt{}
		r                            any
		err                          error
		createValidatorTxDescription = stakingTypes.Description{
			Name:            "SuperHero",
			Identity:        "YouWouldNotKnow",
			Website:         "Secret Website",
			SecurityContact: "LicenseToKill",
			Details:         "blah blah blah",
		}
		payloadMaker = func() (stakingTypes.Directive, interface{}) {
			fromKey, _ := crypto.GenerateKey()
			return stakingTypes.DirectiveCreateValidator, stakingTypes.CreateValidator{
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

		price := MustReceiptEffectivePrice(r)
		require.EqualValues(t, fmt.Sprintf("0x%x", types.DefaultEffectiveGasPrice), price.String())

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)

		effectivePrice := MustReceiptEffectivePrice(r)
		assert.EqualValues(t, "0x1", effectivePrice.String())
	})
	t.Run("effectiveGasPrice-staking", func(t *testing.T) {
		unsigned, err := helpers.CreateTestStakingTransaction(payloadMaker, nil, 0, gasLimit, gasPrice)
		tx, _ := staking.Sign(unsigned, stakingTypes.NewEIP155Signer(unsigned.ChainID()), FaucetPriKey)
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, receipt)
		require.NoError(t, err)

		receiptEffectivePrice := MustReceiptEffectivePrice(r)
		assert.EqualValues(t, fmt.Sprintf("0x%x", types.DefaultEffectiveGasPrice), receiptEffectivePrice.String())

		rec := &types.Receipt{
			EffectiveGasPrice: big.NewInt(1),
		}
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)

		mustReceiptEffectivePrice := MustReceiptEffectivePrice(r)
		assert.EqualValues(t, "0x1", mustReceiptEffectivePrice.String())
	})
	t.Run("contract-address", func(t *testing.T) {
		unsigned := types.NewTransaction(nonce, to, 0, big.NewInt(0), 0, big.NewInt(0), nil)
		tx, _ := types.SignTx(unsigned, types.HomesteadSigner{}, FaucetPriKey)
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, receipt)
		require.NoError(t, err)
		require.EqualValues(t, receipt.ContractAddress, MustContractAddress(r))

		rec := &types.Receipt{
			ContractAddress: common.BytesToAddress([]byte{0x1}),
		}
		r, err = NewReceipt(tx, blockHash, blockNumber, blockIndex, rec)
		require.NoError(t, err)
		require.EqualValues(t, rec.ContractAddress, MustContractAddress(r))
	})
	t.Run("unknown-receipt", func(t *testing.T) {
		_, err := NewReceipt(nil, blockHash, blockNumber, blockIndex, receipt)
		require.Error(t, err)
	})
}
