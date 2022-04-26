package vm

import (
	"context"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	"github.com/harmony-one/harmony/contracts/router"
)

// A ContractTransactor for test purposes. SendTransaction just stores
// the data & value for later inspection, other methods are stubs.
type mockContractTxer struct {
	Data  []byte
	Value *big.Int
}

func (ct *mockContractTxer) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	ct.Data = tx.Data()
	ct.Value = tx.Value()
	return nil
}

func (ct *mockContractTxer) PendingCodeAt(ctx context.Context, account common.Address) ([]byte, error) {
	// We need to return some non-empty slice, but a single STOP instruction will do:
	return []byte{0}, nil
}
func (ct *mockContractTxer) PendingNonceAt(ctx context.Context, account common.Address) (uint64, error) {
	return 0, nil
}

func (ct *mockContractTxer) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	// Arbitrary:
	return big.NewInt(7), nil
}

func (ct *mockContractTxer) EstimateGas(ctx context.Context, call ethereum.CallMsg) (gas uint64, err error) {
	// Arbitrary:
	return 4, nil
}

func newTestRouterTxer() (*router.RouterTransactor, *mockContractTxer) {
	mockTxer := &mockContractTxer{}
	routerTxer, err := router.NewRouterTransactor(routerAddress, mockTxer)
	if err != nil {
		panic(err)
	}
	return routerTxer, mockTxer
}

func TestRouter(t *testing.T) {
	rtx, mtx := newTestRouterTxer()

	opts := &bind.TransactOpts{
		Signer: bind.SignerFn(func(_ types.Signer, _ common.Address, tx *types.Transaction) (*types.Transaction, error) {
			return tx, nil
		}),
	}
	_, err := rtx.Send(opts,
		common.BytesToAddress([]byte{0x0b, 0x0b}),
		7,
		[]byte("hello"),
		big.NewInt(404044),
		big.NewInt(342),
		big.NewInt(70),
		common.BytesToAddress([]byte{0xa1, 0x1c, 0xe0}),
	)
	assert.Nil(t, err, "send() transaction failed.")
	m, err := parseRouterMethod(mtx.Data)
	assert.Nil(t, err, "parsing the data should not produce an error")
	assert.Nil(t, m.retrySend)
	assert.NotNil(t, m.send)
}
