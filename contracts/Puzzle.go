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
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// PuzzleABI is the input ABI used to generate the binding from.
const PuzzleABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"}],\"name\":\"endGame\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"string\"}],\"name\":\"payout\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"}],\"name\":\"setLevel\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPlayers\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"play\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"}],\"name\":\"resetPlayer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reset\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"players\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"constructor\"}]"

// PuzzleBin is the compiled bytecode used for deploying new contracts.
const PuzzleBin = `0x6080604052600480546001600160a01b0319163317905561087a806100256000396000f3fe6080604052600436106100865760003560e01c80638b5b9ccc116100595780638b5b9ccc146101f257806393e84cd914610257578063c95e09091461025f578063d826f88f14610292578063f71d96cb146102a757610086565b80632a035b6c1461008b578063481c6a75146100c057806352bcd7c8146100f1578063722dcd8f146101b9575b600080fd5b34801561009757600080fd5b506100be600480360360208110156100ae57600080fd5b50356001600160a01b03166102d1565b005b3480156100cc57600080fd5b506100d56102f2565b604080516001600160a01b039092168252519081900360200190f35b3480156100fd57600080fd5b506100be6004803603606081101561011457600080fd5b6001600160a01b038235169160208101359181019060608101604082013564010000000081111561014457600080fd5b82018360208201111561015657600080fd5b8035906020019184600183028401116401000000008311171561017857600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610301945050505050565b3480156101c557600080fd5b506100be600480360360408110156101dc57600080fd5b506001600160a01b03813516906020013561045e565b3480156101fe57600080fd5b5061020761047a565b60408051602080825283518183015283519192839290830191858101910280838360005b8381101561024357818101518382015260200161022b565b505050509050019250505060405180910390f35b6100be6104dd565b34801561026b57600080fd5b506100be6004803603602081101561028257600080fd5b50356001600160a01b0316610617565b34801561029e57600080fd5b506100be6106f2565b3480156102b357600080fd5b506100d5600480360360208110156102ca57600080fd5b50356107df565b6001600160a01b03166000908152600260205260409020805460ff19169055565b6004546001600160a01b031681565b6001600160a01b038316600090815260026020908152604091829020548251808401909352601f83527f506c61796572206973206e6f7420696e20616e206163746976652067616d65009183019190915260ff1615156001146103e557604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b838110156103aa578181015183820152602001610392565b50505050905090810190601f1680156103d75780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b506001600160a01b038316600090815260208190526040902054820361040b848461045e565b6001600160a01b03841660008181526001602052604080822054905160049091048402929183156108fc02918491818181858888f19350505050158015610456573d6000803e3d6000fd5b505050505050565b6001600160a01b03909116600090815260208190526040902055565b606060058054806020026020016040519081016040528092919081815260200182805480156104d257602002820191906000526020600020905b81546001600160a01b031681526001909101906020018083116104b4575b505050505090505b90565b60408051808201909152601181527f496e73756666696369656e742046756e6400000000000000000000000000000060208201526801158e460913d0000034101561056d57604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156103aa578181015183820152602001610392565b503360009081526003602052604090205460ff1615156105e457336000818152600360205260408120805460ff191660019081179091556005805491820181559091527f036b6384b5eca791c62761152d0c79bb0604c104a5fb6f4eb0703f3154bb3db00180546001600160a01b03191690911790555b3360009081526020818152604080832083905560018083528184203490556002909252909120805460ff19169091179055565b60045460408051808201909152601381527f556e617574686f72697a656420416363657373000000000000000000000000006020820152906001600160a01b031633146106a957604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156103aa578181015183820152602001610392565b506001600160a01b031660009081526020818152604080832083905560028252808320805460ff1990811690915560018352818420849055600390925290912080549091169055565b60045460408051808201909152601381527f556e617574686f72697a656420416363657373000000000000000000000000006020820152906001600160a01b0316331461078457604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156103aa578181015183820152602001610392565b5060055460005b818110156107cd5760006005828154811015156107a457fe5b6000918252602090912001546001600160a01b031690506107c481610617565b5060010161078b565b5060006107db600582610807565b5050565b60058054829081106107ed57fe5b6000918252602090912001546001600160a01b0316905081565b81548183558181111561082b5760008381526020902061082b918101908301610830565b505050565b6104da91905b8082111561084a5760008155600101610836565b509056fea165627a7a723058207e349c9ba80ecd1828e602dc6e700265bb3c6e79db4bca293419023b65fee3f20029`

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
func (_Puzzle *PuzzleRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
func (_Puzzle *PuzzleCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
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
	var results []interface{}
	err := _Puzzle.contract.Call(opts, &results, "getPlayers")
	if err != nil {
		return *new([]common.Address), err
	}
	out := *abi.ConvertType(results, new([]common.Address)).(*[]common.Address)
	return out, err
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
	var results []interface{}
	err := _Puzzle.contract.Call(opts, &results, "manager")
	if err != nil {
		return *new(common.Address), err
	}
	out := *abi.ConvertType(results[0], new([]common.Address)).(*[]common.Address)
	return out[0], err
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
// Solidity: function players(uint256 ) constant returns(address)
func (_Puzzle *PuzzleCaller) Players(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var results []interface{}
	err := _Puzzle.contract.Call(opts, &results, "players", arg0)
	if err != nil {
		return *new(common.Address), err
	}
	out := *abi.ConvertType(results[0], new([]common.Address)).(*[]common.Address)
	return out[0], err
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players(uint256 ) constant returns(address)
func (_Puzzle *PuzzleSession) Players(arg0 *big.Int) (common.Address, error) {
	return _Puzzle.Contract.Players(&_Puzzle.CallOpts, arg0)
}

// Players is a free data retrieval call binding the contract method 0xf71d96cb.
//
// Solidity: function players(uint256 ) constant returns(address)
func (_Puzzle *PuzzleCallerSession) Players(arg0 *big.Int) (common.Address, error) {
	return _Puzzle.Contract.Players(&_Puzzle.CallOpts, arg0)
}

// EndGame is a paid mutator transaction binding the contract method 0x2a035b6c.
//
// Solidity: function endGame(address player) returns()
func (_Puzzle *PuzzleTransactor) EndGame(opts *bind.TransactOpts, player common.Address) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "endGame", player)
}

// EndGame is a paid mutator transaction binding the contract method 0x2a035b6c.
//
// Solidity: function endGame(address player) returns()
func (_Puzzle *PuzzleSession) EndGame(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.EndGame(&_Puzzle.TransactOpts, player)
}

// EndGame is a paid mutator transaction binding the contract method 0x2a035b6c.
//
// Solidity: function endGame(address player) returns()
func (_Puzzle *PuzzleTransactorSession) EndGame(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.EndGame(&_Puzzle.TransactOpts, player)
}

// Payout is a paid mutator transaction binding the contract method 0x52bcd7c8.
//
// Solidity: function payout(address player, uint256 level, string ) returns()
func (_Puzzle *PuzzleTransactor) Payout(opts *bind.TransactOpts, player common.Address, level *big.Int, arg2 string) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "payout", player, level, arg2)
}

// Payout is a paid mutator transaction binding the contract method 0x52bcd7c8.
//
// Solidity: function payout(address player, uint256 level, string ) returns()
func (_Puzzle *PuzzleSession) Payout(player common.Address, level *big.Int, arg2 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, arg2)
}

// Payout is a paid mutator transaction binding the contract method 0x52bcd7c8.
//
// Solidity: function payout(address player, uint256 level, string ) returns()
func (_Puzzle *PuzzleTransactorSession) Payout(player common.Address, level *big.Int, arg2 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, arg2)
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
// Solidity: function resetPlayer(address player) returns()
func (_Puzzle *PuzzleTransactor) ResetPlayer(opts *bind.TransactOpts, player common.Address) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "resetPlayer", player)
}

// ResetPlayer is a paid mutator transaction binding the contract method 0xc95e0909.
//
// Solidity: function resetPlayer(address player) returns()
func (_Puzzle *PuzzleSession) ResetPlayer(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.ResetPlayer(&_Puzzle.TransactOpts, player)
}

// ResetPlayer is a paid mutator transaction binding the contract method 0xc95e0909.
//
// Solidity: function resetPlayer(address player) returns()
func (_Puzzle *PuzzleTransactorSession) ResetPlayer(player common.Address) (*types.Transaction, error) {
	return _Puzzle.Contract.ResetPlayer(&_Puzzle.TransactOpts, player)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(address player, uint256 level) returns()
func (_Puzzle *PuzzleTransactor) SetLevel(opts *bind.TransactOpts, player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "setLevel", player, level)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(address player, uint256 level) returns()
func (_Puzzle *PuzzleSession) SetLevel(player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.SetLevel(&_Puzzle.TransactOpts, player, level)
}

// SetLevel is a paid mutator transaction binding the contract method 0x722dcd8f.
//
// Solidity: function setLevel(address player, uint256 level) returns()
func (_Puzzle *PuzzleTransactorSession) SetLevel(player common.Address, level *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.SetLevel(&_Puzzle.TransactOpts, player, level)
}
