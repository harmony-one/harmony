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
const PuzzleABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"manager\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"sessionID\",\"type\":\"uint256\"}],\"name\":\"play\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"}],\"name\":\"setLevel\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"getPlayers\",\"outputs\":[{\"name\":\"\",\"type\":\"address[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"},{\"name\":\"level\",\"type\":\"uint256\"},{\"name\":\"session\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"string\"}],\"name\":\"payout\",\"outputs\":[],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"player\",\"type\":\"address\"}],\"name\":\"resetPlayer\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"reset\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"players\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"}]"

// PuzzleBin is the compiled bytecode used for deploying new contracts.
const PuzzleBin = `0x608060405234801561001057600080fd5b50600380546001600160a01b0319163317905561087f806100326000396000f3fe60806040526004361061007b5760003560e01c80638f3a7d8b1161004e5780638f3a7d8b1461016e578063c95e090914610230578063d826f88f14610263578063f71d96cb146102785761007b565b8063481c6a75146100805780636898f82b146100b1578063722dcd8f146100d05780638b5b9ccc14610109575b600080fd5b34801561008c57600080fd5b506100956102a2565b604080516001600160a01b039092168252519081900360200190f35b6100ce600480360360208110156100c757600080fd5b50356102b1565b005b3480156100dc57600080fd5b506100ce600480360360408110156100f357600080fd5b506001600160a01b0381351690602001356103ff565b34801561011557600080fd5b5061011e61041b565b60408051602080825283518183015283519192839290830191858101910280838360005b8381101561015a578181015183820152602001610142565b505050509050019250505060405180910390f35b6100ce6004803603608081101561018457600080fd5b6001600160a01b0382351691602081013591604082013591908101906080810160608201356401000000008111156101bb57600080fd5b8201836020820111156101cd57600080fd5b803590602001918460018302840111640100000000831117156101ef57600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955061047e945050505050565b34801561023c57600080fd5b506100ce6004803603602081101561025357600080fd5b50356001600160a01b0316610616565b34801561026f57600080fd5b506100ce6106d2565b34801561028457600080fd5b506100956004803603602081101561029b57600080fd5b50356107b8565b6003546001600160a01b031681565b60408051808201909152601181527f496e73756666696369656e742046756e6400000000000000000000000000000060208201526801158e460913d0000034101561037d57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561034257818101518382015260200161032a565b50505050905090810190601f16801561036f5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b503360009081526020819052604090205415156103d757600480546001810182556000919091527f8a35acfbc15ff81a39ae7d344fd709f28e8600b4aa8c65c6b64bfe7fe36bd19b0180546001600160a01b031916331790555b3360009081526020818152604080832093909355600181528282208290556002905220349055565b6001600160a01b03909116600090815260016020526040902055565b6060600480548060200260200160405190810160405280929190818152602001828054801561047357602002820191906000526020600020905b81546001600160a01b03168152600190910190602001808311610455575b505050505090505b90565b6003546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b0316331461050957604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561034257818101518382015260200161032a565b5081600080866001600160a01b03166001600160a01b0316815260200190815260200160002054146040518060600160405280602c8152602001610828602c913990151561059c57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561034257818101518382015260200161032a565b506001600160a01b03841660009081526001602052604090205483036105c285856103ff565b6001600160a01b03851660008181526002602052604080822054905160059091048402929183156108fc02918491818181858888f1935050505015801561060d573d6000803e3d6000fd5b50505050505050565b6003546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b031633146106a157604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561034257818101518382015260200161032a565b506001600160a01b031660009081526001602090815260408083208390558282528083208390556002909152812055565b6003546040805180820190915260138152600160681b72556e617574686f72697a656420416363657373026020820152906001600160a01b0316331461075d57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561034257818101518382015260200161032a565b5060045460005b818110156107a657600060048281548110151561077d57fe5b6000918252602090912001546001600160a01b0316905061079d81610616565b50600101610764565b5060006107b46004826107e0565b5050565b60048054829081106107c657fe5b6000918252602090912001546001600160a01b0316905081565b81548183558181111561080457600083815260209020610804918101908301610809565b505050565b61047b91905b80821115610823576000815560010161080f565b509056fe506c617965722072657175657374696e67207061796f757420666f7220756e6b6e6f776e2073657373696f6ea165627a7a72305820565023c31d341faf87f17e3031d315462faab38087fb5c7191bc471c812bf5680029`

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
// Solidity: function players(uint256 ) constant returns(address)
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

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(address player, uint256 level, uint256 session, string ) returns()
func (_Puzzle *PuzzleTransactor) Payout(opts *bind.TransactOpts, player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "payout", player, level, session, arg3)
}

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(address player, uint256 level, uint256 session, string ) returns()
func (_Puzzle *PuzzleSession) Payout(player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, session, arg3)
}

// Payout is a paid mutator transaction binding the contract method 0x8f3a7d8b.
//
// Solidity: function payout(address player, uint256 level, uint256 session, string ) returns()
func (_Puzzle *PuzzleTransactorSession) Payout(player common.Address, level *big.Int, session *big.Int, arg3 string) (*types.Transaction, error) {
	return _Puzzle.Contract.Payout(&_Puzzle.TransactOpts, player, level, session, arg3)
}

// Play is a paid mutator transaction binding the contract method 0x6898f82b.
//
// Solidity: function play(uint256 sessionID) returns()
func (_Puzzle *PuzzleTransactor) Play(opts *bind.TransactOpts, sessionID *big.Int) (*types.Transaction, error) {
	return _Puzzle.contract.Transact(opts, "play", sessionID)
}

// Play is a paid mutator transaction binding the contract method 0x6898f82b.
//
// Solidity: function play(uint256 sessionID) returns()
func (_Puzzle *PuzzleSession) Play(sessionID *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts, sessionID)
}

// Play is a paid mutator transaction binding the contract method 0x6898f82b.
//
// Solidity: function play(uint256 sessionID) returns()
func (_Puzzle *PuzzleTransactorSession) Play(sessionID *big.Int) (*types.Transaction, error) {
	return _Puzzle.Contract.Play(&_Puzzle.TransactOpts, sessionID)
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
