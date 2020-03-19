package apiv2

import (
	"context"
	"fmt"

	"github.com/harmony-one/harmony/common/denominations"

	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
)

const (
	defaultGasPrice    = denominations.Nano
	defaultFromAddress = "0x0000000000000000000000000000000000000000"
)

// PublicBlockChainAPI provides an API to access the Harmony blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new Harmony blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// BlockArgs is struct to include optional block formatting params.
type BlockArgs struct {
	WithSigners bool     `json:"withSigners"`
	InclTx      bool     `json:"inclTx"`
	FullTx      bool     `json:"fullTx"`
	Signers     []string `json:"signers"`
	InclStaking bool     `json:"inclStaking"`
}

// GetBlockByNumber returns the requested block. When fullTx in blockArgs is true all transactions in the block are returned in full detail,
// otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true it shows block signers for this block in list of one addresses.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr uint64, blockArgs BlockArgs) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(blockNr))
	blockArgs.InclTx = true
	if block != nil {
		response, err := RPCMarshalBlock(block, blockArgs)
		if err == nil && rpc.BlockNumber(blockNr) == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// GetBlockByHash returns the requested block. When fullTx in blockArgs is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, blockArgs BlockArgs) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	blockArgs.InclTx = true
	if block != nil {
		return RPCMarshalBlock(block, blockArgs)
	}
	return nil, err
}

// GetCommittee returns committee for a particular epoch.
func (s *PublicBlockChainAPI) GetCommittee(ctx context.Context, epoch int64) (map[string]interface{}, error) {
	committee, err := s.b.GetCommittee(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}
	validators := make([]map[string]interface{}, 0)
	for _, validator := range committee.NodeList {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		validatorBalance, err := s.GetBalance(ctx, oneAddress)
		if err != nil {
			return nil, err
		}
		validatorsFields := map[string]interface{}{
			"address": oneAddress,
			"balance": (*hexutil.Big)(validatorBalance),
		}
		validators = append(validators, validatorsFields)
	}
	result := map[string]interface{}{
		"shardID":    committee.ShardID,
		"validators": validators,
	}
	return result, nil
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockChainAPI) GetShardingStructure(ctx context.Context) ([]map[string]interface{}, error) {
	// Get header and number of shards.
	header := s.b.CurrentBlock().Header()
	numShard := core.ShardingSchedule.InstanceForEpoch(header.Epoch()).NumShards()

	// Return shareding structure for each case.
	return core.ShardingSchedule.GetShardingStructure(int(numShard), int(s.b.GetShardID())), nil
}

// GetShardID returns shard ID of the requested node.
func (s *PublicBlockChainAPI) GetShardID(ctx context.Context) (int, error) {
	return int(s.b.GetShardID()), nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, addr string, blockNr uint64) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.b.StateAndHeaderByNumber(ctx, rpc.BlockNumber(blockNr))
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, addr string, key string, blockNr uint64) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.b.StateAndHeaderByNumber(ctx, rpc.BlockNumber(blockNr))
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

// GetBalanceByBlockNumber returns balance by block number.
func (s *PublicBlockChainAPI) GetBalanceByBlockNumber(ctx context.Context, address string, blockNr int64) (*big.Int, error) {
	addr := internal_common.ParseAddr(address)
	return s.b.GetBalance(ctx, addr, rpc.BlockNumber(blockNr))
}

// GetBalance returns the amount of Nano for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	return s.GetBalanceByBlockNumber(ctx, address, -1)
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockChainAPI) BlockNumber() uint64 {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return header.Number().Uint64()
}

// ResendCx requests that the egress receipt for the given cross-shard
// transaction be sent to the destination shard for credit.  This is used for
// unblocking a half-complete cross-shard transaction whose fund has been
// withdrawn already from the source shard but not credited yet in the
// destination account due to transient failures.
func (s *PublicBlockChainAPI) ResendCx(ctx context.Context, txID common.Hash) (bool, error) {
	_, success := s.b.ResendCx(ctx, txID)
	return success, nil
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is useful to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, _, err := doCall(ctx, s.b, args, blockNr, vm.Config{}, 5*time.Second, s.b.RPCGasCap())
	return (hexutil.Bytes)(result), err
}

func doCall(ctx context.Context, b Backend, args CallArgs, blockNr rpc.BlockNumber, vmCfg vm.Config, timeout time.Duration, globalGasCap *big.Int) ([]byte, uint64, bool, error) {
	defer func(start time.Time) {
		utils.Logger().Debug().
			Dur("runtime", time.Since(start)).
			Msg("Executing EVM call finished")
	}(time.Now())

	state, header, err := b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, 0, false, err
	}
	// Set sender address or use a default if none specified
	var addr common.Address
	if args.From == nil {
		// TODO(ricl): this logic was borrowed from [go-ethereum](https://github.com/ethereum/go-ethereum/blob/f578d41ee6b3087f8021fd561a0b5665aea3dba6/internal/ethapi/api.go#L738)
		// [question](https://ethereum.stackexchange.com/questions/72979/why-does-the-docall-function-use-the-first-account-by-default)
		// Might need to reconsider the logic
		// if wallets := b.AccountManager().Wallets(); len(wallets) > 0 {
		// 	if accounts := wallets[0].Accounts(); len(accounts) > 0 {
		// 		addr = accounts[0].Address
		// 	}
		// }
		// The logic in ethereum is to pick a random address managed under the account manager.
		// Currently Harmony no longers support the account manager.
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

	value := new(big.Int)
	if args.Value != nil {
		value = args.Value.ToInt()
	}

	var data []byte
	if args.Data != nil {
		data = []byte(*args.Data)
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
	evm, vmError, err := b.GetEVM(ctx, msg, state, header)
	if err != nil {
		return nil, 0, false, err
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
	res, gas, failed, err := core.ApplyMessage(evm, msg, gp)
	if err := vmError(); err != nil {
		return nil, 0, false, err
	}
	// If the timer caused an abort, return an appropriate error message
	if evm.Cancelled() {
		return nil, 0, false, fmt.Errorf("execution aborted (timeout = %v)", timeout)
	}
	return res, gas, failed, err
}

// LatestHeader returns the latest header information
func (s *PublicBlockChainAPI) LatestHeader(ctx context.Context) *HeaderInformation {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return newHeaderInformation(header)
}

// doEstimateGas ..
func doEstimateGas(ctx context.Context, b Backend, args CallArgs, gasCap *big.Int) (hexutil.Uint64, error) {
	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo  uint64 = params.TxGas - 1
		hi  uint64
		cap uint64
	)
	blockNum := rpc.LatestBlockNumber
	if args.Gas != nil && uint64(*args.Gas) >= params.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Retrieve the block to act as the gas ceiling
		block, err := b.BlockByNumber(ctx, blockNum)
		if err != nil {
			return 0, err
		}
		hi = block.GasLimit()
	}
	if gasCap != nil && hi > gasCap.Uint64() {
		// log.Warn("Caller gas above allowance, capping", "requested", hi, "cap", gasCap)
		hi = gasCap.Uint64()
	}
	cap = hi

	// Use zero-address if none other is available
	if args.From == nil {
		args.From = &common.Address{}
	}
	// Create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) bool {
		args.Gas = (*hexutil.Uint64)(&gas)

		_, _, failed, err := doCall(ctx, b, args, blockNum, vm.Config{}, 0, big.NewInt(int64(cap)))
		if err != nil || failed {
			return false
		}
		return true
	}
	// Execute the binary search and hone in on an executable gas limit
	for lo+1 < hi {
		mid := (hi + lo) / 2
		if !executable(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	// Reject the transaction as invalid if it still fails at the highest allowance
	if hi == cap {
		if !executable(hi) {
			return 0, fmt.Errorf("gas required exceeds allowance (%d) or always failing transaction", cap)
		}
	}
	return hexutil.Uint64(hi), nil
}

// EstimateGas returns an estimate of the amount of gas needed to execute the
// given transaction against the current pending block.
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args CallArgs) (hexutil.Uint64, error) {
	return doEstimateGas(ctx, s.b, args, nil)
}
