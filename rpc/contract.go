package rpc

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/hmy"
	hmyCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	defaultGasPrice    = denominations.Nano
	defaultFromAddress = "0x0000000000000000000000000000000000000000"
)

// PublicContractService provides an API to access Harmony's contract services.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicContractService struct {
	hmy     *hmy.Harmony
	version Version
}

// NewPublicContractAPI creates a new API for the RPC interface
func NewPublicContractAPI(hmy *hmy.Harmony, version Version) rpc.API {
	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service:   &PublicContractService{hmy, version},
		Public:    true,
	}
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicContractService) Call(
	ctx context.Context, args CallArgs, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Execute call
	result, err := doCall(ctx, s.hmy, args, blockNum, vm.Config{}, CallTimeout, s.hmy.RPCGasCap)
	if err != nil {
		return nil, err
	}

	// If VM returns error, still return the ReturnData, which is the contract error message
	return result.ReturnData, nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicContractService) GetCode(
	ctx context.Context, addr string, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch state
	address := hmyCommon.ParseAddr(addr)
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)

	// Response output is the same for all versions
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicContractService) GetStorageAt(
	ctx context.Context, addr string, key string, blockNumber BlockNumber,
) (hexutil.Bytes, error) {
	// Process number based on version
	blockNum := blockNumber.EthBlockNumber()

	// Fetch state
	state, _, err := s.hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return nil, err
	}
	address := hmyCommon.ParseAddr(addr)
	res := state.GetState(address, common.HexToHash(key))

	// Response output is the same for all versions
	return res[:], state.Error()
}

// docall executes an EVM call
func doCall(
	ctx context.Context, hmy *hmy.Harmony, args CallArgs, blockNum rpc.BlockNumber,
	vmCfg vm.Config, timeout time.Duration, globalGasCap *big.Int,
) (core.ExecutionResult, error) {
	defer func(start time.Time) {
		utils.Logger().Debug().
			Dur("runtime", time.Since(start)).
			Msg("Executing EVM call finished")
	}(time.Now())

	// Fetch state
	state, header, err := hmy.StateAndHeaderByNumber(ctx, blockNum)
	if state == nil || err != nil {
		return core.ExecutionResult{}, err
	}

	// Set sender address or use a default if none specified
	var addr common.Address
	if args.From == nil {
		// Any address does not affect the logic of this call.
		addr = common.HexToAddress(defaultFromAddress)
	} else {
		addr = *args.From
	}

	// Set default gas & gas price if none were set
	gas := uint64(math.MaxUint64 / 2)
	if args.Gas != nil {
		gas = uint64(*args.Gas)
	}
	if globalGasCap != nil && globalGasCap.Uint64() < gas {
		utils.Logger().Warn().
			Uint64("requested", gas).
			Uint64("cap", globalGasCap.Uint64()).
			Msg("Caller gas above allowance, capping")
		gas = globalGasCap.Uint64()
	}
	gasPrice := new(big.Int).SetUint64(defaultGasPrice)
	if args.GasPrice != nil {
		gasPrice = args.GasPrice.ToInt()
	}

	// Set value & data
	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}
	var data []byte
	if args.Data != nil {
		data = *args.Data
	}

	// Create new call message
	msg := types.NewMessage(addr, args.To, 0, value, gas, gasPrice, data, false)

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}

	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()

	// Get a new instance of the EVM.
	evm, err := hmy.GetEVM(ctx, msg, state, header)
	if err != nil {
		return core.ExecutionResult{}, err
	}

	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(core.GasPool).AddGas(math.MaxUint64)
	result, err := core.ApplyMessage(evm, msg, gp)
	if err != nil {
		return core.ExecutionResult{}, err
	}

	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		return core.ExecutionResult{}, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}

	// Response output is the same for all versions
	return result, nil
}
