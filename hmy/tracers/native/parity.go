package native

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/hmy/tracers"
	"github.com/holiman/uint256"
)

func init() {
	register("ParityBlockTracer", newParityTracer)
}

func newParityTracer(ctx *tracers.Context, _ json.RawMessage) (tracers.Tracer, error) {
	return &ParityBlockTracer{}, nil
}

func (jst *ParityBlockTracer) CaptureTxStart(gasLimit uint64) {

}

func (jst *ParityBlockTracer) CaptureTxEnd(restGas uint64) {
}

func (jst *ParityBlockTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {

}

func (jst *ParityBlockTracer) CaptureExit(output []byte, gasUsed uint64, err error) {
}

// get TraceBlockStorage from tracer, then store it to db
func (jst *ParityBlockTracer) GetStorage() *TraceBlockStorage {
	blockStorage := &TraceBlockStorage{
		Hash:         jst.Hash,
		Number:       jst.Number,
		addressIndex: make(map[common.Address]int),
		dataIndex:    make(map[common.Hash]int),
	}
	txStorage := &TxStorage{
		Storages: make([]*ActionStorage, 0, 1024),
	}
	var finalize func(ac *action, traceAddress []uint)
	finalize = func(ac *action, traceAddress []uint) {
		acStorage := ac.toStorage(blockStorage)
		acStorage.Subtraces = uint(len(ac.subCalls))
		acStorage.TraceAddress = make([]uint, len(traceAddress))
		copy(acStorage.TraceAddress, traceAddress)
		txStorage.Storages = append(txStorage.Storages, acStorage)
		for i, subAc := range ac.subCalls {
			finalize(subAc, append(traceAddress[:], uint(i)))
		}
	}
	for _, curTx := range jst.tracers {
		root := &curTx.action
		txStorage.Hash = curTx.transactionHash
		txStorage.Storages = txStorage.Storages[:0]
		finalize(root, make([]uint, 0))
		b, _ := rlp.EncodeToBytes(txStorage)
		blockStorage.TraceStorages = append(blockStorage.TraceStorages, b)
	}
	return blockStorage
}

type action struct {
	op       vm.OpCode
	from     common.Address
	to       common.Address
	input    []byte
	output   []byte
	gasIn    uint64
	gasCost  uint64
	gas      uint64
	gasUsed  uint64
	outOff   int64
	outLen   int64
	value    *big.Int
	err      error
	revert   []byte
	subCalls []*action
}

func (c *action) push(ac *action) {
	c.subCalls = append(c.subCalls, ac)
}

func (c *action) fromStorage(blockStorage *TraceBlockStorage, acStorage *ActionStorage) {

	c.op = vm.OpCode(acStorage.readByte())
	errByte := acStorage.readByte()
	if errByte != 0 {
		revertIndex := int(acStorage.readNumber().Int64())
		c.revert = blockStorage.getData(revertIndex)
		c.err = errors.New("Reverted")
	}

	if c.op == vm.CREATE || c.op == vm.CREATE2 {
		fromIndex := int(acStorage.readNumber().Int64())
		toIndex := int(acStorage.readNumber().Int64())
		c.value = acStorage.readNumber()
		inputIndex := int(acStorage.readNumber().Int64())
		outputIndex := int(acStorage.readNumber().Int64())
		c.gas = acStorage.readNumber().Uint64()
		c.gasUsed = acStorage.readNumber().Uint64()

		c.from = blockStorage.getAddress(fromIndex)
		c.to = blockStorage.getAddress(toIndex)
		c.input = blockStorage.getData(inputIndex)
		c.output = blockStorage.getData(outputIndex)
	}
	if c.op == vm.CALL || c.op == vm.CALLCODE || c.op == vm.DELEGATECALL || c.op == vm.STATICCALL {
		fromIndex := int(acStorage.readNumber().Int64())
		toIndex := int(acStorage.readNumber().Int64())
		c.value = acStorage.readNumber()
		inputIndex := int(acStorage.readNumber().Int64())
		outputIndex := int(acStorage.readNumber().Int64())
		c.gas = acStorage.readNumber().Uint64()
		c.gasUsed = acStorage.readNumber().Uint64()

		c.from = blockStorage.getAddress(fromIndex)
		c.to = blockStorage.getAddress(toIndex)
		c.input = blockStorage.getData(inputIndex)
		c.output = blockStorage.getData(outputIndex)
	}
	if c.op == vm.SELFDESTRUCT {
		fromIndex := int(acStorage.readNumber().Int64())
		toIndex := int(acStorage.readNumber().Int64())
		c.from = blockStorage.getAddress(fromIndex)
		c.to = blockStorage.getAddress(toIndex)
		c.value = acStorage.readNumber()
	}
}

func (c action) toStorage(blockStorage *TraceBlockStorage) *ActionStorage {
	acStorage := &ActionStorage{
		TraceData: make([]byte, 0, 1024),
	}

	acStorage.appendByte(byte(c.op))
	var errByte byte
	if c.err != nil {
		errByte = 1
	}
	acStorage.appendByte(errByte)
	if errByte != 0 {
		revertIndex := blockStorage.indexData(c.revert)
		acStorage.appendNumber(big.NewInt(int64(revertIndex)))
	}
	if c.op == vm.CREATE || c.op == vm.CREATE2 {
		fromIndex := blockStorage.indexAddress(c.from)
		toIndex := blockStorage.indexAddress(c.to)
		inputIndex := blockStorage.indexData(c.input)
		outputIndex := blockStorage.indexData(c.output)
		acStorage.appendNumber(big.NewInt(int64(fromIndex)))
		acStorage.appendNumber(big.NewInt(int64(toIndex)))
		acStorage.appendNumber(c.value)
		acStorage.appendNumber(big.NewInt(int64(inputIndex)))
		acStorage.appendNumber(big.NewInt(int64(outputIndex)))
		acStorage.appendNumber((&big.Int{}).SetUint64(c.gas))
		acStorage.appendNumber((&big.Int{}).SetUint64(c.gasUsed))
		return acStorage
	}
	if c.op == vm.CALL || c.op == vm.CALLCODE || c.op == vm.DELEGATECALL || c.op == vm.STATICCALL {
		if c.value == nil {
			c.value = big.NewInt(0)
		}
		fromIndex := blockStorage.indexAddress(c.from)
		toIndex := blockStorage.indexAddress(c.to)
		inputIndex := blockStorage.indexData(c.input)
		outputIndex := blockStorage.indexData(c.output)
		acStorage.appendNumber(big.NewInt(int64(fromIndex)))
		acStorage.appendNumber(big.NewInt(int64(toIndex)))
		acStorage.appendNumber(c.value)
		acStorage.appendNumber(big.NewInt(int64(inputIndex)))
		acStorage.appendNumber(big.NewInt(int64(outputIndex)))
		acStorage.appendNumber((&big.Int{}).SetUint64(c.gas))
		acStorage.appendNumber((&big.Int{}).SetUint64(c.gasUsed))
		return acStorage
	}
	if c.op == vm.SELFDESTRUCT {
		fromIndex := blockStorage.indexAddress(c.from)
		toIndex := blockStorage.indexAddress(c.to)
		acStorage.appendNumber(big.NewInt(int64(fromIndex)))
		acStorage.appendNumber(big.NewInt(int64(toIndex)))
		acStorage.appendNumber(c.value)
		return acStorage
	}
	return nil
}

func (c action) toJsonStr() (string, *string, *string) {
	callType := strings.ToLower(c.op.String())
	if c.op == vm.CREATE || c.op == vm.CREATE2 {
		action := fmt.Sprintf(
			`{"from":"0x%x","gas":"0x%x","init":"0x%x","value":"0x%s"}`,
			c.from, c.gas, c.input, c.value.Text(16),
		)
		output := fmt.Sprintf(
			`{"address":"0x%x","code":"0x%x","gasUsed":"0x%x"}`,
			c.to, c.output, c.gasUsed,
		)
		return "create", &action, &output
	}
	if c.op == vm.CALL || c.op == vm.CALLCODE || c.op == vm.DELEGATECALL || c.op == vm.STATICCALL {
		if c.value == nil {
			c.value = big.NewInt(0)
		}

		var valueStr string
		if c.op != vm.STATICCALL && c.op != vm.DELEGATECALL {
			valueStr = fmt.Sprintf(`,"value":"0x%s"`, c.value.Text(16))
		}

		action := fmt.Sprintf(
			`{"callType":"%s"%s,"to":"0x%x","gas":"0x%x","from":"0x%x","input":"0x%x"}`,
			callType, valueStr, c.to, c.gas, c.from, c.input,
		)

		output := fmt.Sprintf(
			`{"output":"0x%x","gasUsed":"0x%x"}`,
			c.output, c.gasUsed,
		)
		return "call", &action, &output
	}
	if c.op == vm.SELFDESTRUCT {
		action := fmt.Sprintf(
			`{"refundAddress":"0x%x","balance":"0x%s","address":"0x%x"}`,
			c.to, c.value.Text(16), c.from,
		)
		return "suicide", &action, nil
	}
	return "unkonw", nil, nil
}

type ParityTxTracer struct {
	blockNumber         uint64
	blockHash           common.Hash
	transactionPosition uint64
	transactionHash     common.Hash
	descended           bool
	calls               []*action
	action
}

func (ptt *ParityTxTracer) push(ac *action) {
	ptt.calls = append(ptt.calls, ac)
}

func (ptt *ParityTxTracer) pop() *action {
	popIndex := len(ptt.calls) - 1
	ac := ptt.calls[popIndex]
	ptt.calls = ptt.calls[:popIndex]
	return ac
}

func (ptt *ParityTxTracer) last() *action {
	return ptt.calls[len(ptt.calls)-1]
}

func (ptt *ParityTxTracer) len() int {
	return len(ptt.calls)
}

type ParityBlockTracer struct {
	Hash    common.Hash
	Number  uint64
	cur     *ParityTxTracer
	tracers []*ParityTxTracer
}

func (jst *ParityBlockTracer) CaptureStart(env *vm.EVM, from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) {
	jst.cur = &ParityTxTracer{}
	jst.cur.op = vm.CALL // vritual call
	if create {
		jst.cur.op = vm.CREATE // virtual create
	}
	jst.cur.from = from
	jst.cur.to = to
	jst.cur.input = input
	jst.cur.gas = gas
	jst.cur.value = (&big.Int{}).Set(value)
	jst.cur.blockHash = env.StateDB.BlockHash()
	jst.cur.transactionPosition = uint64(env.StateDB.TxIndex())
	jst.cur.transactionHash = env.StateDB.TxHashETH()
	jst.cur.blockNumber = env.Context.BlockNumber.Uint64()
	jst.cur.descended = false
	jst.cur.push(&jst.cur.action)
	return
}

func (jst *ParityBlockTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) {
	jst.cur.output = output
	jst.cur.gasUsed = gasUsed
	if err != nil {
		jst.cur.err = err
	}
	jst.tracers = append(jst.tracers, jst.cur)
	return
}

func (jst *ParityBlockTracer) CaptureState(env *vm.EVM, pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
	var (
		//retErr   error
		stack    = scope.Stack
		memory   = scope.Memory
		contract = scope.Contract
	)
	if err != nil {
		jst.CaptureFault(pc, op, gas, cost, scope, depth, err)
		return
	}

	stackPeek := func(n int) *uint256.Int {
		if n >= len(stack.Data()) {
			//retErr = errors.New("tracer bug:stack overflow")
			return uint256.NewInt(0)
		}
		return stack.Back(n)
	}
	memoryCopy := func(off, size int64) []byte {
		if off+size > int64(memory.Len()) {
			//retErr = errors.New("tracer bug:memory leak")
			return nil
		}
		return memory.GetCopy(off, size)
	}

	switch op {
	case vm.CREATE, vm.CREATE2:
		inOff := int64(stackPeek(1).Uint64())
		inSize := int64(stackPeek(2).Uint64())
		jst.cur.push(&action{
			op:      op,
			from:    contract.Address(),
			input:   memoryCopy(inOff, inSize),
			gasIn:   gas,
			gasCost: cost,
			value:   stackPeek(0).ToBig(),
		})
		jst.cur.descended = true
		return
	case vm.SELFDESTRUCT:
		ac := jst.cur.last()
		ac.push(&action{
			op:      op,
			from:    contract.Address(),
			to:      common.BigToAddress(stackPeek(0).ToBig()),
			gasIn:   gas,
			gasCost: cost,
			value:   env.StateDB.GetBalance(contract.Address()),
		})
		return
	case vm.CALL, vm.CALLCODE, vm.DELEGATECALL, vm.STATICCALL:
		to := common.BigToAddress(stackPeek(1).ToBig())
		precompiles := vm.PrecompiledContractsVRF
		if _, exist := precompiles[to]; exist {
			return
		}
		off := 1
		if op == vm.DELEGATECALL || op == vm.STATICCALL {
			off = 0
		}
		inOff := int64(stackPeek(2 + off).Uint64())
		inSize := int64(stackPeek(3 + off).Uint64())
		callObj := &action{
			op:      op,
			from:    contract.Address(),
			to:      to,
			input:   memoryCopy(inOff, inSize),
			gasIn:   gas,
			gasCost: cost,
			outOff:  stackPeek(4 + off).ToBig().Int64(),
			outLen:  stackPeek(5 + off).ToBig().Int64(),
		}
		if op != vm.DELEGATECALL && op != vm.STATICCALL {
			callObj.value = (&big.Int{}).Set(stackPeek(2).ToBig())
		}
		jst.cur.push(callObj)
		jst.cur.descended = true
		return
	}

	if jst.cur.descended {
		jst.cur.descended = false
		if depth >= jst.cur.len() { // >= to >
			jst.cur.last().gas = gas
		}
	}
	if op == vm.REVERT {
		last := jst.cur.last()
		last.err = errors.New("execution reverted")
		revertOff := int64(stackPeek(0).Uint64())
		revertLen := int64(stackPeek(1).Uint64())
		last.revert = memoryCopy(revertOff, revertLen)
		return
	}
	if depth == jst.cur.len()-1 { // depth == len - 1
		call := jst.cur.pop()
		if call.op == vm.CREATE || call.op == vm.CREATE2 {
			call.gasUsed = call.gasIn - call.gasCost - gas

			ret := stackPeek(0)
			if ret.Sign() != 0 {
				call.to = common.BigToAddress(ret.ToBig())
				call.output = env.StateDB.GetCode(call.to)
			} else if call.err == nil {
				call.err = errors.New("internal failure")
			}
		} else {
			if call.gas != 0 {
				call.gasUsed = call.gasIn - call.gasCost + call.gas - gas
			}
			ret := stackPeek(0)
			if ret.Sign() != 0 {
				call.output = memoryCopy(call.outOff, call.outLen)
			} else if call.err == nil {
				call.err = errors.New("internal failure")
			}
		}
		jst.cur.last().push(call)
	}
	return
}

func (jst *ParityBlockTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
	if jst.cur.last().err != nil {
		return
	}
	call := jst.cur.pop()
	call.err = err
	// Consume all available gas and clean any leftovers
	if call.gas != 0 {
		call.gas = gas
		call.gasUsed = call.gas
	}

	// Flatten the failed call into its parent
	if jst.cur.len() > 0 {
		jst.cur.last().push(call)
		return
	}
	jst.cur.push(call)
	return
}

// GetResult calls the Javascript 'result' function and returns its value, or any accumulated error
func (jst *ParityBlockTracer) GetParityResult() ([]json.RawMessage, error) {
	var results []json.RawMessage
	var err error
	var headPiece string
	var finalize func(ac *action, traceAddress []int)
	finalize = func(ac *action, traceAddress []int) {
		typStr, acStr, outStr := ac.toJsonStr()
		if acStr == nil {
			err = errors.New("tracer internal failure")
			return
		}
		traceStr, _ := json.Marshal(traceAddress)
		bodyPiece := fmt.Sprintf(
			`,"subtraces":%d,"traceAddress":%s,"type":"%s","action":%s`,
			len(ac.subCalls), string(traceStr), typStr, *acStr,
		)
		var resultPiece string
		if ac.err != nil {
			resultPiece = fmt.Sprintf(`,"error":"Reverted","revert":"0x%x"`, ac.revert)

		} else if outStr != nil {
			resultPiece = fmt.Sprintf(`,"result":%s`, *outStr)
		} else {
			resultPiece = `,"result":null`
		}

		jstr := "{" + headPiece + bodyPiece + resultPiece + "}"
		results = append(results, json.RawMessage(jstr))
		for i, subAc := range ac.subCalls {
			finalize(subAc, append(traceAddress[:], i))
		}
	}
	for _, curTx := range jst.tracers {
		root := &curTx.action
		headPiece = fmt.Sprintf(
			`"blockNumber":%d,"blockHash":"%s","transactionHash":"%s","transactionPosition":%d`,
			curTx.blockNumber, curTx.blockHash.Hex(), curTx.transactionHash.Hex(), curTx.transactionPosition,
		)
		finalize(root, make([]int, 0))
	}
	return results, err
}

func (jst *ParityBlockTracer) GetResult() (json.RawMessage, error) {
	var builder strings.Builder
	var err error
	var headPiece string
	builder.WriteString("[")
	var finalize func(ac *action, traceAddress []int)
	finalize = func(ac *action, traceAddress []int) {
		typStr, acStr, outStr := ac.toJsonStr()
		if acStr == nil {
			err = errors.New("tracer internal failure")
			return
		}
		traceStr, _ := json.Marshal(traceAddress)
		bodyPiece := fmt.Sprintf(
			`,"subtraces":%d,"traceAddress":%s,"type":"%s","action":%s`,
			len(ac.subCalls), string(traceStr), typStr, *acStr,
		)
		var resultPiece string
		if ac.err != nil {
			resultPiece = fmt.Sprintf(`,"error":"Reverted","revert":"%x"`, ac.revert)

		} else if outStr != nil {
			resultPiece = fmt.Sprintf(`,"result":%s`, *outStr)
		} else {
			resultPiece = `,"result":null`
		}

		builder.WriteString("{")
		builder.WriteString(headPiece)
		builder.WriteString(bodyPiece)
		builder.WriteString(resultPiece)
		builder.WriteString("}")
		//results = append(results, jstr)
		for i, subAc := range ac.subCalls {
			finalize(subAc, append(traceAddress[:], i))
		}
	}
	for _, curTx := range jst.tracers {
		root := &curTx.action
		headPiece = fmt.Sprintf(
			`"blockNumber":%d,"blockHash":"%s","transactionHash":"%s","transactionPosition":%d`,
			curTx.blockNumber, curTx.blockHash.Hex(), curTx.transactionHash.Hex(), curTx.transactionPosition,
		)
		finalize(root, make([]int, 0))
	}
	builder.WriteString("]")
	return json.RawMessage(builder.String()), err
}

func (jst *ParityBlockTracer) Stop(err error) {

}

type TraceBlockStorage struct {
	Hash           common.Hash
	Number         uint64
	AddressTable   []common.Address       // address table
	DataKeyTable   []common.Hash          // data key table
	dataValueTable [][]byte               // data, store in db, avoid RLPEncode
	TraceStorages  [][]byte               // trace data, length equal the number of transaction in a block
	addressIndex   map[common.Address]int // address index in AddressTable
	dataIndex      map[common.Hash]int    // data index in DataKeyTable
}

// get data of index i
func (ts *TraceBlockStorage) getData(i int) []byte {
	return ts.dataValueTable[i]
}

// get address of index i
func (ts *TraceBlockStorage) getAddress(i int) common.Address {
	return ts.AddressTable[i]
}

// store data and assign an index to it. if data existed, just return it's index
func (ts *TraceBlockStorage) indexData(data []byte) int {
	key := hash.Keccak256Hash(data)
	if index, exist := ts.dataIndex[key]; exist {
		return index
	}
	index := len(ts.DataKeyTable)
	ts.DataKeyTable = append(ts.DataKeyTable, key)
	ts.dataValueTable = append(ts.dataValueTable, data)
	ts.dataIndex[key] = index
	return index
}

// store address and assign an index to it. if address existed, just return it's index
func (ts *TraceBlockStorage) indexAddress(address common.Address) int {
	if index, exist := ts.addressIndex[address]; exist {
		return index
	}
	index := len(ts.addressIndex)
	ts.AddressTable = append(ts.AddressTable, address)
	ts.addressIndex[address] = index
	return index
}

// use this key as
func (ts *TraceBlockStorage) KeyDB() []byte {
	return ts.Hash[:]
}

// store TraceBlockStorage to db
func (ts *TraceBlockStorage) ToDB(write func([]byte, []byte)) {
	for index, key := range ts.DataKeyTable {
		write(key[:], ts.dataValueTable[index])
	}
	bytes, _ := rlp.EncodeToBytes(ts)
	write(ts.KeyDB(), bytes)
}

// load TraceBlockStorage from db
func (ts *TraceBlockStorage) FromDB(read func([]byte) ([]byte, error)) error {
	bytes, err := read(ts.KeyDB())
	if err != nil {
		return err
	}
	err = rlp.DecodeBytes(bytes, ts)
	if err != nil {
		return err
	}
	for _, key := range ts.DataKeyTable {
		data, err := read(key[:])
		if err != nil {
			return err
		}
		ts.dataValueTable = append(ts.dataValueTable, data)
	}
	return nil
}

// return trace result of a tx for giving index
func (ts *TraceBlockStorage) TxJson(index int) ([]json.RawMessage, error) {
	var results []json.RawMessage
	var txStorage TxStorage
	var err error
	b := ts.TraceStorages[index]
	err = rlp.DecodeBytes(b, &txStorage)
	if err != nil {
		return nil, err
	}

	headPiece := fmt.Sprintf(
		`"blockNumber":%d,"blockHash":"%s","transactionHash":"%s","transactionPosition":%d`,
		ts.Number, ts.Hash.Hex(), txStorage.Hash.Hex(), index,
	)

	for _, acStorage := range txStorage.Storages {
		ac := &action{}
		ac.fromStorage(ts, acStorage)

		typStr, acStr, outStr := ac.toJsonStr()
		if acStr == nil {
			err = errors.New("tracer internal failure")
			return nil, err
		}
		traceStr, _ := json.Marshal(acStorage.TraceAddress)
		bodyPiece := fmt.Sprintf(
			`,"subtraces":%d,"traceAddress":%s,"type":"%s","action":%s`,
			acStorage.Subtraces, string(traceStr), typStr, *acStr,
		)
		var resultPiece string
		if ac.err != nil {
			resultPiece = fmt.Sprintf(`,"error":"Reverted","revert":"0x%x"`, ac.revert)
		} else if outStr != nil {
			resultPiece = fmt.Sprintf(`,"result":%s`, *outStr)
		} else {
			resultPiece = `,"result":null`
		}
		jstr := "{" + headPiece + bodyPiece + resultPiece + "}"
		results = append(results, json.RawMessage(jstr))
	}
	return results, nil
}

// return trace result of a block
func (ts *TraceBlockStorage) ToJson() (json.RawMessage, error) {
	var results []json.RawMessage
	for i := range ts.TraceStorages {
		tx, err := ts.TxJson(i)
		if err != nil {
			return nil, err
		}
		results = append(results, tx...)
	}
	return json.Marshal(results)
}

type ActionStorage struct {
	Subtraces    uint
	TraceAddress []uint
	TraceData    []byte
}

func (storage *ActionStorage) appendByte(byt byte) {
	storage.TraceData = append(storage.TraceData, byt)
}

func (storage *ActionStorage) appendFixed(data []byte) {
	storage.TraceData = append(storage.TraceData, data...)
}
func (storage *ActionStorage) appendNumber(num *big.Int) {
	bytes, _ := rlp.EncodeToBytes(num)
	storage.appendByte(uint8(len(bytes)))
	storage.appendFixed(bytes)
}

func (storage *ActionStorage) readByte() byte {
	val := storage.TraceData[0]
	storage.TraceData = storage.TraceData[1:]
	return val
}
func (storage *ActionStorage) readFixedData(size uint) []byte {
	fixedData := storage.TraceData[:size]
	storage.TraceData = storage.TraceData[size:]
	return fixedData
}
func (storage *ActionStorage) readNumber() *big.Int {
	size := storage.readByte()
	bytes := storage.readFixedData(uint(size))
	var num big.Int
	rlp.DecodeBytes(bytes, &num)
	return &num
}

type TxStorage struct {
	Hash     common.Hash
	Storages []*ActionStorage
}
