// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// PuzzleABI is the input ABI used to generate the binding from.
const PuzzleABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"address\"}],\"name\":\"level_map\",\"outputs\":[{\"name\":\"\",\"type\":\"uint8\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"new_level\",\"type\":\"uint8\"}],\"name\":\"payout\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"play\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"}]"

// PuzzleBin is the compiled bytecode used for deploying new contracts.
const PuzzleBin = `0x608060405260008054600160a060020a031916331790556103be806100256000396000f3fe60806040526004361061005b577c0100000000000000000000000000000000000000000000000000000000600035046311b88fef8114610060578063158b4aa6146100a9578063481c6a75146100da57806393e84cd91461010b575b600080fd5b34801561006c57600080fd5b506100936004803603602081101561008357600080fd5b5035600160a060020a0316610113565b6040805160ff9092168252519081900360200190f35b6100d8600480360360408110156100bf57600080fd5b508035600160a060020a0316906020013560ff16610128565b005b3480156100e657600080fd5b506100ef6102da565b60408051600160a060020a039092168252519081900360200190f35b6100d86102e9565b60016020526000908152604090205460ff1681565b60005460408051808201909152601381527f556e617574686f72697a65642041636365737300000000000000000000000000602082015290600160a060020a0316331461020d576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b838110156101d25781810151838201526020016101ba565b50505050905090810190601f1680156101ff5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b50600160a060020a03821660009081526001602052604090205460ff90811690821681116102b457604051600160a060020a0384169067ffffffffffffffff670de0b6b3a764000060ff60026001878903010216021680156108fc02916000818181858888f19350505050158015610289573d6000803e3d6000fd5b50600160a060020a0383166000908152600160205260409020805460ff191660ff84161790556102d5565b600160a060020a0383166000908152600160205260409020805460ff191690555b505050565b600054600160a060020a031681565b60408051808201909152601181527f496e73756666696369656e742046756e640000000000000000000000000000006020820152671bc16d674ec8000034101561038f576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382528381815181526020019150805190602001908083836000838110156101d25781810151838201526020016101ba565b5056fea165627a7a72305820dcdfb1bf37c0dddda1b2ec670fa6ef6c8913a8b4b6e815bf184697e5da7b0d1c0029`

// DeployPuzzle deploys a new Ethereum contract, binding an instance of Puzzle to it.
func DeployPuzzle(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Puzzle, error) {
	parsed, err := abi.JSON(strings.NewReader(PuzzleABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(PuzzleBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Puzzle{PuzzleCaller: PuzzleCaller{contract: contract}, PuzzleTransactor: PuzzleTransactor{contract: contract}, PuzzleFilterer: PuzzleFilterer{contract: contract}}, nil
}

// Puzzle is an auto generated Go binding around an Ethereum contract.
type Puzzle struct {
	PuzzleCaller     // Read-only binding to the contract
	PuzzleTransactor // Write-only binding to the contract
	PuzzleFilterer   // Log filterer for contract events
}

// PuzzleCaller is an auto generated read-only Go binding around an Ethereum contract.
type PuzzleCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PuzzleTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PuzzleTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PuzzleFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PuzzleFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PuzzleSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PuzzleSession struct {
	Contract     *Puzzle           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PuzzleCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PuzzleCallerSession struct {
	Contract *PuzzleCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// PuzzleTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PuzzleTransactorSession struct {
	Contract     *PuzzleTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PuzzleRaw is an auto generated low-level Go binding around an Ethereum contract.
type PuzzleRaw struct {
	Contract *Puzzle // Generic contract binding to access the raw methods on
}

// PuzzleCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PuzzleCallerRaw struct {
	Contract *PuzzleCaller // Generic read-only contract binding to access the raw methods on
}

// PuzzleTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PuzzleTransactorRaw struct {
	Contract *PuzzleTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPuzzle creates a new instance of Puzzle, bound to a specific deployed contract.
func NewPuzzle(address common.Address, backend bind.ContractBackend) (*Puzzle, error) {
	contract, err := bindPuzzle(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Puzzle{PuzzleCaller: PuzzleCaller{contract: contract}, PuzzleTransactor: PuzzleTransactor{contract: contract}, PuzzleFilterer: PuzzleFilterer{contract: contract}}, nil
}

// NewPuzzleCaller creates a new read-only instance of Puzzle, bound to a specific deployed contract.
func NewPuzzleCaller(address common.Address, caller bind.ContractCaller) (*PuzzleCaller, error) {
	contract, err := bindPuzzle(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PuzzleCaller{contract: contract}, nil
}

// NewPuzzleTransactor creates a new write-only instance of Puzzle, bound to a specific deployed contract.
func NewPuzzleTransactor(address common.Address, transactor bind.ContractTransactor) (*PuzzleTransactor, error) {
	contract, err := bindPuzzle(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PuzzleTransactor{contract: contract}, nil
}

// NewPuzzleFilterer creates a new log filterer instance of Puzzle, bound to a specific deployed contract.
func NewPuzzleFilterer(address common.Address, filterer bind.ContractFilterer) (*PuzzleFilterer, error) {
	contract, err := bindPuzzle(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PuzzleFilterer{contract: contract}, nil
}

// bindPuzzle binds a generic wrapper to an already deployed contract.
func bindPuzzle(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(PuzzleABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Puzzle *PuzzleRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Puzzle.Contract.PuzzleCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Puzzle *PuzzleRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Puzzle.Contract.PuzzleTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Puzzle *PuzzleRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Puzzle.Contract.PuzzleTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Puzzle *PuzzleCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Puzzle.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Puzzle *PuzzleTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Puzzle.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Puzzle *PuzzleTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Puzzle.Contract.contract.Transact(opts, method, params...)
}

// LevelMap is a free data retrieval call binding the contract method 0x11b88fef.
//
// Solidity: function level_map( address) constant returns(uint8)
func (_Puzzle *PuzzleCaller) LevelMap(opts *bind.CallOpts, arg0 common.Address) (uint8, error) {
	var (
		ret0 = new(uint8)
	)
	out := ret0
	err := _Puzzle.contract.Call(opts, out, "level_map", arg0)
	return *ret0, err
}

// LevelMap is a free data retrieval call binding the contract method 0x11b88fef.
//
// Solidity: function level_map( address) constant returns(uint8)
func (_Puzzle *PuzzleSession) LevelMap(arg0 common.Address) (uint8, error) {
	return _Puzzle.Contract.LevelMap(&_Puzzle.CallOpts, arg0)
}

// LevelMap is a free data retrieval call binding the contract method 0x11b88fef.
//
// Solidity: function level_map( address) constant returns(uint8)
func (_Puzzle *PuzzleCallerSession) LevelMap(arg0 common.Address) (uint8, error) {
	return _Puzzle.Contract.LevelMap(&_Puzzle.CallOpts, arg0)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Puzzle *PuzzleCaller) Manager(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Puzzle.contract.Call(opts, out, "manager")
	return *ret0, err
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Puzzle *PuzzleSession) Manager() (common.Address, error) {
	return _Puzzle.Contract.Manager(&_Puzzle.CallOpts)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Puzzle *PuzzleCallerSession) Manager() (common.Address, error) {
	return _Puzzle.Contract.Manager(&_Puzzle.CallOpts)
}

// Payout is a paid mutator transaction binding the contract method 0x158b4aa6.
//
// Solidity: function payout(player address, new_level uint8) returns()
func (_Puzzle *PuzzleTransactor) Payout(opts *bind.TransactOpts, player common.Address, new_level uint8) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "payout", player, new_level)
}

// Payout is a paid mutator transaction binding the contract method 0x158b4aa6.
//
// Solidity: function payout(player address, new_level uint8) returns()
func (_Puzzle *PuzzleSession) Payout(player common.Address, new_level uint8) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, new_level)
}

// Payout is a paid mutator transaction binding the contract method 0x158b4aa6.
//
// Solidity: function payout(player address, new_level uint8) returns()
func (_Puzzle *PuzzleTransactorSession) Payout(player common.Address, new_level uint8) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, new_level)
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns()
func (_Puzzle *PuzzleTransactor) Play(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "play")
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns()
func (_Puzzle *PuzzleSession) Play() (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts)
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns()
func (_Puzzle *PuzzleTransactorSession) Play() (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts)
}
