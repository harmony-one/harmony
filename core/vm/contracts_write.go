package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/crypto/sha3"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking"
	stakingTypes "github.com/harmony-one/harmony/staking/types"
)

// NOTE: the proposal suggested using 252, but H1 already uses 252-255 for
// other things, so we pick another address instead:
var routerAddress = common.BytesToAddress([]byte{1, 1})

// WriteCapablePrecompiledContractsStaking lists out the write capable precompiled contracts
// which are available after the StakingPrecompileEpoch
// for now, we have only one contract at 252 or 0xfc - which is the staking precompile
var WriteCapablePrecompiledContractsStaking = map[common.Address]WriteCapablePrecompiledContract{
	common.BytesToAddress([]byte{252}): &stakingPrecompile{},
	// TODO(isd): we probably want to have this in a separate set; this map is used
	// by pre-router H1 deployments.
	routerAddress: &routerPrecompile{},
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

func (m routerMethod) requiredGas() uint64 {
	const sstoreCost = 20000
	switch {
	case m.send != nil:
		storedWords := uint64(len(m.send.payload))/32 + 10
		return sstoreCost * storedWords
	case m.retrySend != nil:
		return 3 * sstoreCost
	default:
		panic("Exactly one of send or retrySend must be set.")
	}
}

type routerSendArgs struct {
	to                            common.Address
	toShard                       uint32
	payload                       []byte
	gasBudget, gasPrice, gasLimit *big.Int
	gasLeftoverTo                 common.Address
}

type routerRetrySendArgs struct {
	msgAddr            common.Address
	gasLimit, gasPrice *big.Int
}

// TODO: probably should go in some utility module somewhere, if we don't have one
// already
func readBig(input []byte) *big.Int {
	ret := &big.Int{}
	for _, b := range input {
		ret = ret.Lsh(ret, 8)
		ret = ret.Add(ret, big.NewInt(int64(b)))
	}
	return ret
}

func parseRouterMethod(input []byte) (m routerMethod, err error) {
	if len(input) < 4 {
		return m, fmt.Errorf(
			"Input too short (%d bytes); does not contain method identifier.",
			len(input),
		)
	}
	methodId := binary.BigEndian.Uint32(input[:4])
	args := input[4:]

	argWord := func(i int) []byte {
		return args[32*i : 32*(i+1)]
	}
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
				gasLimit: readBig(argWord(1)),
				gasPrice: readBig(argWord(2)),
			},
		}
		copy(m.retrySend.msgAddr[:], argWord(0)[12:])
		return m, nil
	case 0x3ba2ea6b: // send
		if len(args) < 32*8 {
			return m, fmt.Errorf(
				"Arguments to send() are too short: %d",
				len(args),
			)
		}
		m = routerMethod{
			send: &routerSendArgs{
				toShard:   binary.BigEndian.Uint32(argWord(1)[32-4:]),
				gasBudget: readBig(argWord(3)),
				gasPrice:  readBig(argWord(4)),
				gasLimit:  readBig(argWord(5)),
			},
		}
		copy(m.send.to[:], argWord(0)[12:])
		copy(m.send.gasLeftoverTo[:], argWord(6)[12:])
		payloadOffset := readBig(argWord(2))
		if !payloadOffset.IsInt64() {
			return m, fmt.Errorf("Payload offset is too large: %v", payloadOffset)
		}
		off := payloadOffset.Int64()
		if off > int64(len(args)) {
			return m, fmt.Errorf("Payload offset is out of bounds: %v (max %v)",
				off, len(args))
		}
		payloadLen := readBig(args[off : off+32])
		if !payloadLen.IsInt64() {
			return m, fmt.Errorf("Payload length is *way* too big: %v", payloadLen)
		}
		payloadBytes := args[off+32:]
		payloadLen64 := payloadLen.Int64()
		if payloadLen64 > int64(len(payloadBytes)) {
			return m, fmt.Errorf("Payload length is out of bounds: %v (max %v)",
				payloadLen64, len(payloadBytes))
		}
		m.send.payload = payloadBytes[:payloadLen64]
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
	m, err := parseRouterMethod(input)
	if err != nil {
		return 0, fmt.Errorf("Error parsing method arguments: %w", err)
	}
	return m.requiredGas(), nil
}

func (c *routerPrecompile) RunWriteCapable(
	evm *EVM,
	contract *Contract,
	input []byte,
) ([]byte, error) {
	// XXX: it would be nice if we could avoid parsing this twice, since we
	// already did it in requiredGas.
	m, err := parseRouterMethod(input)
	if err != nil {
		return nil, fmt.Errorf("Error parsing method arguments: %w", err)
	}

	switch {
	case m.send != nil:
		args := m.send
		fromAddr := contract.Caller()
		fromShard := evm.Context.ShardID
		value := contract.Value()
		if args.gasBudget.Cmp(value) < 0 {
			return nil, fmt.Errorf(
				"gasBudget (%v) is less that attached value (%v).",
				args.gasBudget,
				value,
			)
		}
		gasPriceTimesLimit := (&big.Int{}).Mul(args.gasPrice, args.gasLimit)
		if args.gasBudget.Cmp(gasPriceTimesLimit) < 0 {
			return nil, fmt.Errorf(
				"gasBudget (%v) is less than gasPrice * gasLimit (%v * %v = %v)",
				args.gasBudget,
				args.gasPrice,
				args.gasLimit,
				gasPriceTimesLimit,
			)
		}
		value.Sub(value, args.gasBudget) // value transferred (rather than spent on gas).

		// compute the message address
		h := sha3.NewLegacyKeccak256()
		h.Write(args.payload)
		payloadHash := h.Sum(nil)
		h.Reset()
		h.Write([]byte{0xff})
		h.Write(fromAddr[:])
		h.Write(args.to[:])
		binary.Write(h, binary.BigEndian, fromShard)
		binary.Write(h, binary.BigEndian, args.toShard)
		h.Write(payloadHash)
		var valBytes [32]byte
		value.FillBytes(valBytes[:])
		h.Write(valBytes[:])
		nonce := c.newNonce(evm.StateDB, fromAddr)
		binary.Write(h, binary.BigEndian, nonce)
		msgAddr := h.Sum(nil)[12:]

		// Store data in the message outbox:
		ms := msgStorage{db: evm.StateDB}
		copy(ms.addr[:], msgAddr)

		ms.StoreAddr(msIdxFromAddr, fromAddr)
		ms.StoreAddr(msIdxToAddr, args.to)
		ms.StoreNonceToShard(nonce, args.toShard)
		ms.StoreU256(msIdxValue, value)
		ms.StoreU256(msIdxGasBudget, args.gasBudget)
		ms.StoreU256(msIdxGasPrice, args.gasPrice)
		ms.StoreU256(msIdxGasLimit, args.gasLimit)
		ms.StoreAddr(msIdxGasLeftoverTo, args.gasLeftoverTo)
		ms.StoreLen(msIdxPayloadLen, len(args.payload))
		ms.StoreSlice(msIdxPayloadHash, payloadHash)

		panic("TODO: emit a receipt/actually send the message somehow")
	case m.retrySend != nil:
		panic("TODO")
	default:
		panic("parseRouterMethod left all fields nil!")
	}
}

// Offsets (in words) of various data in a msgStorage
const (
	msIdxFromAddr = iota
	msIdxToAddr
	msIdxNonceToShard // The nonce and toShard fields live in the same word.
	msIdxValue
	msIdxGasBudget
	msIdxGasPrice
	msIdxGasLimit
	msIdxGasLeftoverTo
	msIdxPayloadLen
	msIdxPayloadHash
)

type msgStorage struct {
	addr common.Address
	db   StateDB
}

// compute the storage address of the nth word in the entry for
// this message.
func (ms msgStorage) wordAddr(n uint8) common.Hash {
	var ret common.Hash
	copy(ret[:], ms.addr[:])
	ret[20] = 0x01
	ret[31] = n
	return ret
}

func (ms msgStorage) StoreWord(n uint8, w common.Hash) {
	ms.db.SetState(routerAddress, ms.wordAddr(n), w)
}

func (ms msgStorage) StoreAddr(n uint8, addr common.Address) {
	var buf common.Hash
	copy(buf[:], addr[:])
	ms.StoreWord(n, buf)
}

func (ms msgStorage) StoreU256(n uint8, val *big.Int) {
	var buf common.Hash
	val.FillBytes(buf[:])
	ms.StoreWord(n, buf)
}

func (ms msgStorage) StoreLen(n uint8, val int) {
	var buf common.Hash
	binary.BigEndian.PutUint64(buf[:], uint64(val))
	ms.StoreWord(n, buf)
}

// Store a byte slice, which must be at most one word in length (panics otherwise).
func (ms msgStorage) StoreSlice(n uint8, buf []byte) {
	var wbuf common.Hash
	if len(buf) < len(wbuf[:]) {
		panic("Buffer is the wrong size")
	}
	copy(wbuf[:], buf)
	ms.StoreWord(n, wbuf)
}

func (ms msgStorage) StoreNonceToShard(nonce uint64, toShard uint32) {
	var buf common.Hash
	binary.BigEndian.PutUint64(buf[:8], nonce)
	binary.BigEndian.PutUint32(buf[8:8+4], toShard)
	ms.StoreWord(msIdxNonceToShard, buf)
}

func (c *routerPrecompile) newNonce(db StateDB, addr common.Address) uint64 {
	nonce := db.GetNonce(addr)
	db.SetNonce(addr, nonce+1)
	return nonce
}
