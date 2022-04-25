// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package router

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

// RouterABI is the input ABI used to generate the binding from.
const RouterABI = "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"msgAddr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"gasLimit\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"gasPrice\",\"type\":\"uint256\"}],\"name\":\"retrySend\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"to_\",\"type\":\"address\"},{\"internalType\":\"shardId\",\"name\":\"toShard\",\"type\":\"uint32\"},{\"internalType\":\"bytes\",\"name\":\"payload\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"gasBudget\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"gasPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"gasLimit\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"gasLeftoverTo\",\"type\":\"address\"}],\"name\":\"send\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"msgAddr\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]"

// Router is an auto generated Go binding around an Ethereum contract.
type Router struct {
	RouterCaller     // Read-only binding to the contract
	RouterTransactor // Write-only binding to the contract
	RouterFilterer   // Log filterer for contract events
}

// RouterCaller is an auto generated read-only Go binding around an Ethereum contract.
type RouterCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RouterTransactor is an auto generated write-only Go binding around an Ethereum contract.
type RouterTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RouterFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type RouterFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// RouterSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type RouterSession struct {
	Contract     *Router           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RouterCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type RouterCallerSession struct {
	Contract *RouterCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// RouterTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type RouterTransactorSession struct {
	Contract     *RouterTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// RouterRaw is an auto generated low-level Go binding around an Ethereum contract.
type RouterRaw struct {
	Contract *Router // Generic contract binding to access the raw methods on
}

// RouterCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type RouterCallerRaw struct {
	Contract *RouterCaller // Generic read-only contract binding to access the raw methods on
}

// RouterTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type RouterTransactorRaw struct {
	Contract *RouterTransactor // Generic write-only contract binding to access the raw methods on
}

// NewRouter creates a new instance of Router, bound to a specific deployed contract.
func NewRouter(address common.Address, backend bind.ContractBackend) (*Router, error) {
	contract, err := bindRouter(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Router{RouterCaller: RouterCaller{contract: contract}, RouterTransactor: RouterTransactor{contract: contract}, RouterFilterer: RouterFilterer{contract: contract}}, nil
}

// NewRouterCaller creates a new read-only instance of Router, bound to a specific deployed contract.
func NewRouterCaller(address common.Address, caller bind.ContractCaller) (*RouterCaller, error) {
	contract, err := bindRouter(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &RouterCaller{contract: contract}, nil
}

// NewRouterTransactor creates a new write-only instance of Router, bound to a specific deployed contract.
func NewRouterTransactor(address common.Address, transactor bind.ContractTransactor) (*RouterTransactor, error) {
	contract, err := bindRouter(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &RouterTransactor{contract: contract}, nil
}

// NewRouterFilterer creates a new log filterer instance of Router, bound to a specific deployed contract.
func NewRouterFilterer(address common.Address, filterer bind.ContractFilterer) (*RouterFilterer, error) {
	contract, err := bindRouter(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &RouterFilterer{contract: contract}, nil
}

// bindRouter binds a generic wrapper to an already deployed contract.
func bindRouter(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(RouterABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Router *RouterRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Router.Contract.RouterCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Router *RouterRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Router.Contract.RouterTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Router *RouterRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Router.Contract.RouterTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Router *RouterCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Router.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Router *RouterTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Router.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Router *RouterTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Router.Contract.contract.Transact(opts, method, params...)
}

// RetrySend is a paid mutator transaction binding the contract method 0x0db24a7d.
//
// Solidity: function retrySend(address msgAddr, uint256 gasLimit, uint256 gasPrice) returns()
func (_Router *RouterTransactor) RetrySend(opts *bind.TransactOpts, msgAddr common.Address, gasLimit *big.Int, gasPrice *big.Int) (*types.Transaction, error) {
	return _Router.contract.Transact(opts, "retrySend", msgAddr, gasLimit, gasPrice)
}

// RetrySend is a paid mutator transaction binding the contract method 0x0db24a7d.
//
// Solidity: function retrySend(address msgAddr, uint256 gasLimit, uint256 gasPrice) returns()
func (_Router *RouterSession) RetrySend(msgAddr common.Address, gasLimit *big.Int, gasPrice *big.Int) (*types.Transaction, error) {
	return _Router.Contract.RetrySend(&_Router.TransactOpts, msgAddr, gasLimit, gasPrice)
}

// RetrySend is a paid mutator transaction binding the contract method 0x0db24a7d.
//
// Solidity: function retrySend(address msgAddr, uint256 gasLimit, uint256 gasPrice) returns()
func (_Router *RouterTransactorSession) RetrySend(msgAddr common.Address, gasLimit *big.Int, gasPrice *big.Int) (*types.Transaction, error) {
	return _Router.Contract.RetrySend(&_Router.TransactOpts, msgAddr, gasLimit, gasPrice)
}

// Send is a paid mutator transaction binding the contract method 0x3ba2ea6b.
//
// Solidity: function send(address to_, uint32 toShard, bytes payload, uint256 gasBudget, uint256 gasPrice, uint256 gasLimit, address gasLeftoverTo) returns(address msgAddr)
func (_Router *RouterTransactor) Send(opts *bind.TransactOpts, to_ common.Address, toShard uint32, payload []byte, gasBudget *big.Int, gasPrice *big.Int, gasLimit *big.Int, gasLeftoverTo common.Address) (*types.Transaction, error) {
	return _Router.contract.Transact(opts, "send", to_, toShard, payload, gasBudget, gasPrice, gasLimit, gasLeftoverTo)
}

// Send is a paid mutator transaction binding the contract method 0x3ba2ea6b.
//
// Solidity: function send(address to_, uint32 toShard, bytes payload, uint256 gasBudget, uint256 gasPrice, uint256 gasLimit, address gasLeftoverTo) returns(address msgAddr)
func (_Router *RouterSession) Send(to_ common.Address, toShard uint32, payload []byte, gasBudget *big.Int, gasPrice *big.Int, gasLimit *big.Int, gasLeftoverTo common.Address) (*types.Transaction, error) {
	return _Router.Contract.Send(&_Router.TransactOpts, to_, toShard, payload, gasBudget, gasPrice, gasLimit, gasLeftoverTo)
}

// Send is a paid mutator transaction binding the contract method 0x3ba2ea6b.
//
// Solidity: function send(address to_, uint32 toShard, bytes payload, uint256 gasBudget, uint256 gasPrice, uint256 gasLimit, address gasLeftoverTo) returns(address msgAddr)
func (_Router *RouterTransactorSession) Send(to_ common.Address, toShard uint32, payload []byte, gasBudget *big.Int, gasPrice *big.Int, gasLimit *big.Int, gasLeftoverTo common.Address) (*types.Transaction, error) {
	return _Router.Contract.Send(&_Router.TransactOpts, to_, toShard, payload, gasBudget, gasPrice, gasLimit, gasLeftoverTo)
}
