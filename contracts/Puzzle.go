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

// PuzzleABI is the input ABI used to generate the binding from.
const PuzzleABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"}],\"name\":\"setLevel\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPlayers\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"},{\"name\":\"session\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"string\"}],\"name\":\"payout\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"play\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"}],\"name\":\"resetPlayer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reset\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"players\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

// PuzzleBin is the compiled bytecode used for deploying new contracts.
const PuzzleBin = `0x608060405234801561001057600080fd5b50600780546001600160a01b0319163317905561096e806100326000396000f3fe60806040526004361061007b5760003560e01c806393e84cd91161004e57806393e84cd914610213578063c95e09091461022d578063d826f88f14610260578063f71d96cb146102755761007b565b8063481c6a7514610080578063722dcd8f146100b15780638b5b9ccc146100ec5780638f3a7d8b14610151575b600080fd5b34801561008c57600080fd5b5061009561029f565b604080516001600160a01b039092168252519081900360200190f35b3480156100bd57600080fd5b506100ea600480360360408110156100d457600080fd5b506001600160a01b0381351690602001356102ae565b005b3480156100f857600080fd5b506101016102ca565b60408051602080825283518183015283519192839290830191858101910280838360005b8381101561013d578181015183820152602001610125565b505050509050019250505060405180910390f35b6100ea6004803603608081101561016757600080fd5b6001600160a01b03823516916020810135916040820135919081019060808101606082013564010000000081111561019e57600080fd5b8201836020820111156101b057600080fd5b803590602001918460018302840111640100000000831117156101d257600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955061032d945050505050565b61021b610592565b60408051918252519081900360200190f35b34801561023957600080fd5b506100ea6004803603602081101561025057600080fd5b50356001600160a01b03166106a0565b34801561026c57600080fd5b506100ea61075c565b34801561028157600080fd5b506100956004803603602081101561029857600080fd5b503561083f565b6007546001600160a01b031681565b6001600160a01b03909116600090815260046020526040902055565b6060600880548060200260200160405190810160405280929190818152602001828054801561032257602002820191906000526020600020905b81546001600160a01b03168152600190910190602001808311610304575b505050505090505b90565b6007546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b031633146103f457604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b838110156103b95781810151838201526020016103a1565b50505050905090810190601f1680156103e65780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b508160056000866001600160a01b03166001600160a01b0316815260200190815260200160002054146040518060600160405280602c8152602001610917602c91399061048557604051600160e51b62461bcd0281526020600482018181528351602484015283519092839260449091019190850190808383600083156103b95781810151838201526020016103a1565b5060046000856001600160a01b03166001600160a01b031681526020019081526020016000205483116040518060600160405280602a81526020016108ed602a91399061051657604051600160e51b62461bcd0281526020600482018181528351602484015283519092839260449091019190850190808383600083156103b95781810151838201526020016103a1565b506001600160a01b038416600090815260046020526040902054830360025561053f84846102ae565b6001600160a01b0384166000818152600660205260408082205460025460049091040280835590516108fc82150292818181858888f1935050505015801561058b573d6000803e3d6000fd5b5050505050565b6000671bc16d674ec800003410156040518060400160405280601181526020017f496e73756666696369656e742046756e640000000000000000000000000000008152509061062557604051600160e51b62461bcd0281526020600482018181528351602484015283519092839260449091019190850190808383600083156103b95781810151838201526020016103a1565b5061062e610866565b6001908155336000818152600460209081526040808320839055845460058352818420556006909152812034905560088054808501825591527ff3f7a9fe364faab93b216da50a3214154f22a0a2b415b23a84c8169e8b636ee30180546001600160a01b031916909117905554905090565b6007546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b0316331461072a57604051600160e51b62461bcd0281526020600482018181528351602484015283519092839260449091019190850190808383600083156103b95781810151838201526020016103a1565b506001600160a01b03166000908152600460209081526040808320839055600582528083208390556006909152812055565b6007546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b031633146107e657604051600160e51b62461bcd0281526020600482018181528351602484015283519092839260449091019190850190808383600083156103b95781810151838201526020016103a1565b5060085460005b8181101561082d5760006008828154811061080457fe5b6000918252602090912001546001600160a01b03169050610824816106a0565b506001016107ed565b50600061083b6008826108a5565b5050565b6008818154811061084c57fe5b6000918252602090912001546001600160a01b0316905081565b604080514260208083019190915233606090811b8385015230901b60548301528251604881840301815260689092019092528051910120600181905590565b8154818355818111156108c9576000838152602090206108c99181019083016108ce565b505050565b61032a91905b808211156108e857600081556001016108d4565b509056fe506c617965722072657175657374696e67207061796f757420666f72206561726c696572206c6576656c506c617965722072657175657374696e67207061796f757420666f7220756e6b6e6f776e2073657373696f6ea165627a7a72305820249ad70e884ad82ab0ef801dd3ad421330f63fe8fc704dcf1e595db8893d86090029`

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

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[])
func (_Puzzle *PuzzleCaller) GetPlayers(opts *bind.CallOpts) ([]common.Address, error) {
	var (
		ret0 = new([]common.Address)
	)
	out := ret0
	err := _Puzzle.contract.Call(opts, out, "getPlayers")
	return *ret0, err
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[])
func (_Puzzle *PuzzleSession) GetPlayers() ([]common.Address, error) {
	return _Puzzle.Contract.GetPlayers(&_Puzzle.CallOpts)
}

// GetPlayers is a free data retrieval call binding the contract method 0x8b5b9ccc.
//
// Solidity: function getPlayers() constant returns(address[])
func (_Puzzle *PuzzleCallerSession) GetPlayers() ([]common.Address, error) {
	return _Puzzle.Contract.GetPlayers(&_Puzzle.CallOpts)
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

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Puzzle *PuzzleCaller) Players(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _Puzzle.contract.Call(opts, out, "players", arg0)
	return *ret0, err
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Puzzle *PuzzleSession) Players(arg0 *big.Int) (common.Address, error) {
	return _Puzzle.Contract.Players(&_Puzzle.CallOpts, arg0)
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players( uint256) constant returns(address)
func (_Puzzle *PuzzleCallerSession) Players(arg0 *big.Int) (common.Address, error) {
	return _Puzzle.Contract.Players(&_Puzzle.CallOpts, arg0)
}

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(player address, level uint256, session uint256,  string) returns()
func (_Puzzle *PuzzleTransactor) Payout(opts *bind.TransactOpts, player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "payout", player, level, session, arg3)
}

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(player address, level uint256, session uint256,  string) returns()
func (_Puzzle *PuzzleSession) Payout(player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, session, arg3)
}

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(player address, level uint256, session uint256,  string) returns()
func (_Puzzle *PuzzleTransactorSession) Payout(player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, session, arg3)
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns(uint256)
func (_Puzzle *PuzzleTransactor) Play(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "play")
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns(uint256)
func (_Puzzle *PuzzleSession) Play() (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts)
}

// Play is a paid mutator transaction binding the contract method 0x93e84cd9.
//
// Solidity: function play() returns(uint256)
func (_Puzzle *PuzzleTransactorSession) Play() (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts)
}

// Reset is a paid mutator transaction binding the contract method 0xd826f88f.
//
// Solidity: function reset() returns()
func (_Puzzle *PuzzleTransactor) Reset(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "reset")
}

// Reset is a paid mutator transaction binding the contract method 0xd826f88f.
//
// Solidity: function reset() returns()
func (_Puzzle *PuzzleSession) Reset() (*types.Transaction, error) {
	return _Puzzle.Contract.Reset(&_Puzzle.TransactOpts)
}

// Reset is a paid mutator transaction binding the contract method 0xd826f88f.
//
// Solidity: function reset() returns()
func (_Puzzle *PuzzleTransactorSession) Reset() (*types.Transaction, error) {
	return _Puzzle.Contract.Reset(&_Puzzle.TransactOpts)
}

// ResetPlayer is a paid mutator transaction binding the contract method 0xc95e0909.
//
// Solidity: function resetPlayer(player address) returns()
func (_Puzzle *PuzzleTransactor) ResetPlayer(opts *bind.TransactOpts, player common.Address) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "resetPlayer", player)
}

// ResetPlayer is a paid mutator transaction binding the contract method 0xc95e0909.
//
// Solidity: function resetPlayer(player address) returns()
func (_Puzzle *PuzzleSession) ResetPlayer(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.ResetPlayer(&_Puzzle.TransactOpts, player)
}

// ResetPlayer is a paid mutator transaction binding the contract method 0xc95e0909.
//
// Solidity: function resetPlayer(player address) returns()
func (_Puzzle *PuzzleTransactorSession) ResetPlayer(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.ResetPlayer(&_Puzzle.TransactOpts, player)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(player address, level uint256) returns()
func (_Puzzle *PuzzleTransactor) SetLevel(opts *bind.TransactOpts, player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "setLevel", player, level)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(player address, level uint256) returns()
func (_Puzzle *PuzzleSession) SetLevel(player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.SetLevel(&_Puzzle.TransactOpts, player, level)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(player address, level uint256) returns()
func (_Puzzle *PuzzleTransactorSession) SetLevel(player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.SetLevel(&_Puzzle.TransactOpts, player, level)
}
