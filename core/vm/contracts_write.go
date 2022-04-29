package vm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/crypto/sha3"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
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
		return storedWords * sstoreCost
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

func (args routerSendArgs) MakeMessage(evm *EVM, contract *Contract) (types.CXMessage, error) {
	m := types.CXMessage{
		From:      contract.Caller(),
		FromShard: evm.Context.ShardID,
		To:        args.to,
		ToShard:   args.toShard,
		Payload:   args.payload,
		GasBudget: args.gasBudget,
		GasPrice:  args.gasPrice,
		GasLimit:  args.gasLimit,
	}
	totalValue := contract.Value()
	if args.gasBudget.Cmp(totalValue) < 0 {
		return m, fmt.Errorf(
			"gasBudget (%v) is less that attached value (%v).",
			args.gasBudget,
			totalValue,
		)
	}
	m.Value = (&big.Int{}).Sub(totalValue, args.gasBudget)
	gasPriceTimesLimit := (&big.Int{}).Mul(args.gasPrice, args.gasLimit)
	if args.gasBudget.Cmp(gasPriceTimesLimit) < 0 {
		return m, fmt.Errorf(
			"gasBudget (%v) is less than gasPrice * gasLimit (%v * %v = %v)",
			args.gasBudget,
			args.gasPrice,
			args.gasLimit,
			gasPriceTimesLimit,
		)
	}
	return m, nil
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
		msg, err := m.send.MakeMessage(evm, contract)
		if err != nil {
			return nil, err
		}
		msgAddr, payloadHash := messageAddrAndPayloadHash(msg)
		msgStorage{
			db:   evm.StateDB,
			addr: msgAddr,
		}.StoreMessage(msg, payloadHash)

		// TODO: We should be returning the message address, but I'm not
		// 100% sure this is the right way to encode it. Check.
		return msgAddr[:], evm.Context.EmitCXMessage(msg)
	case m.retrySend != nil:
		ms := msgStorage{
			db:   evm.StateDB,
			addr: m.retrySend.msgAddr,
		}
		oldMsg, _ := ms.LoadMessage(evm.Context.ShardID)
		newMsg := oldMsg
		newMsg.GasLimit = m.retrySend.gasLimit
		newMsg.GasPrice = m.retrySend.gasPrice
		newMsg.GasBudget = (&big.Int{}).Add(oldMsg.GasBudget, contract.Value())

		if newMsg.GasLimit.Cmp(oldMsg.GasLimit) < 0 {
			return nil, fmt.Errorf(
				"New gas limit (%v) must not be smaller than old gas limit (%v).",
				newMsg.GasLimit, oldMsg.GasLimit,
			)
		}
		if newMsg.GasPrice.Cmp(oldMsg.GasPrice) < 0 {
			return nil, fmt.Errorf(
				"New gas price (%v) must not be smaller than old gas price (%v).",
				newMsg.GasPrice, oldMsg.GasPrice,
			)
		}
		gasPriceTimesLimit := (&big.Int{}).Mul(newMsg.GasPrice, newMsg.GasLimit)
		if newMsg.GasBudget.Cmp(gasPriceTimesLimit) < 0 {
			return nil, fmt.Errorf(
				"new gas budget (%v) must be at least gasPrice * gasLimit (%v).",
				newMsg.GasBudget, gasPriceTimesLimit,
			)
		}

		// Save just the bits that changed:
		ms.StoreU256(msIdxGasBudget, newMsg.GasBudget)
		ms.StoreU256(msIdxGasPrice, newMsg.GasPrice)
		ms.StoreU256(msIdxGasLimit, newMsg.GasLimit)

		return nil, evm.Context.EmitCXMessage(newMsg)
	default:
		panic("parseRouterMethod left all fields nil!")
	}
}

// Compute the message address and address of the payload.
func messageAddrAndPayloadHash(m types.CXMessage) (common.Address, common.Hash) {
	h := sha3.NewLegacyKeccak256()
	h.Write(m.Payload)
	var payloadHash common.Hash
	copy(payloadHash[:], h.Sum(nil))
	h.Reset()
	h.Write([]byte{0xff})
	h.Write(m.From[:])
	h.Write(m.To[:])
	binary.Write(h, binary.BigEndian, m.FromShard)
	binary.Write(h, binary.BigEndian, m.ToShard)
	h.Write(payloadHash[:])
	var valBytes [32]byte
	m.Value.FillBytes(valBytes[:])
	h.Write(valBytes[:])
	binary.Write(h, binary.BigEndian, m.Nonce)
	var msgAddr common.Address
	copy(msgAddr[:], h.Sum(nil)[12:])
	return msgAddr, payloadHash
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

func (ms msgStorage) StoreMessage(m types.CXMessage, payloadHash common.Hash) {
	ms.StoreAddr(msIdxFromAddr, m.From)
	ms.StoreAddr(msIdxToAddr, m.To)
	ms.StoreNonceToShard(m.Nonce, m.ToShard)
	ms.StoreU256(msIdxValue, m.Value)
	ms.StoreU256(msIdxGasBudget, m.GasBudget)
	ms.StoreU256(msIdxGasPrice, m.GasPrice)
	ms.StoreU256(msIdxGasLimit, m.GasLimit)
	ms.StoreAddr(msIdxGasLeftoverTo, m.GasLeftoverTo)
	ms.StoreLen(msIdxPayloadLen, len(m.Payload))
	ms.StoreWord(msIdxPayloadHash, payloadHash)
	storePayload(ms.db, payloadHash, m.Payload)
}

func storePayload(db StateDB, hash common.Hash, data []byte) {
	offset := readBig(hash[:])
	key := hash
	for len(data) > 0 {
		var val common.Hash
		copy(val[:], data[:])
		db.SetState(routerAddress, key, val)
		if len(data) < len(val[:]) {
			data = nil
		} else {
			data = data[len(val[:]):]
			offset.Add(offset, big.NewInt(1))
			offset.FillBytes(key[:])
		}
	}
}

// Load a message from storage. The fromShard field is supplied by the
// caller; it is not stored, since storage is per-shard anyway.
func (ms msgStorage) LoadMessage(fromShard uint32) (msg types.CXMessage, payloadHash common.Hash) {
	nonce, toShard := ms.LoadNonceToShard()
	payloadLen := ms.LoadLen(msIdxPayloadLen)
	payloadHash = ms.LoadWord(msIdxPayloadHash)
	payload := loadPayload(ms.db, payloadHash, payloadLen)
	msg = types.CXMessage{
		From:          ms.LoadAddr(msIdxFromAddr),
		To:            ms.LoadAddr(msIdxToAddr),
		ToShard:       toShard,
		FromShard:     fromShard,
		Nonce:         nonce,
		Value:         ms.LoadU256(msIdxValue),
		GasBudget:     ms.LoadU256(msIdxGasBudget),
		GasPrice:      ms.LoadU256(msIdxGasPrice),
		GasLimit:      ms.LoadU256(msIdxGasLimit),
		GasLeftoverTo: ms.LoadAddr(msIdxGasLeftoverTo),
		Payload:       payload,
	}
	// TODO: sanity check that the hash is correct.
	return
}

func loadPayload(db StateDB, hash common.Hash, length int) []byte {
	ret := make([]byte, length)
	buf := ret
	offset := readBig(hash[:])
	key := hash
	for len(buf) > 0 {
		word := db.GetState(routerAddress, key)
		copy(buf, word[:])
		if len(buf) < len(word) {
			buf = nil
		} else {
			buf = buf[len(word):]
			offset.Add(offset, big.NewInt(1))
			offset.FillBytes(key[:])
		}
	}
	return ret
}

func (ms msgStorage) StoreWord(n uint8, w common.Hash) {
	ms.db.SetState(routerAddress, ms.wordAddr(n), w)
}

func (ms msgStorage) LoadWord(n uint8) common.Hash {
	return ms.db.GetState(routerAddress, ms.wordAddr(n))
}

func (ms msgStorage) StoreAddr(n uint8, addr common.Address) {
	var buf common.Hash
	copy(buf[:], addr[:])
	ms.StoreWord(n, buf)
}

func (ms msgStorage) LoadAddr(n uint8) common.Address {
	var ret common.Address
	w := ms.LoadWord(n)
	copy(ret[:], w[:])
	return ret
}

func (ms msgStorage) StoreU256(n uint8, val *big.Int) {
	var buf common.Hash
	val.FillBytes(buf[:])
	ms.StoreWord(n, buf)
}

func (ms msgStorage) LoadU256(n uint8) *big.Int {
	w := ms.LoadWord(n)
	return readBig(w[:])
}

func (ms msgStorage) StoreLen(n uint8, val int) {
	var buf common.Hash
	binary.BigEndian.PutUint64(buf[:], uint64(val))
	ms.StoreWord(n, buf)
}

func (ms msgStorage) LoadLen(n uint8) int {
	w := ms.LoadWord(n)
	return int(binary.BigEndian.Uint64(w[:]))
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

func (ms msgStorage) LoadNonceToShard() (nonce uint64, toShard uint32) {
	buf := ms.LoadWord(msIdxNonceToShard)
	nonce = binary.BigEndian.Uint64(buf[:8])
	toShard = binary.BigEndian.Uint32(buf[8 : 8+4])
	return
}

func (c *routerPrecompile) newNonce(db StateDB, addr common.Address) uint64 {
	nonce := db.GetNonce(addr)
	db.SetNonce(addr, nonce+1)
	return nonce
}
