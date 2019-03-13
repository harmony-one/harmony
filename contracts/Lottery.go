// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

// LotteryABI is the input ABI used to generate the binding from.
const LotteryABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pickWinner\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPlayers\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"},{\"name\":\"balances\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"enter\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"players\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

// LotteryBin is the compiled bytecode used for deploying new contracts.
const LotteryBin = `0x608060405234801561001057600080fd5b5060008054600160a060020a0319163317905561063f806100326000396000f3fe608060405260043610610066577c01000000000000000000000000000000000000000000000000000000006000350463481c6a75811461006b5780635d495aea1461009c5780638b5b9ccc146100a6578063e97dcb6214610154578063f71d96cb1461015c575b600080fd5b34801561007757600080fd5b50610080610186565b60408051600160a060020a039092168252519081900360200190f35b6100a4610195565b005b3480156100b257600080fd5b506100bb610308565b604051808060200180602001838103835285818151815260200191508051906020019060200280838360005b838110156100ff5781810151838201526020016100e7565b50505050905001838103825284818151815260200191508051906020019060200280838360005b8381101561013e578181015183820152602001610126565b5050505090500194505050505060405180910390f35b6100a46103f9565b34801561016857600080fd5b506100806004803603602081101561017f57600080fd5b50356104d7565b600054600160a060020a031681565b60005460408051808201909152601381527f4f6e6c79206d616e616765722063616e20646f00000000000000000000000000602082015290600160a060020a0316331461027a576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561023f578181015183820152602001610227565b50505050905090810190601f16801561026c5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b506001546000906102896104ff565b81151561029257fe5b0690506001818154811015156102a457fe5b6000918252602082200154604051600160a060020a0390911691303180156108fc02929091818181858888f193505050501580156102e6573d6000803e3d6000fd5b50604080516000815260208101918290525161030491600191610544565b5050565b60608060018054905060405190808252806020026020018201604052801561033a578160200160208202803883390190505b50905060005b60015481101561039157600180548290811061035857fe5b6000918252602090912001548251600160a060020a03909116319083908390811061037f57fe5b60209081029091010152600101610340565b50600181818054806020026020016040519081016040528092919081815260200182805480156103ea57602002820191906000526020600020905b8154600160a060020a031681526001909101906020018083116103cc575b50505050509150915091509091565b662386f26fc100003411606060405190810160405280602c81526020016105e8602c9139901515610486576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360008381101561023f578181015183820152602001610227565b506001805480820182556000919091527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf601805473ffffffffffffffffffffffffffffffffffffffff191633179055565b60018054829081106104e557fe5b600091825260209091200154600160a060020a0316905081565b60408051426020808301919091526c01000000000000000000000000338102838501523002605483015282516048818403018152606890920190925280519101205b90565b8280548282559060005260206000209081019282156105a6579160200282015b828111156105a6578251825473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a03909116178255602090920191600190910190610564565b506105b29291506105b6565b5090565b61054191905b808211156105b257805473ffffffffffffffffffffffffffffffffffffffff191681556001016105bc56fe54686520706c61796572206e6565647320746f207374616b65206174206c6561737420302e31206574686572a165627a7a723058200241259e41b5c525fe88fed1715ddf6a97c87e7d98a5b7f65bcf9dfd26143e770029`

// DeployLottery deploys a new Ethereum contract, binding an instance of Lottery to it.
func DeployLottery(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Lottery, error) {
	parsed, err := abi.JSON(strings.NewReader(LotteryABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(LotteryBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Lottery{LotteryCaller: LotteryCaller{contract: contract}, LotteryTransactor: LotteryTransactor{contract: contract}, LotteryFilterer: LotteryFilterer{contract: contract}}, nil
}

// Lottery is an auto generated Go binding around an Ethereum contract.
type Lottery struct {
	LotteryCaller     // Read-only binding to the contract
	LotteryTransactor // Write-only binding to the contract
	LotteryFilterer   // Log filterer for contract events
}

// LotteryCaller is an auto generated read-only Go binding around an Ethereum contract.
type LotteryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LotteryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LotteryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LotteryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LotteryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LotterySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LotterySession struct {
	Contract     *Lottery          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LotteryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LotteryCallerSession struct {
	Contract *LotteryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// LotteryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LotteryTransactorSession struct {
	Contract     *LotteryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// LotteryRaw is an auto generated low-level Go binding around an Ethereum contract.
type LotteryRaw struct {
	Contract *Lottery // Generic contract binding to access the raw methods on
}

// LotteryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LotteryCallerRaw struct {
	Contract *LotteryCaller // Generic read-only contract binding to access the raw methods on
}

// LotteryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LotteryTransactorRaw struct {
	Contract *LotteryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLottery creates a new instance of Lottery, bound to a specific deployed contract.
func NewLottery(address common.Address, backend bind.ContractBackend) (*Lottery, error) {
	contract, err := bindLottery(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Lottery{LotteryCaller: LotteryCaller{contract: contract}, LotteryTransactor: LotteryTransactor{contract: contract}, LotteryFilterer: LotteryFilterer{contract: contract}}, nil
}

// NewLotteryCaller creates a new read-only instance of Lottery, bound to a specific deployed contract.
func NewLotteryCaller(address common.Address, caller bind.ContractCaller) (*LotteryCaller, error) {
	contract, err := bindLottery(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LotteryCaller{contract: contract}, nil
}

// NewLotteryTransactor creates a new write-only instance of Lottery, bound to a specific deployed contract.
func NewLotteryTransactor(address common.Address, transactor bind.ContractTransactor) (*LotteryTransactor, error) {
	contract, err := bindLottery(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LotteryTransactor{contract: contract}, nil
}

// NewLotteryFilterer creates a new log filterer instance of Lottery, bound to a specific deployed contract.
func NewLotteryFilterer(address common.Address, filterer bind.ContractFilterer) (*LotteryFilterer, error) {
	contract, err := bindLottery(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LotteryFilterer{contract: contract}, nil
}

// bindLottery binds a generic wrapper to an already deployed contract.
func bindLottery(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LotteryABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Lottery *LotteryRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Lottery.Contract.LotteryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Lottery *LotteryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Lottery.Contract.LotteryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Lottery *LotteryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Lottery.Contract.LotteryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Lottery *LotteryCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _Lottery.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Lottery *LotteryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Lottery.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Lottery *LotteryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Lottery.Contract.contract.Transact(opts, method, params...)
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[], balances uint256[])
func (_Lottery *LotteryCaller) GetPlayers(opts *bind.CallOpts) ([]common.Address, []*big.Int, error) {
	var (
		ret0 = new([]common.Address)
		ret1 = new([]*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
	}
	err := _Lottery.contract.Call(opts, out, "getPlayers")
	return *ret0, *ret1, err
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[], balances uint256[])
func (_Lottery *LotterySession) GetPlayers() ([]common.Address, []*big.Int, error) {
	return _Lottery.Contract.GetPlayers(&_Lottery.CallOpts)
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[], balances uint256[])
func (_Lottery *LotteryCallerSession) GetPlayers() ([]common.Address, []*big.Int, error) {
	return _Lottery.Contract.GetPlayers(&_Lottery.CallOpts)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Lottery *LotteryCaller) Manager(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Lottery.contract.Call(opts, out, "manager")
	return *ret0, err
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Lottery *LotterySession) Manager() (common.Address, error) {
	return _Lottery.Contract.Manager(&_Lottery.CallOpts)
}

// Manager is a free data retrieval call binding the contract method 0x481c6a75.
//
// Solidity: function manager() constant returns(address)
func (_Lottery *LotteryCallerSession) Manager() (common.Address, error) {
	return _Lottery.Contract.Manager(&_Lottery.CallOpts)
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Lottery *LotteryCaller) Players(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Lottery.contract.Call(opts, out, "players", arg0)
	return *ret0, err
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Lottery *LotterySession) Players(arg0 *big.Int) (common.Address, error) {
	return _Lottery.Contract.Players(&_Lottery.CallOpts, arg0)
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Lottery *LotteryCallerSession) Players(arg0 *big.Int) (common.Address, error) {
	return _Lottery.Contract.Players(&_Lottery.CallOpts, arg0)
}

// Enter is a paid mutator transaction binding the contract method 0xe97dcb62.
//
// Solidity: function enter() returns()
func (_Lottery *LotteryTransactor) Enter(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Lottery.contract.Transact(opts, "enter")
}

// Enter is a paid mutator transaction binding the contract method 0xe97dcb62.
//
// Solidity: function enter() returns()
func (_Lottery *LotterySession) Enter() (*types.Transaction, error) {
	return _Lottery.Contract.Enter(&_Lottery.TransactOpts)
}

// Enter is a paid mutator transaction binding the contract method 0xe97dcb62.
//
// Solidity: function enter() returns()
func (_Lottery *LotteryTransactorSession) Enter() (*types.Transaction, error) {
	return _Lottery.Contract.Enter(&_Lottery.TransactOpts)
}

// PickWinner is a paid mutator transaction binding the contract method 0x5d495aea.
//
// Solidity: function pickWinner() returns()
func (_Lottery *LotteryTransactor) PickWinner(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Lottery.contract.Transact(opts, "pickWinner")
}

// PickWinner is a paid mutator transaction binding the contract method 0x5d495aea.
//
// Solidity: function pickWinner() returns()
func (_Lottery *LotterySession) PickWinner() (*types.Transaction, error) {
	return _Lottery.Contract.PickWinner(&_Lottery.TransactOpts)
}

// PickWinner is a paid mutator transaction binding the contract method 0x5d495aea.
//
// Solidity: function pickWinner() returns()
func (_Lottery *LotteryTransactorSession) PickWinner() (*types.Transaction, error) {
	return _Lottery.Contract.PickWinner(&_Lottery.TransactOpts)
}
