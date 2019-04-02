// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

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

// LotteryABI is the input ABI used to generate the binding from.
const LotteryABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pickWinner\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPlayers\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"enter\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"players\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

// LotteryBin is the compiled bytecode used for deploying new contracts.
const LotteryBin = `0x608060405234801561001057600080fd5b50600080546001600160a01b031916331790556104ec806100326000396000f3fe60806040526004361061004a5760003560e01c8063481c6a751461004f5780635d495aea146100805780638b5b9ccc1461008a578063e97dcb62146100ef578063f71d96cb146100f7575b600080fd5b34801561005b57600080fd5b50610064610121565b604080516001600160a01b039092168252519081900360200190f35b610088610130565b005b34801561009657600080fd5b5061009f61028c565b60408051602080825283518183015283519192839290830191858101910280838360005b838110156100db5781810151838201526020016100c3565b505050509050019250505060405180910390f35b6100886102ef565b34801561010357600080fd5b506100646004803603602081101561011a57600080fd5b50356103a9565b6000546001600160a01b031681565b60005460408051808201909152601381527f4f6e6c79206d616e616765722063616e20646f000000000000000000000000006020820152906001600160a01b031633146101fe57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b838110156101c35781810151838201526020016101ab565b50505050905090810190601f1680156101f05780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b5060015460009061020d6103d1565b81151561021657fe5b06905060018181548110151561022857fe5b60009182526020822001546040516001600160a01b0390911691303180156108fc02929091818181858888f1935050505015801561026a573d6000803e3d6000fd5b5060408051600081526020810191829052516102889160019161040b565b5050565b606060018054806020026020016040519081016040528092919081815260200182805480156102e457602002820191906000526020600020905b81546001600160a01b031681526001909101906020018083116102c6575b505050505090505b90565b67016345785d8a000034116040518060600160405280602c8152602001610495602c913990151561036557604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156101c35781810151838201526020016101ab565b506001805480820182556000919091527fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b03191633179055565b60018054829081106103b757fe5b6000918252602090912001546001600160a01b0316905081565b604080514260208083019190915233606090811b8385015230901b6054830152825160488184030181526068909201909252805191012090565b828054828255906000526020600020908101928215610460579160200282015b8281111561046057825182546001600160a01b0319166001600160a01b0390911617825560209092019160019091019061042b565b5061046c929150610470565b5090565b6102ec91905b8082111561046c5780546001600160a01b031916815560010161047656fe54686520706c61796572206e6565647320746f207374616b65206174206c6561737420302e31206574686572a165627a7a72305820159638f94fdba6b1dc61d4071f6673028ee53a12a39d66f7ec00701aed93a0b60029`

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
// Solidity: function getPlayers() constant returns(address[])
func (_Lottery *LotteryCaller) GetPlayers(opts *bind.CallOpts) ([]common.Address, error) {
	var (
		ret0 = new([]common.Address)
	)
	out := ret0
	err := _Lottery.contract.Call(opts, out, "getPlayers")
	return *ret0, err
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[])
func (_Lottery *LotterySession) GetPlayers() ([]common.Address, error) {
	return _Lottery.Contract.GetPlayers(&_Lottery.CallOpts)
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[])
func (_Lottery *LotteryCallerSession) GetPlayers() ([]common.Address, error) {
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
// Solidity: function players(uint256 ) constant returns(address)
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
// Solidity: function players(uint256 ) constant returns(address)
func (_Lottery *LotterySession) Players(arg0 *big.Int) (common.Address, error) {
	return _Lottery.Contract.Players(&_Lottery.CallOpts, arg0)
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players(uint256 ) constant returns(address)
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
