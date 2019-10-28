package hmyapi

import (
	"context"
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
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		blockArgs := BlockArgs{WithSigners: false, InclTx: true, FullTx: fullTx}
		response, err := RPCMarshalBlock(block, blockArgs)
		if err == nil && blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		blockArgs := BlockArgs{WithSigners: false, InclTx: true, FullTx: fullTx}
		return RPCMarshalBlock(block, blockArgs)
	}
	return nil, err
}

// GetBlockByNumberNew returns the requested block. When blockNr is -1 the chain head is returned. When fullTx in blockArgs is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockChainAPI) GetBlockByNumberNew(ctx context.Context, blockNr rpc.BlockNumber, blockArgs BlockArgs) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	blockArgs.InclTx = true
	if blockArgs.WithSigners {
		blockArgs.Signers, err = s.GetBlockSigners(ctx, blockNr)
		if err != nil {
			return nil, err
		}
	}
	if block != nil {
		response, err := RPCMarshalBlock(block, blockArgs)
		if err == nil && blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// GetBlockByHashNew returns the requested block. When fullTx in blockArgs is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned. When withSigners in BlocksArgs is true
// it shows block signers for this block in list of one addresses.
func (s *PublicBlockChainAPI) GetBlockByHashNew(ctx context.Context, blockHash common.Hash, blockArgs BlockArgs) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	blockArgs.InclTx = true
	if blockArgs.WithSigners {
		blockArgs.Signers, err = s.GetBlockSigners(ctx, rpc.BlockNumber(block.NumberU64()))
		if err != nil {
			return nil, err
		}
	}
	if block != nil {
		return RPCMarshalBlock(block, blockArgs)
	}
	return nil, err
}

// GetBlocks method returns blocks in range blockStart, blockEnd just like GetBlockByNumber but all at once.
func (s *PublicBlockChainAPI) GetBlocks(ctx context.Context, blockStart rpc.BlockNumber, blockEnd rpc.BlockNumber, blockArgs BlockArgs) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0)
	for i := blockStart; i <= blockEnd; i++ {
		block, err := s.b.BlockByNumber(ctx, i)
		blockArgs.InclTx = true
		if blockArgs.WithSigners {
			blockArgs.Signers, err = s.GetBlockSigners(ctx, rpc.BlockNumber(i))
			if err != nil {
				return nil, err
			}
		}
		if block != nil {
			rpcBlock, err := RPCMarshalBlock(block, blockArgs)
			if err == nil && i == rpc.PendingBlockNumber {
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
	for _, validator := range committee.NodeList {
		validatorBalance := new(hexutil.Big)
		validatorBalance, err = s.b.GetBalance(validator.EcdsaAddress)
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
func (s *PublicBlockChainAPI) GetBlockSigners(ctx context.Context, blockNr rpc.BlockNumber) ([]string, error) {
	if uint64(blockNr) == 0 {
		return make([]string, 0), nil
	}
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if err != nil {
		return nil, err
	}
	blockWithSigners, err := s.b.BlockByNumber(ctx, blockNr+1)
	if err != nil {
		return nil, err
	}
	committee, err := s.b.GetValidators(block.Epoch())
	if err != nil {
		return nil, err
	}
	pubkeys := make([]*bls.PublicKey, len(committee.NodeList))
	for i, validator := range committee.NodeList {
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
	for _, validator := range committee.NodeList {
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
func (s *PublicBlockChainAPI) IsBlockSigner(ctx context.Context, blockNr rpc.BlockNumber, address string) (bool, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if err != nil {
		return false, err
	}
	blockWithSigners, err := s.b.BlockByNumber(ctx, blockNr+1)
	if err != nil {
		return false, err
	}
	committee, err := s.b.GetValidators(block.Epoch())
	if err != nil {
		return false, err
	}
	pubkeys := make([]*bls.PublicKey, len(committee.NodeList))
	for i, validator := range committee.NodeList {
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
	for _, validator := range committee.NodeList {
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
func (s *PublicBlockChainAPI) GetSignedBlocks(ctx context.Context, address string) hexutil.Uint64 {
	header := s.LatestHeader(ctx)
	totalSigned := uint64(0)
	lastBlock := uint64(0)
	if header.BlockNumber >= defaultBlocksPeriod {
		lastBlock = header.BlockNumber - defaultBlocksPeriod + 1
	}
	for i := header.BlockNumber; i >= lastBlock; i-- {
		signed, err := s.IsBlockSigner(ctx, rpc.BlockNumber(i), address)
		if err == nil && signed {
			totalSigned++
		}
	}
	return hexutil.Uint64(totalSigned)
}

// GetEpoch returns current epoch.
func (s *PublicBlockChainAPI) GetEpoch(ctx context.Context) hexutil.Uint64 {
	return hexutil.Uint64(s.LatestHeader(ctx).Epoch)
}

// GetLeader returns current shard leader.
func (s *PublicBlockChainAPI) GetLeader(ctx context.Context) string {
	return s.LatestHeader(ctx).Leader
}

// GetValidatorInformation returns full validator info.
func (s *PublicBlockChainAPI) GetValidatorInformation(ctx context.Context, address string) (map[string]interface{}, error) {
	validator := s.b.GetValidatorInformation(internal_common.ParseAddr(address))
	slotPubKeys := make([]string, 0)
	for _, slotPubKey := range validator.SlotPubKeys {
		slotPubKeys = append(slotPubKeys, slotPubKey.Hex())
	}
	fields := map[string]interface{}{
		"address":                 validator.Address.String(),
		"stake":                   hexutil.Uint64(validator.Stake.Uint64()),
		"name":                    validator.Description.Name,
		"slotPubKeys":             slotPubKeys,
		"unbondingHeight":         hexutil.Uint64(validator.UnbondingHeight.Uint64()),
		"minSelfDelegation":       hexutil.Uint64(validator.MinSelfDelegation.Uint64()),
		"active":                  validator.Active,
		"identity":                validator.Description.Identity,
		"commissionRate":          hexutil.Uint64(validator.Commission.CommissionRates.Rate.Int.Uint64()),
		"commissionUpdateHeight":  hexutil.Uint64(validator.Commission.UpdateHeight.Uint64()),
		"commissionMaxRate":       hexutil.Uint64(validator.Commission.CommissionRates.MaxRate.Uint64()),
		"commissionMaxChangeRate": hexutil.Uint64(validator.Commission.CommissionRates.MaxChangeRate.Uint64()),
		"website":                 validator.Description.Website,
		"securityContact":         validator.Description.SecurityContact,
		"details":                 validator.Description.Details,
	}
	return fields, nil
}

// GetStake returns validator stake.
func (s *PublicBlockChainAPI) GetStake(ctx context.Context, address string) hexutil.Uint64 {
	validator := s.b.GetValidatorInformation(internal_common.ParseAddr(address))
	return hexutil.Uint64(validator.Stake.Uint64())
}

// GetValidatorStakingAddress stacking address returns validator stacking address.
func (s *PublicBlockChainAPI) GetValidatorStakingAddress(ctx context.Context, address string) string {
	validator := s.b.GetValidatorInformation(internal_common.ParseAddr(address))
	return validator.Address.String()
}

// GetValidatorStakingWithDelegation returns total balace stacking for validator with delegation.
func (s *PublicBlockChainAPI) GetValidatorStakingWithDelegation(ctx context.Context, address string) hexutil.Uint64 {
	return hexutil.Uint64(s.b.GetValidatorStakingWithDelegation(internal_common.ParseAddr(address)).Uint64())
}

// GetDelegatorsInformation returns list of delegators for a validator address.
func (s *PublicBlockChainAPI) GetDelegatorsInformation(ctx context.Context, address string) ([]map[string]interface{}, error) {
	delegators := s.b.GetDelegatorsInformation(internal_common.ParseAddr(address))
	delegatorsFields := make([]map[string]interface{}, 0)
	for _, delegator := range delegators {
		fields := map[string]interface{}{
			"delegator": delegator.DelegatorAddress.String(),
			"amount":    hexutil.Uint64(delegator.Amount.Uint64()),
		}
		delegatorsFields = append(delegatorsFields, fields)
	}
	return delegatorsFields, nil
}

// GetShardingStructure returns an array of sharding structures.
func (s *PublicBlockChainAPI) GetShardingStructure(ctx context.Context) ([]map[string]interface{}, error) {
	// Get header and number of shards.
	epoch := s.GetEpoch(ctx)
	numShard := core.ShardingSchedule.InstanceForEpoch(big.NewInt(int64(epoch))).NumShards()

	// Return shareding structure for each case.
	return core.ShardingSchedule.GetShardingStructure(int(numShard), int(s.b.GetShardID())), nil
}

// GetShardID returns shard ID of the requested node.
func (s *PublicBlockChainAPI) GetShardID(ctx context.Context) (int, error) {
	return int(s.b.GetShardID()), nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, addr string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	code := state.GetCode(address)
	return code, state.Error()
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, addr string, key string, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	address := internal_common.ParseAddr(addr)
	state, _, err := s.b.StateAndHeaderByNumber(ctx, blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	res := state.GetState(address, common.HexToHash(key))
	return res[:], state.Error()
}

// GetBalance returns the amount of Nano for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address string, blockNr rpc.BlockNumber) (*hexutil.Big, error) {
	// TODO: currently only get latest balance. Will add complete logic later.
	addr := internal_common.ParseAddr(address)
	return s.b.GetBalance(addr)
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockChainAPI) BlockNumber() hexutil.Uint64 {
	header, _ := s.b.HeaderByNumber(context.Background(), rpc.LatestBlockNumber) // latest header should always be available
	return hexutil.Uint64(header.Number().Uint64())
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
