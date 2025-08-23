package hmy

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/harmony-one/harmony/block"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/vm"
	chain2 "github.com/harmony-one/harmony/internal/chain"
	"github.com/harmony-one/harmony/internal/params"
)

// dummy version of vm.PrecompiledContract
type dummyPrecompile struct{}

func (d dummyPrecompile) IsWrite() bool {
	return false
}

var _ vm.WriteCapablePrecompiledContract = dummyPrecompile{}

func (d dummyPrecompile) Run(input []byte) ([]byte, error) { return nil, nil }

// func (d dummyPrecompile) RequiredGas(input []byte) uint64  { return 0 }
func (d dummyPrecompile) RequiredGas(evm *vm.EVM, contract *vm.Contract, input []byte) (uint64, error) {
	return 0, nil
}

func (d dummyPrecompile) RunWriteCapable(evm *vm.EVM, contract *vm.Contract, input []byte) ([]byte, error) {
	return nil, nil
}

func TestStateOverrides(t *testing.T) {
	key, _ := crypto.GenerateKey()

	addr1 := common.HexToAddress("0x1111")
	addr2 := common.HexToAddress("0x2222") // precompile
	addr3 := common.HexToAddress("0x3333") // MovePreCompileTo

	dummyPrecompiles := map[common.Address]vm.WriteCapablePrecompiledContract{
		addr2: dummyPrecompile{},
	}

	reset := func(precompiles *map[common.Address]vm.WriteCapablePrecompiledContract) {
		*precompiles = map[common.Address]vm.WriteCapablePrecompiledContract{
			addr2: dummyPrecompile{},
		}
	}

	simpleState := map[common.Hash]common.Hash{
		common.HexToHash("0x01"): common.HexToHash("0xAA"),
	}
	simpleStateDiff := map[common.Hash]common.Hash{
		common.HexToHash("0x02"): common.HexToHash("0xBB"),
	}

	balance := (*hexutil.Big)(big.NewInt(10000))
	nonce := hexutil.Uint64(0)
	code := hexutil.Bytes([]byte("code"))

	tests := []struct {
		name          string
		expectedError error
		overrides     StateOverrides
		precompiles   map[common.Address]vm.WriteCapablePrecompiledContract
	}{
		{
			name:          "ValidOverride",
			expectedError: nil,
			overrides: StateOverrides{
				addr1: OverrideAccount{
					Nonce:   &nonce,
					Code:    &code,
					Balance: &balance,
					State:   &simpleState,
				},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "MoveToPrecompile",
			expectedError: nil,
			overrides: StateOverrides{
				addr2: OverrideAccount{
					MovePrecompileTo: &addr3,
				},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "MoveNonPrecompileError",
			expectedError: fmt.Errorf("account %s is not a precompile", addr1.Hex()),
			overrides: StateOverrides{
				addr1: OverrideAccount{
					MovePrecompileTo: &addr3,
				},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "MoveToOverriddenError",
			expectedError: fmt.Errorf("account %s has already been overridden", addr3.Hex()),
			overrides: StateOverrides{
				addr2: OverrideAccount{
					MovePrecompileTo: &addr3,
				},
				addr3: OverrideAccount{},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "StateOverride",
			expectedError: nil,
			overrides: StateOverrides{
				addr1: OverrideAccount{
					State: &simpleState,
				},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "StateDiffOverride",
			expectedError: nil,
			overrides: StateOverrides{
				addr1: OverrideAccount{
					StateDiff: &simpleStateDiff,
				},
			},
			precompiles: dummyPrecompiles,
		},
		{
			name:          "StateAndDiffConflict",
			expectedError: fmt.Errorf("account %s has both 'state' and 'stateDiff'", addr1.Hex()),
			overrides: StateOverrides{
				addr1: OverrideAccount{
					State:     &simpleState,
					StateDiff: &simpleStateDiff,
				},
			},
			precompiles: dummyPrecompiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset(&tt.precompiles)
			_, db, _, _ := getTestEnvironment(*key)
			err := tt.overrides.Apply(db, tt.precompiles)
			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedError)
				}
				if err.Error() != tt.expectedError.Error() {
					t.Fatalf("expected error %q, got %q", tt.expectedError.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %q", err.Error())
				}
			}
		})
	}
}

func TestBlockOverrides(t *testing.T) {
	numberVal := (*hexutil.Big)(big.NewInt(12345))
	timeVal := hexutil.Uint64(1617181920)
	gasLimitVal := hexutil.Uint64(8000000)
	feeRecipient := common.HexToAddress("0xABCDEF")

	blockOverrides := BlockOverrides{
		Number:       numberVal,
		Time:         &timeVal,
		GasLimit:     &gasLimitVal,
		FeeRecipient: &feeRecipient,
		// Difficulty, PrevRandao, BaseFeePerGas, and BlobBaseFee are no-op
	}

	ctx := &vm.BlockContext{
		BlockNumber: big.NewInt(0),
		Time:        big.NewInt(0),
		GasLimit:    0,
		Coinbase:    common.Address{},
	}

	blockOverrides.Apply(ctx)

	if ctx.BlockNumber.Cmp(numberVal.ToInt()) != 0 {
		t.Errorf("BlockNumber: expected %s, got %s", numberVal.ToInt().String(), ctx.BlockNumber.String())
	}
	if ctx.Time.Cmp(big.NewInt(int64(timeVal))) != 0 {
		t.Errorf("Time: expected %d, got %s", timeVal, ctx.Time.String())
	}
	if ctx.GasLimit != uint64(gasLimitVal) {
		t.Errorf("GasLimit: expected %d, got %d", gasLimitVal, ctx.GasLimit)
	}
	if ctx.Coinbase != feeRecipient {
		t.Errorf("FeeRecipient: expected %s, got %s", feeRecipient.Hex(), ctx.Coinbase.Hex())
	}
}

func getTestEnvironment(testBankKey ecdsa.PrivateKey) (*core.BlockChainImpl, *state.DB, *block.Header, ethdb.Database) {
	// initialize
	var (
		testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
		testBankFunds   = new(big.Int).Mul(big.NewInt(denominations.One), big.NewInt(40000))
		chainConfig     = params.TestChainConfig
		blockFactory    = blockfactory.ForTest
		database        = rawdb.NewMemoryDatabase()
		gspec           = core.Genesis{
			Config:  chainConfig,
			Factory: blockFactory,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 0,
		}
		engine = chain2.NewEngine()
	)
	genesis := gspec.MustCommit(database)

	// fake blockchain
	cacheConfig := &core.CacheConfig{SnapshotLimit: 0}
	chain, _ := core.NewBlockChain(database, nil, nil, cacheConfig, gspec.Config, engine, vm.Config{})
	db, _ := chain.StateAt(genesis.Root())

	// make a fake block header (use epoch 1 so that locked tokens can be tested)
	header := blockFactory.NewHeader(common.Big0)

	return chain, db, header, database
}
