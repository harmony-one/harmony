package vm

import (
	"context"
	"math/big"
	"testing"
	"unicode"

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
	gasBudget := &big.Int{}
	gasPrice := &big.Int{}
	gasLimit := &big.Int{}
	assert.Nil(t, gasBudget.UnmarshalText([]byte("0x4444444444444444444444444444444444444444444444444444444444444444")))
	assert.Nil(t, gasPrice.UnmarshalText([]byte("0x3333333333333333333333333333333333333333333333333333333333333333")))
	assert.Nil(t, gasLimit.UnmarshalText([]byte("0x2222222222222222222222222222222222222222222222222222222222222222")))

	_, err := rtx.Send(opts,
		common.BytesToAddress([]byte{0x0b, 0x0b}),
		(1<<32)-1,
		[]byte("hello, strange world!"),
		gasBudget, gasPrice, gasLimit,
		common.BytesToAddress([]byte{0xa1, 0x1c, 0xe0}),
	)
	assert.Nil(t, err, "send() transaction failed.")

	t.Log("## Payload ##")
	t.Log("")
	logPayload(t, mtx.Data)

	m, err := parseRouterMethod(mtx.Data)
	assert.Nil(t, err, "parsing the data should not produce an error")
	assert.Nil(t, m.retrySend)
	assert.NotNil(t, m.send)
}

func logPayload(t *testing.T, data []byte) {
	i := 0
	for len(data) > 0 {
		var prefix []byte
		if len(data) >= 32 {
			prefix = data[:32]
			data = data[32:]
		} else {
			prefix = data
			data = nil
		}
		buf := []byte{}
		for _, b := range prefix {
			if b < 128 && unicode.IsPrint(rune(b)) {
				buf = append(buf, b)
			} else {
				buf = append(buf, '.')
			}
		}
		t.Logf("%2d: %0x %s", i, prefix, buf)
		i++
	}
}
