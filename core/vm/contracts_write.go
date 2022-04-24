package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// WriteCapablePrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the StakingPrecompileEpoch
// for now, we have only one contract at 252 or 0xfc - which is the staking precompile
var WriteCapablePrecompiledContractsStaking = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
	// TODO(isd): we probably want to have this in a separate set; this map is used
	// by pre-router H1 deployments. Also note: the proposal suggested using 252,
	// but H1 already uses 252-255 for other things, so we pick another address instead:
	common.BytesToAddress([]byte{1, 1}): &routerPrecompile{},
}

// WriteCapablePrecompiledContract represents the interface for Native Go contracts
// which are available as a precompile in the EVM
// As with (read-only) PrecompiledContracts, these need a RequiredGas function
// Note that these contracts have the capability to alter the state
// while those in contracts.go do not
type WriteCapablePrecompiledContract interface {
	// RequiredGas calculates the contract gas use
	RequiredGas(evm *EVM, contract *Contract, input []byte) (uint64, error)
	// use a different name from read-only contracts to be safe
	RunWriteCapable(evm *EVM, contract *Contract, input []byte) ([]byte, error)
}

// RunWriteCapablePrecompiledContract runs and evaluates the output of a write capable precompiled contract.
func RunWriteCapablePrecompiledContract(
	p WriteCapablePrecompiledContract,
	evm *EVM,
	contract *Contract,
	input []byte,
	readOnly bool,
) ([]byte, error) {
	// immediately error out if readOnly
	if readOnly {
		return nil, errWriteProtection
	}
	gas, err := p.RequiredGas(evm, contract, input)
	if err != nil {
		return nil, err
	}
	if !contract.UseGas(gas) {
		return nil, ErrOutOfGas
	}
	return p.RunWriteCapable(evm, contract, input)
}

type stakingPrecompile struct{}

// RequiredGas returns the gas required to execute the pre-compiled contract.
//
// This method does not require any overflow checking as the input size gas costs
// required for anything significant is so high it's impossible to pay for.
func (c *stakingPrecompile) RequiredGas(
	evm *EVM,
	contract *Contract,
	input []byte,
) (uint64, error) {
	// if invalid data or invalid shard
	// set payload to blank and charge minimum gas
	var payload []byte = make([]byte, 0)
	// availability of staking and precompile has already been checked
	if evm.Context.ShardID == shard.BeaconChainShardID {
		// check that input is well formed
		// meaning all the expected parameters are available
		// and that we are only trying to perform staking tx
		// on behalf of the correct entity
		stakeMsg, err := staking.ParseStakeMsg(contract.Caller(), input)
		if err == nil {
			// otherwise charge similar to a regular staking tx
			if migrationMsg, ok := stakeMsg.(*stakingTypes.MigrationMsg); ok {
				// charge per delegation to migrate
				return evm.CalculateMigrationGas(evm.StateDB,
					migrationMsg,
					evm.ChainConfig().IsS3(evm.EpochNumber),
					evm.ChainConfig().IsIstanbul(evm.EpochNumber),
				)
			} else if encoded, err := rlp.EncodeToBytes(stakeMsg); err == nil {
				payload = encoded
			}
		}
	}
	if gas, err := IntrinsicGas(
		payload,
		false,                                   // contractCreation
		evm.ChainConfig().IsS3(evm.EpochNumber), // homestead
		evm.ChainConfig().IsIstanbul(evm.EpochNumber), // istanbul
		false, // isValidatorCreation
	); err != nil {
		return 0, err // ErrOutOfGas occurs when gas payable > uint64
	} else {
		return gas, nil
	}
}

// RunWriteCapable runs the actual contract (that is it performs the staking)
func (c *stakingPrecompile) RunWriteCapable(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	if evm.Context.ShardID != shard.BeaconChainShardID {
		return nil, errors.New("Staking not supported on this shard")
	}
	stakeMsg, err := staking.ParseStakeMsg(contract.Caller(), input)
	if err != nil {
		return nil, err
	}

	var rosettaBlockTracer RosettaTracer
	if tmpTracker, ok := evm.vmConfig.Tracer.(RosettaTracer); ok {
		rosettaBlockTracer = tmpTracker
	}

	if delegate, ok := stakeMsg.(*stakingTypes.Delegate); ok {
		if err := evm.Delegate(evm.StateDB, rosettaBlockTracer, delegate); err != nil {
			return nil, err
		} else {
			evm.StakeMsgs = append(evm.StakeMsgs, delegate)
			return nil, nil
		}
	}
	if undelegate, ok := stakeMsg.(*stakingTypes.Undelegate); ok {
		return nil, evm.Undelegate(evm.StateDB, rosettaBlockTracer, undelegate)
	}
	if collectRewards, ok := stakeMsg.(*stakingTypes.CollectRewards); ok {
		return nil, evm.CollectRewards(evm.StateDB, rosettaBlockTracer, collectRewards)
	}
	// Migrate is not supported in precompile and will be done in a batch hard fork
	//if migrationMsg, ok := stakeMsg.(*stakingTypes.MigrationMsg); ok {
	//	stakeMsgs, err := evm.MigrateDelegations(evm.StateDB, migrationMsg)
	//	if err != nil {
	//		return nil, err
	//	} else {
	//		for _, stakeMsg := range stakeMsgs {
	//			if delegate, ok := stakeMsg.(*stakingTypes.Delegate); ok {
	//				evm.StakeMsgs = append(evm.StakeMsgs, delegate)
	//			} else {
	//				return nil, errors.New("[StakingPrecompile] Received incompatible stakeMsg from evm.MigrateDelegations")
	//			}
	//		}
	//		return nil, nil
	//	}
	//}
	return nil, errors.New("[StakingPrecompile] Received incompatible stakeMsg from staking.ParseStakeMsg")
}

// routerPrecompile implements the router contract used for cross-shard messaging
// (via the WriteCapablePrecompiledContract interface).
type routerPrecompile struct{}

type routerMethod struct {
	// Exactly one of these, corresponding to the method
	// being called, must be non-nil:
	send      *routerSendArgs
	retrySend *routerRetrySendArgs
}

type routerSendArgs struct {
	to                            common.Address
	toShard                       uint16
	payload                       []byte
	gasBudget, gasPrice, gasLimit *big.Int
	gasLeftoverTo                 common.Address
}

type routerRetrySendArgs struct {
	msgAddr            common.Address
	gasLimit, gasPrice *big.Int
}

func readBig(input []byte) *big.Int {
	ret := &big.Int{}
	for _, b := range input {
		ret = ret.Lsh(ret, 8)
		ret = ret.Add(ret, big.NewInt(int64(b)))
	}
	return ret
}

func parseRouterMethod(input []byte) (m routerMethod, err error) {
	fmt.Fprintf(os.Stderr, "input: %q\n", input)
	if len(input) < 4 {
		return m, fmt.Errorf(
			"Input too short (%d bytes); does not contain method identifier.",
			len(input),
		)
	}
	methodId := binary.BigEndian.Uint32(input[:4])
	args := input[4:]
	switch methodId {
	// TODO(cleanup): make constants for these, or something.
	case 0x0db24a7d: // retrySend
		if len(args) != 32*3 {
			return m, fmt.Errorf(
				"Arguments to retrySend() are the wrong length: %d",
				len(args),
			)
		}
		m = routerMethod{
			retrySend: &routerRetrySendArgs{
				gasLimit: readBig(input[32:64]),
				gasPrice: readBig(input[64:96]),
			},
		}
		copy(m.retrySend.msgAddr[:], input[12:32])
		return m, nil
	case 0x98054083: // send
		if len(args) < 32*7 {
			return m, fmt.Errorf(
				"Arguments to send() are too short: %d",
				len(args),
			)
		}
		m = routerMethod{
			send: &routerSendArgs{
				toShard:   binary.BigEndian.Uint16(input[62:64]),
				gasBudget: readBig(input[96:128]),
				gasPrice:  readBig(input[128:160]),
				gasLimit:  readBig(input[160:192]),
			},
		}
		copy(m.send.to[:], input[12:32])
		copy(m.send.gasLeftoverTo[:], input[192+12:192+32])
		payloadOffset := readBig(input[64:96])
		if !payloadOffset.IsInt64() {
			return m, fmt.Errorf("Payload offset is too large: %v", payloadOffset)
		}
		off := payloadOffset.Int64()
		if off > int64(len(input)) {
			return m, fmt.Errorf("Payload offset is out of bounds: %v (max %v)",
				off, len(input))
		}
		m.send.payload = input[off:]
		return m, nil
	default:
		return m, fmt.Errorf("Unknown method id: 0x%x\n", methodId)
	}
}

func (c *routerPrecompile) RequiredGas(
	evm *EVM,
	contract *Contract,
	input []byte,
) (uint64, error) {
	fmt.Fprintf(os.Stderr, "[RouterPrecompile]: RequiredGas(...)\n")

	m, err := parseRouterMethod(input)
	if err != nil {
		fmt.Fprint(os.Stderr, "parseRouterMethod: ", err)
		return 0, nil
	}
	fmt.Fprint(os.Stderr, "router method: ", m)
	panic("TODO")
}

func (c *routerPrecompile) RunWriteCapable(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "[RouterPrecompile]: RunWriteCapable(_, _, %q)\n", input)
	return nil, nil
}
