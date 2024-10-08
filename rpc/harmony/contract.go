package rpc

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/time/rate"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/eth/rpc"
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
	// TEMP SOLUTION to rpc node spamming issue
	limiterCall    *rate.Limiter
	evmCallTimeout time.Duration
}

// NewPublicContractAPI creates a new API for the RPC interface
func NewPublicContractAPI(
	hmy *hmy.Harmony,
	version Version,
	limiterEnable bool,
	limit int,
	evmCallTimeout time.Duration,
) rpc.API {
	var limiter *rate.Limiter
	if limiterEnable {
		limiter = rate.NewLimiter(rate.Limit(limit), limit)
	}

	return rpc.API{
		Namespace: version.Namespace(),
		Version:   APIVersion,
		Service: &PublicContractService{
			hmy:            hmy,
			version:        version,
			limiterCall:    limiter,
			evmCallTimeout: evmCallTimeout,
		},
		Public: true,
	}
}

func (s *PublicContractService) wait(limiter *rate.Limiter, ctx context.Context) error {
	if limiter != nil {
		deadlineCtx, cancel := context.WithTimeout(ctx, DefaultRateLimiterWaitTimeout)
		defer cancel()
		if !limiter.Allow() {
			name := reflect.TypeOf(limiter).Elem().Name()
			rpcRateLimitCounterVec.With(prometheus.Labels{
				"limiter_name": name,
			}).Inc()
		}

		return limiter.Wait(deadlineCtx)
	}
	return nil
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicContractService) Call(
	ctx context.Context, args CallArgs, blockNrOrHash rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	timer := DoMetricRPCRequest(Call)
	defer DoRPCRequestDuration(Call, timer)

	err := s.wait(s.limiterCall, ctx)
	if err != nil {
		DoMetricRPCQueryInfo(Call, RateLimitedNumber)
		return nil, err
	}

	// Execute call
	result, err := DoEVMCall(ctx, s.hmy, args, blockNrOrHash, s.evmCallTimeout)
	if err != nil {
		return nil, err
	}

	// If the result contains a revert reason, try to unpack and return it.
	if len(result.Revert()) > 0 {
		return nil, newRevertError(&result)
	}
	// If VM returns error, still return the ReturnData, which is the contract error message
	return result.ReturnData, nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicContractService) GetCode(
	ctx context.Context, addr string, blockNrOrHash rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	timer := DoMetricRPCRequest(GetCode)
	defer DoRPCRequestDuration(GetCode, timer)

	// Fetch state
	address, err := hmyCommon.ParseAddr(addr)
	if err != nil {
		DoMetricRPCQueryInfo(GetCode, FailedNumber)
		return nil, err
	}
	state, _, err := s.hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		DoMetricRPCQueryInfo(GetCode, FailedNumber)
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
	ctx context.Context, addr string, key string, blockNrOrHash rpc.BlockNumberOrHash,
) (hexutil.Bytes, error) {
	timer := DoMetricRPCRequest(GetStorageAt)
	defer DoRPCRequestDuration(GetStorageAt, timer)

	// Fetch state
	state, _, err := s.hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		DoMetricRPCQueryInfo(GetStorageAt, FailedNumber)
		return nil, err
	}
	address, err := hmyCommon.ParseAddr(addr)
	if err != nil {
		DoMetricRPCQueryInfo(GetStorageAt, FailedNumber)
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))

	// Response output is the same for all versions
	return res[:], state.Error()
}

// DoEVMCall executes an EVM call
func DoEVMCall(
	ctx context.Context, hmy *hmy.Harmony, args CallArgs, blockNrOrHash rpc.BlockNumberOrHash,
	timeout time.Duration,
) (core.ExecutionResult, error) {
	defer func(start time.Time) {
		utils.Logger().Debug().
			Dur("runtime", time.Since(start)).
			Msg("Executing EVM call finished")
	}(time.Now())

	// Fetch state
	state, header, err := hmy.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if state == nil || err != nil {
		DoMetricRPCQueryInfo(DoEvmCall, FailedNumber)
		return core.ExecutionResult{}, err
	}

	// Create new call message
	msg := args.ToMessage(hmy.RPCGasCap)

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
		DoMetricRPCQueryInfo(DoEvmCall, FailedNumber)
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
		DoMetricRPCQueryInfo(DoEvmCall, FailedNumber)
		return core.ExecutionResult{}, err
	}

	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		DoMetricRPCQueryInfo(DoEvmCall, FailedNumber)
		return core.ExecutionResult{}, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}

	// Response output is the same for all versions
	return result, nil
}
