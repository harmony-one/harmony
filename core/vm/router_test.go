package vm

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"testing"
	"testing/quick"
	"unicode"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	"github.com/harmony-one/harmony/contracts/router"
	"github.com/harmony-one/harmony/core/state"
	harmonyTypes "github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/params"
)

// A ContractTransactor for test purposes, which doesn't actually execute
// transactions. SendTransaction just stores the data & value for a call
// for later inspection, other methods are stubs.
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

var dummyTxOpts = &bind.TransactOpts{
	Signer: bind.SignerFn(func(_ types.Signer, _ common.Address, tx *types.Transaction) (*types.Transaction, error) {
		return tx, nil
	}),
}

func callRouterMethod(rtx *router.RouterTransactor, m routerMethod) error {
	var err error
	switch {
	case m.send != nil:
		args := m.send
		_, err = rtx.Send(dummyTxOpts,
			args.to,
			args.toShard,
			args.payload,
			args.gasBudget,
			args.gasPrice,
			args.gasLimit,
			args.gasLeftoverTo,
		)
		return err
	case m.retrySend != nil:
		args := m.retrySend
		_, err = rtx.RetrySend(dummyTxOpts, args.msgAddr, args.gasLimit, args.gasPrice)
		return err
	default:
		err = fmt.Errorf("routerMethod has no variant set: %v", m)
	}
	return err
}

// Test that the decoding arguments for the supplied generates the right
// result.
func testParseRouterMethod(t *testing.T, m routerMethod) {
	rtx, mtx := newTestRouterTxer()
	err := callRouterMethod(rtx, m)
	assert.Nil(t, err, "transaction failed.")

	t.Log("## Payload ##")
	t.Log("")
	logPayload(t, mtx.Data)

	gotM, err := parseRouterMethod(mtx.Data)
	assert.Nil(t, err, "Parsing the data should not produce an error.")
	assert.Equal(t, m, gotM, "Parsed method is not as expected.")
}

// Dump the payload in a semi-usable hexdump format, via t.Logf.
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

// genSendArgs is an alternate encoding of routerSendArgs, which does the right
// thing when interacting with quick.Check.
type genSendArgs struct {
	To, GasLeftoverTo             common.Address
	ToShard                       uint32
	GasBudget, GasPrice, GasLimit common.Hash
	Payload                       []byte
}

// Like genSendArgs, but for retrySend().
type genRetrySendArgs struct {
	MsgAddr            common.Address
	GasLimit, GasPrice common.Hash
}

// Like genSendArgs, but for types.CXMessage
type genCXMessage struct {
	To, From                      common.Address
	ToShard, FromShard            uint32
	Payload                       []byte
	GasBudget, GasPrice, GasLimit common.Hash
	GasLeftoverTo                 common.Address
	Nonce                         uint64
	Value                         common.Hash
}

func (g genSendArgs) ToRouterMethod() routerMethod {
	return routerMethod{send: &routerSendArgs{
		to:        g.To,
		toShard:   g.ToShard,
		payload:   g.Payload,
		gasBudget: readBig(g.GasBudget[:]),
		gasPrice:  readBig(g.GasPrice[:]),
		gasLimit:  readBig(g.GasLimit[:]),
	}}
}

func (g genRetrySendArgs) ToRouterMethod() routerMethod {
	return routerMethod{retrySend: &routerRetrySendArgs{
		msgAddr:  g.MsgAddr,
		gasLimit: readBig(g.GasLimit[:]),
		gasPrice: readBig(g.GasPrice[:]),
	}}
}

func (g genCXMessage) ToMessage() harmonyTypes.CXMessage {
	return harmonyTypes.CXMessage{
		To: g.To, From: g.From,
		ToShard: g.ToShard, FromShard: g.FromShard,
		Payload:       g.Payload,
		GasBudget:     readBig(g.GasBudget[:]),
		GasPrice:      readBig(g.GasPrice[:]),
		GasLimit:      readBig(g.GasLimit[:]),
		GasLeftoverTo: g.GasLeftoverTo,
		Nonce:         g.Nonce,
		Value:         readBig(g.Value[:]),
	}
}

// Test decoding arguments for hand-picked arguments to send()
func TestParseRouterSendExample(t *testing.T) {
	bobAddr := common.BytesToAddress([]byte{0x0b, 0x0b})
	toShard := uint32(math.MaxUint32)
	payload := []byte("hello, strange world!")
	gasBudget := &big.Int{}
	gasPrice := &big.Int{}
	gasLimit := &big.Int{}
	assert.Nil(t, gasBudget.UnmarshalText([]byte("0x4444444444444444444444444444444444444444444444444444444444444444")))
	assert.Nil(t, gasPrice.UnmarshalText([]byte("0x3333333333333333333333333333333333333333333333333333333333333333")))
	assert.Nil(t, gasLimit.UnmarshalText([]byte("0x2222222222222222222222222222222222222222222222222222222222222222")))
	aliceAddr := common.BytesToAddress([]byte{0xa1, 0x1c, 0xe0})

	testParseRouterMethod(t, routerMethod{send: &routerSendArgs{
		to:            bobAddr,
		toShard:       toShard,
		payload:       payload,
		gasBudget:     gasBudget,
		gasPrice:      gasPrice,
		gasLimit:      gasLimit,
		gasLeftoverTo: aliceAddr,
	}})
}

// Test decoding randomized arguments to send()
func TestParseRouterSendRandom(t *testing.T) {
	err := quick.Check(func(g genSendArgs) bool {
		testParseRouterMethod(t, g.ToRouterMethod())
		return true
	}, nil)
	assert.Nil(t, err)
}

// Test decoding randomized arguments to retrySend()
func TestParseRouterRetrySendRandom(t *testing.T) {
	err := quick.Check(func(g genRetrySendArgs) bool {
		testParseRouterMethod(t, g.ToRouterMethod())
		return true
	}, nil)
	assert.Nil(t, err)
}

func newTestStateDB() (*state.DB, error) {
	return state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()))
}

// Tries storing and then loading random messages, and makes sure the results are correct.
func TestLoadStoreMessage(t *testing.T) {
	err := quick.Check(func(gmsg genCXMessage) bool {
		msg := gmsg.ToMessage()
		db, err := newTestStateDB()
		assert.Nil(t, err, "newTestStateDB() failed.")
		msgAddr, payloadHash := messageAddrAndPayloadHash(msg)
		ms := msgStorage{
			db:   db,
			addr: msgAddr,
		}
		ms.StoreMessage(msg, payloadHash)
		loadedMsg, loadedHash, err := ms.LoadMessage(msg.FromShard)
		assert.Nil(t, err, "Loading message failed")
		assert.Equal(t, loadedHash, payloadHash, "Payload hashes should match")
		assert.Equal(t, loadedMsg, msg, "Messages should match.")
		return true
	}, nil)
	assert.Nil(t, err)
}

func TestCallRouter(t *testing.T) {
	txAddr := common.BytesToAddress([]byte{1, 2, 3})
	rxAddr := common.BytesToAddress([]byte{4, 5, 6})
	leftoverAddr := common.BytesToAddress([]byte{7, 8, 9})

	gasBudget := big.NewInt(40)
	gasPrice := big.NewInt(4)
	gasLimit := big.NewInt(10)
	totalValue := big.NewInt(75)
	value := big.NewInt(0).Sub(totalValue, gasBudget)
	payload := []byte("hello")
	toShard := uint32(4)

	msg := harmonyTypes.CXMessage{
		To:            rxAddr,
		From:          txAddr,
		ToShard:       4,
		FromShard:     0,
		Payload:       []byte("hello"),
		GasBudget:     gasBudget,
		GasPrice:      gasPrice,
		GasLimit:      gasLimit,
		GasLeftoverTo: leftoverAddr,
		Nonce:         0,
		Value:         value,
	}

	db, err := newTestStateDB()
	assert.Nil(t, err, "Creating test DB failed")

	sendMethod := routerMethod{send: &routerSendArgs{
		to:            rxAddr,
		toShard:       toShard,
		payload:       payload,
		gasBudget:     gasBudget,
		gasPrice:      gasPrice,
		gasLimit:      gasLimit,
		gasLeftoverTo: leftoverAddr,
	}}

	ok := t.Run("Initial send", func(t *testing.T) {
		testCallRouter(t, db,
			txAddr,
			sendMethod,
			totalValue,
			7000, // Arbitrary
			[]harmonyTypes.CXMessage{msg},
		)
	})
	assert.True(t, ok, "Initial send failed")

	msgAddr, _ := messageAddrAndPayloadHash(msg)

	additionalValue := big.NewInt(20)
	newGasPrice := big.NewInt(6)
	newMessage := msg
	newMessage.GasPrice = newGasPrice
	newMessage.GasBudget = (&big.Int{}).Add(msg.GasBudget, additionalValue)

	ok = t.Run("Retry", func(t *testing.T) {
		testCallRouter(t, db,
			txAddr,
			routerMethod{retrySend: &routerRetrySendArgs{
				gasLimit: gasLimit,
				gasPrice: newGasPrice,
				msgAddr:  msgAddr,
			}},
			additionalValue,
			7000, // Arbitrary
			[]harmonyTypes.CXMessage{newMessage},
		)
	})

	assert.True(t, ok, "Retry send failed")

	// If we send another message, with the same arguments, it should be the
	// same except for the nonce:
	msg2 := msg
	msg2.Nonce++

	ok = t.Run("Send second message", func(t *testing.T) {
		testCallRouter(t, db,
			txAddr,
			sendMethod,
			totalValue,
			7000, // Arbitrary
			[]harmonyTypes.CXMessage{msg2},
		)
	})
	assert.True(t, ok, "Sending second message failed")
}

func testCallRouter(
	t *testing.T,
	db StateDB,
	callerAddr common.Address,
	m routerMethod,
	value *big.Int,
	gas uint64,
	expectedMsgs []harmonyTypes.CXMessage,
) {
	rtx, mtx := newTestRouterTxer()

	var msgs []harmonyTypes.CXMessage

	evm := NewEVM(
		Context{
			CanTransfer: func(db StateDB, addr common.Address, value *big.Int) bool {
				return true
			},
			EmitCXMessage: func(m harmonyTypes.CXMessage) error {
				msgs = append(msgs, m)
				return nil
			},
			Transfer: func(db StateDB, from common.Address, to common.Address, value *big.Int, txType harmonyTypes.TransactionType) {
				db.SubBalance(from, value)
				db.AddBalance(to, value)
			},
			IsValidator: func(db StateDB, addr common.Address) bool {
				return db.IsValidator(addr)
			},
		},
		db,
		params.TestChainConfig,
		Config{},
	)
	err := callRouterMethod(rtx, m)
	assert.Nil(t, err, "transaction failed.")

	contract := NewContract(
		AccountRef(callerAddr),
		AccountRef(routerAddress),
		value,
		7000000)
	_, err = RunWriteCapablePrecompiledContract(&routerPrecompile{}, evm, contract, mtx.Data, false)
	assert.Nil(t, err, "Error calling router contract.")
	assert.Equal(t, expectedMsgs, msgs)
}
