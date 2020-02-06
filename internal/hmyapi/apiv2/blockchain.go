package apiv2

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	internal_bls "github.com/harmony-one/harmony/crypto/bls"
	internal_common "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/network"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	defaultGasPrice     = denominations.Nano
	defaultFromAddress  = "0x0000000000000000000000000000000000000000"
	defaultBlocksPeriod = 15000
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
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, blockNr)
			if err != nil {
				return nil, err
			}
		}
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
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, block.NumberU64())
			if err != nil {
				return nil, err
			}
		}
		return RPCMarshalBlock(block, blockArgs)
	}
	return nil, err
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockChainAPI) GetBlocks(ctx context.Context, blockStart, blockEnd uint64, blockArgs BlockArgs) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)
	for i := blockStart; i <= blockEnd; i++ {
		block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(i))
		blockArgs.InclTx = true
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, i)
			if err != nil {
				return nil, err
			}
		}
		if block != nil {
			rpcBlock, err := RPCMarshalBlock(block, blockArgs)
			if err == nil && rpc.BlockNumber(i) == rpc.PendingBlockNumber {
				// Pending blocks need to nil out a few fields
				for _, field := range []string{"hash", "nonce", "miner"} {
					rpcBlock[field] = nil
				}
			}
			result = append(result, rpcBlock)
		}
	}
	return result, nil
}

// GetValidators returns validators list for a particular epoch.
func (s *PublicBlockChainAPI) GetValidators(ctx context.Context, epoch int64) (map[string]interface{}, error) {
	committee, err := s.b.GetValidators(big.NewInt(epoch))
	if err != nil {
		return nil, err
	}
	validators := make([]map[string]interface{}, 0)
	for _, validator := range committee.Slots {
		validatorBalance, err := s.b.GetBalance(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return nil, err
		}
		validatorsFields := map[string]interface{}{
			"address": oneAddress,
			"balance": validatorBalance,
		}
		validators = append(validators, validatorsFields)
	}
	result := map[string]interface{}{
		"shardID":    committee.ShardID,
		"validators": validators,
	}
	return result, nil
}

// GetBlockSigners returns signers for a particular block.
func (s *PublicBlockChainAPI) GetBlockSigners(ctx context.Context, blockNr uint64) ([]string, error) {
	if blockNr == 0 || blockNr >= uint64(s.BlockNumber()) {
		return make([]string, 0), nil
	}
	block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(blockNr))
	if err != nil {
		return nil, err
	}
	blockWithSigners, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(blockNr+1))
	if err != nil {
		return nil, err
	}
	committee, err := s.b.GetValidators(block.Epoch())
	if err != nil {
		return nil, err
	}
	pubkeys := make([]*bls.PublicKey, len(committee.Slots))
	for i, validator := range committee.Slots {
		pubkeys[i] = new(bls.PublicKey)
		validator.BlsPublicKey.ToLibBLSPublicKey(pubkeys[i])
	}
	result := make([]string, 0)
	mask, err := internal_bls.NewMask(pubkeys, nil)
	if err != nil {
		return result, err
	}
	if err != nil {
		return result, err
	}
	err = mask.SetMask(blockWithSigners.Header().LastCommitBitmap())
	if err != nil {
		return result, err
	}
	for _, validator := range committee.Slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return result, err
		}
		blsPublicKey := new(bls.PublicKey)
		validator.BlsPublicKey.ToLibBLSPublicKey(blsPublicKey)
		if ok, err := mask.KeyEnabled(blsPublicKey); err == nil && ok {
			result = append(result, oneAddress)
		}
	}
	return result, nil
}

// IsBlockSigner returns true if validator with address signed blockNr block.
func (s *PublicBlockChainAPI) IsBlockSigner(ctx context.Context, blockNr uint64, address string) (bool, error) {
	if blockNr == 0 || blockNr >= uint64(s.BlockNumber()) {
		return false, nil
	}
	block, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(blockNr))
	if err != nil {
		return false, err
	}
	blockWithSigners, err := s.b.BlockByNumber(ctx, rpc.BlockNumber(blockNr+1))
	if err != nil {
		return false, err
	}
	committee, err := s.b.GetValidators(block.Epoch())
	if err != nil {
		return false, err
	}
	pubkeys := make([]*bls.PublicKey, len(committee.Slots))
	for i, validator := range committee.Slots {
		pubkeys[i] = new(bls.PublicKey)
		validator.BlsPublicKey.ToLibBLSPublicKey(pubkeys[i])
	}
	mask, err := internal_bls.NewMask(pubkeys, nil)
	if err != nil {
		return false, err
	}
	err = mask.SetMask(blockWithSigners.Header().LastCommitBitmap())
	if err != nil {
		return false, err
	}
	for _, validator := range committee.Slots {
		oneAddress, err := internal_common.AddressToBech32(validator.EcdsaAddress)
		if err != nil {
			return false, err
		}
		if oneAddress != address {
			continue
		}
		blsPublicKey := new(bls.PublicKey)
		validator.BlsPublicKey.ToLibBLSPublicKey(blsPublicKey)
		if ok, err := mask.KeyEnabled(blsPublicKey); err == nil && ok {
			return true, nil
		}
	}
	return false, nil
}

// GetSignedBlocks returns how many blocks a particular validator signed for last defaultBlocksPeriod (3 hours ~ 1500 blocks).
func (s *PublicBlockChainAPI) GetSignedBlocks(ctx context.Context, address string) uint64 {
	totalSigned := uint64(0)
	lastBlock := uint64(0)
	blockHeight := uint64(s.BlockNumber())
	if blockHeight >= defaultBlocksPeriod {
		lastBlock = blockHeight - defaultBlocksPeriod + 1
	}
	for i := lastBlock; i <= blockHeight; i++ {
		signed, err := s.IsBlockSigner(ctx, i, address)
		if err == nil && signed {
			totalSigned++
		}
	}
	return totalSigned
}

// GetEpoch returns current epoch.
func (s *PublicBlockChainAPI) GetEpoch(ctx context.Context) uint64 {
	return s.LatestHeader(ctx).Epoch
}

// GetLeader returns current shard leader.
func (s *PublicBlockChainAPI) GetLeader(ctx context.Context) string {
	return s.LatestHeader(ctx).Leader
}

// GetValidatorSelfDelegation returns validator stake.
func (s *PublicBlockChainAPI) GetValidatorSelfDelegation(ctx context.Context, address string) *big.Int {
	return s.b.GetValidatorSelfDelegation(internal_common.ParseAddr(address))
}

// GetValidatorTotalDelegation returns total balace stacking for validator with delegation.
func (s *PublicBlockChainAPI) GetValidatorTotalDelegation(ctx context.Context, address string) *big.Int {
	delegations := s.b.GetDelegationsByValidator(internal_common.ParseAddr(address))
	totalStake := big.NewInt(0)
	for _, delegation := range delegations {
		totalStake.Add(totalStake, delegation.Amount)
	}
	return totalStake
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockChainAPI) GetShardingStructure(ctx context.Context) ([]map[string]interface{}, error) {
	// Get header and number of shards.
	epoch := s.GetEpoch(ctx)
	numShard := shard.Schedule.InstanceForEpoch(big.NewInt(int64(epoch))).NumShards()

	// Return shareding structure for each case.
	return shard.Schedule.GetShardingStructure(int(numShard), int(s.b.GetShardID())), nil
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

// GetBalance returns the amount of Nano for the given address in the state.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address string) (*big.Int, error) {
	addr := internal_common.ParseAddr(address)
	return s.b.GetBalance(addr)
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

var (
	errNotBeaconChainShard = errors.New("cannot call this rpc on non beaconchain node")
)

// GetMedianRawStakeSnapshot returns the raw median stake, only meant to be called on beaconchain
// explorer node
func (s *PublicBlockChainAPI) GetMedianRawStakeSnapshot() (*big.Int, error) {
	if s.b.GetShardID() == shard.BeaconChainShardID {
		return s.b.GetMedianRawStakeSnapshot(), nil
	}
	return nil, errNotBeaconChainShard
}

// GetAllValidatorAddresses returns all validator addresses.
func (s *PublicBlockChainAPI) GetAllValidatorAddresses() ([]string, error) {
	addresses := []string{}
	for _, addr := range s.b.GetAllValidatorAddresses() {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		addresses = append(addresses, oneAddr)
	}
	return addresses, nil
}

// GetActiveValidatorAddresses returns active validator addresses.
func (s *PublicBlockChainAPI) GetActiveValidatorAddresses() ([]string, error) {
	addresses := []string{}
	for _, addr := range s.b.GetActiveValidatorAddresses() {
		oneAddr, _ := internal_common.AddressToBech32(addr)
		addresses = append(addresses, oneAddr)
	}
	return addresses, nil
}

// GetValidatorMetrics ..
func (s *PublicBlockChainAPI) GetValidatorMetrics(ctx context.Context, address string) (*staking.ValidatorStats, error) {
	validatorAddress := internal_common.ParseAddr(address)
	stats := s.b.GetValidatorStats(validatorAddress)
	if stats == nil {
		return nil, fmt.Errorf("validator stats not found: %s", validatorAddress.Hex())
	}
	return stats, nil
}

// GetValidatorInformation returns information about a validator.
func (s *PublicBlockChainAPI) GetValidatorInformation(ctx context.Context, address string) (*staking.Validator, error) {
	validatorAddress := internal_common.ParseAddr(address)
	validator := s.b.GetValidatorInformation(validatorAddress)
	if validator == nil {
		return nil, fmt.Errorf("validator not found: %s", validatorAddress.Hex())
	}
	return validator, nil
}

// GetDelegationsByDelegator returns list of delegations for a delegator address.
func (s *PublicBlockChainAPI) GetDelegationsByDelegator(ctx context.Context, address string) ([]*RPCDelegation, error) {
	delegatorAddress := internal_common.ParseAddr(address)
	validators, delegations := s.b.GetDelegationsByDelegator(delegatorAddress)
	result := []*RPCDelegation{}
	for i := range delegations {
		delegation := delegations[i]

		undelegations := []RPCUndelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validators[i])
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)
		result = append(result, &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		})
	}
	return result, nil
}

// GetDelegationsByValidator returns list of delegations for a validator address.
func (s *PublicBlockChainAPI) GetDelegationsByValidator(ctx context.Context, address string) ([]*RPCDelegation, error) {
	validatorAddress := internal_common.ParseAddr(address)
	delegations := s.b.GetDelegationsByValidator(validatorAddress)
	result := make([]*RPCDelegation, 0)
	for _, delegation := range delegations {

		undelegations := []RPCUndelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegation.DelegatorAddress)
		result = append(result, &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		})
	}
	return result, nil
}

// GetDelegationByDelegatorAndValidator returns a delegation for delegator and validator.
func (s *PublicBlockChainAPI) GetDelegationByDelegatorAndValidator(ctx context.Context, address string, validator string) (*RPCDelegation, error) {
	delegatorAddress := internal_common.ParseAddr(address)
	validatorAddress := internal_common.ParseAddr(validator)
	validators, delegations := s.b.GetDelegationsByDelegator(delegatorAddress)
	for i := range delegations {
		if validators[i] != validatorAddress {
			continue
		}
		delegation := delegations[i]

		undelegations := []RPCUndelegation{}

		for j := range delegation.Undelegations {
			undelegations = append(undelegations, RPCUndelegation{
				delegation.Undelegations[j].Amount,
				delegation.Undelegations[j].Epoch,
			})
		}
		valAddr, _ := internal_common.AddressToBech32(validatorAddress)
		delAddr, _ := internal_common.AddressToBech32(delegatorAddress)
		return &RPCDelegation{
			valAddr,
			delAddr,
			delegation.Amount,
			delegation.Reward,
			undelegations,
		}, nil
	}
	return nil, nil
}

// GetCurrentUtilityMetrics ..
func (s *PublicBlockChainAPI) GetCurrentUtilityMetrics() (*network.UtilityMetric, error) {
	if s.b.GetShardID() == shard.BeaconChainShardID {
		return s.b.GetCurrentUtilityMetrics()
	}
	return nil, errNotBeaconChainShard
}
