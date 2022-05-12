// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package cxmessagereceiver

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = abi.U256
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// CXMessageReceiverABI is the input ABI used to generate the binding from.
const CXMessageReceiverABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"fromAddr\",\"type\":\"address\"},{\"internalType\":\"shardId\",\"name\":\"fromShard\",\"type\":\"uint32\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"}],\"name\":\"recvCrossShardMessage\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// CXMessageReceiver is an auto generated Go binding around an Ethereum contract.
type CXMessageReceiver struct {
	CXMessageReceiverCaller     // Read-only binding to the contract
	CXMessageReceiverTransactor // Write-only binding to the contract
	CXMessageReceiverFilterer   // Log filterer for contract events
}

// CXMessageReceiverCaller is an auto generated read-only Go binding around an Ethereum contract.
type CXMessageReceiverCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CXMessageReceiverTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CXMessageReceiverTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CXMessageReceiverFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CXMessageReceiverFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CXMessageReceiverSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CXMessageReceiverSession struct {
	Contract     *CXMessageReceiver // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// CXMessageReceiverCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CXMessageReceiverCallerSession struct {
	Contract *CXMessageReceiverCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// CXMessageReceiverTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CXMessageReceiverTransactorSession struct {
	Contract     *CXMessageReceiverTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// CXMessageReceiverRaw is an auto generated low-level Go binding around an Ethereum contract.
type CXMessageReceiverRaw struct {
	Contract *CXMessageReceiver // Generic contract binding to access the raw methods on
}

// CXMessageReceiverCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CXMessageReceiverCallerRaw struct {
	Contract *CXMessageReceiverCaller // Generic read-only contract binding to access the raw methods on
}

// CXMessageReceiverTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CXMessageReceiverTransactorRaw struct {
	Contract *CXMessageReceiverTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCXMessageReceiver creates a new instance of CXMessageReceiver, bound to a specific deployed contract.
func NewCXMessageReceiver(address common.Address, backend bind.ContractBackend) (*CXMessageReceiver, error) {
	contract, err := bindCXMessageReceiver(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CXMessageReceiver{CXMessageReceiverCaller: CXMessageReceiverCaller{contract: contract}, CXMessageReceiverTransactor: CXMessageReceiverTransactor{contract: contract}, CXMessageReceiverFilterer: CXMessageReceiverFilterer{contract: contract}}, nil
}

// NewCXMessageReceiverCaller creates a new read-only instance of CXMessageReceiver, bound to a specific deployed contract.
func NewCXMessageReceiverCaller(address common.Address, caller bind.ContractCaller) (*CXMessageReceiverCaller, error) {
	contract, err := bindCXMessageReceiver(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CXMessageReceiverCaller{contract: contract}, nil
}

// NewCXMessageReceiverTransactor creates a new write-only instance of CXMessageReceiver, bound to a specific deployed contract.
func NewCXMessageReceiverTransactor(address common.Address, transactor bind.ContractTransactor) (*CXMessageReceiverTransactor, error) {
	contract, err := bindCXMessageReceiver(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CXMessageReceiverTransactor{contract: contract}, nil
}

// NewCXMessageReceiverFilterer creates a new log filterer instance of CXMessageReceiver, bound to a specific deployed contract.
func NewCXMessageReceiverFilterer(address common.Address, filterer bind.ContractFilterer) (*CXMessageReceiverFilterer, error) {
	contract, err := bindCXMessageReceiver(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CXMessageReceiverFilterer{contract: contract}, nil
}

// bindCXMessageReceiver binds a generic wrapper to an already deployed contract.
func bindCXMessageReceiver(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(CXMessageReceiverABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CXMessageReceiver *CXMessageReceiverRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _CXMessageReceiver.Contract.CXMessageReceiverCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CXMessageReceiver *CXMessageReceiverRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.CXMessageReceiverTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CXMessageReceiver *CXMessageReceiverRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.CXMessageReceiverTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CXMessageReceiver *CXMessageReceiverCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _CXMessageReceiver.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CXMessageReceiver *CXMessageReceiverTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CXMessageReceiver *CXMessageReceiverTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.contract.Transact(opts, method, params...)
}

// RecvCrossShardMessage is a paid mutator transaction binding the contract method 0xb9cf3c4f.
//
// Solidity: function recvCrossShardMessage(address fromAddr, uint32 fromShard, bytes payload) returns()
func (_CXMessageReceiver *CXMessageReceiverTransactor) RecvCrossShardMessage(opts *bind.TransactOpts, fromAddr common.Address, fromShard uint32, payload []byte) (*types.Transaction, error) {
	return _CXMessageReceiver.contract.Transact(opts, "recvCrossShardMessage", fromAddr, fromShard, payload)
}

// RecvCrossShardMessage is a paid mutator transaction binding the contract method 0xb9cf3c4f.
//
// Solidity: function recvCrossShardMessage(address fromAddr, uint32 fromShard, bytes payload) returns()
func (_CXMessageReceiver *CXMessageReceiverSession) RecvCrossShardMessage(fromAddr common.Address, fromShard uint32, payload []byte) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.RecvCrossShardMessage(&_CXMessageReceiver.TransactOpts, fromAddr, fromShard, payload)
}

// RecvCrossShardMessage is a paid mutator transaction binding the contract method 0xb9cf3c4f.
//
// Solidity: function recvCrossShardMessage(address fromAddr, uint32 fromShard, bytes payload) returns()
func (_CXMessageReceiver *CXMessageReceiverTransactorSession) RecvCrossShardMessage(fromAddr common.Address, fromShard uint32, payload []byte) (*types.Transaction, error) {
	return _CXMessageReceiver.Contract.RecvCrossShardMessage(&_CXMessageReceiver.TransactOpts, fromAddr, fromShard, payload)
}
