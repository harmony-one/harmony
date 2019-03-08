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

// StakeLockContractABI is the input ABI used to generate the binding from.
const StakeLockContractABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"listLockedAddresses\",\"outputs\":[{\"name\":\"lockedAddresses\",\"type\":\"address[]\"},{\"name\":\"blockNums\",\"type\":\"uint256[]\"},{\"name\":\"lockPeriodCounts\",\"type\":\"uint256[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"balance\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentEpoch\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unlock\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"getUnlockableTokens\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"lock\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"_of\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"_amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"_epoch\",\"type\":\"uint256\"}],\"name\":\"Locked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"Unlocked\",\"type\":\"event\"}]"

// StakeLockContractBin is the compiled bytecode used for deploying new contracts.
const StakeLockContractBin = `0x6080604052600560005534801561001557600080fd5b5061095a806100256000396000f3fe6080604052600436106100555760003560e01c806363b125151461005a57806370a082311461019257806376671808146101d7578063a69df4b5146101ec578063ab4a2eb314610201578063f83d08ba14610234575b600080fd5b34801561006657600080fd5b5061006f610250565b6040518080602001806020018060200180602001858103855289818151815260200191508051906020019060200280838360005b838110156100bb5781810151838201526020016100a3565b50505050905001858103845288818151815260200191508051906020019060200280838360005b838110156100fa5781810151838201526020016100e2565b50505050905001858103835287818151815260200191508051906020019060200280838360005b83811015610139578181015183820152602001610121565b50505050905001858103825286818151815260200191508051906020019060200280838360005b83811015610178578181015183820152602001610160565b505050509050019850505050505050505060405180910390f35b34801561019e57600080fd5b506101c5600480360360208110156101b557600080fd5b50356001600160a01b031661045f565b60408051918252519081900360200190f35b3480156101e357600080fd5b506101c561047a565b3480156101f857600080fd5b506101c561048f565b34801561020d57600080fd5b506101c56004803603602081101561022457600080fd5b50356001600160a01b031661067c565b61023c6106d9565b604080519115158252519081900360200190f35b60608060608060038054806020026020016040519081016040528092919081815260200182805480156102ac57602002820191906000526020600020905b81546001600160a01b0316815260019091019060200180831161028e575b505050505093506003805490506040519080825280602002602001820160405280156102e2578160200160208202803883390190505b506003546040805182815260208084028201019091529194508015610311578160200160208202803883390190505b506003546040805182815260208084028201019091529193508015610340578160200160208202803883390190505b50905060005b84518110156104585760016000868381518110151561036157fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060010154848281518110151561039d57fe5b6020908102909101015284516001906000908790849081106103bb57fe5b906020019060200201516001600160a01b03166001600160a01b031681526020019081526020016000206003015483828151811015156103f757fe5b60209081029091010152845160019060009087908490811061041557fe5b60209081029091018101516001600160a01b0316825281019190915260400160002054825183908390811061044657fe5b60209081029091010152600101610346565b5090919293565b6001600160a01b031660009081526001602052604090205490565b600080544381151561048857fe5b0490505b90565b600061049a3361067c565b60408051808201909152601481527f4e6f20746f6b656e7320756e6c6f636b61626c65000000000000000000000000602082015290915081151561055f57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b8381101561052457818101518382015260200161050c565b50505050905090810190601f1680156105515780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b50336000908152600160208190526040822060048101805484835592820184905560028201849055600391820184905592909255815490919060001981019081106105a657fe5b600091825260209091200154600380546001600160a01b0390921691839081106105cc57fe5b6000918252602082200180546001600160a01b0319166001600160a01b0393909316929092179091556003805483926001929091600019810190811061060e57fe5b60009182526020808320909101546001600160a01b0316835282019290925260400190206004015560038054906106499060001983016108e7565b50604051339083156108fc029084906000818181858888f19350505050158015610677573d6000803e3d6000fd5b505090565b60008061068761047a565b6001600160a01b038416600090815260016020526040902060038181015460029092015492935002018111156106d3576001600160a01b03831660009081526001602052604090205491505b50919050565b60006106e43361045f565b60408051808201909152601581527f546f6b656e7320616c7265616479206c6f636b656400000000000000000000006020820152901561076957604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561052457818101518382015260200161050c565b5060408051808201909152601381527f416d6f756e742063616e206e6f7420626520300000000000000000000000000060208201523415156107f057604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360008381101561052457818101518382015260200161050c565b506040518060a0016040528034815260200143815260200161081061047a565b8152600160208083018290526003805480840182557fc2575a0e9e593c00f959f8c92f12db2869c3395a3b0502d05e2516446f71f85b810180546001600160a01b0319163390811790915560409586019190915260008181528484528590208651815592860151938301939093559284015160028201556060840151928101929092556080909201516004909101557fd4665e3049283582ba6f9eba07a5b3e12dab49e02da99e8927a47af5d134bea5346108c961047a565b6040805192835260208301919091528051918290030190a250600190565b81548183558181111561090b5760008381526020902061090b918101908301610910565b505050565b61048c91905b8082111561092a5760008155600101610916565b509056fea165627a7a723058209f27dc5388d4508d0aeccbbf0ae74929289525f4918465e7eee508195182385c0029`

// DeployStakeLockContract deploys a new Ethereum contract, binding an instance of StakeLockContract to it.
func DeployStakeLockContract(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *StakeLockContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StakeLockContractABI))
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	address, tx, contract, err := bind.DeployContract(auth, parsed, common.FromHex(StakeLockContractBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &StakeLockContract{StakeLockContractCaller: StakeLockContractCaller{contract: contract}, StakeLockContractTransactor: StakeLockContractTransactor{contract: contract}, StakeLockContractFilterer: StakeLockContractFilterer{contract: contract}}, nil
}

// StakeLockContract is an auto generated Go binding around an Ethereum contract.
type StakeLockContract struct {
	StakeLockContractCaller     // Read-only binding to the contract
	StakeLockContractTransactor // Write-only binding to the contract
	StakeLockContractFilterer   // Log filterer for contract events
}

// StakeLockContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type StakeLockContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakeLockContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StakeLockContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakeLockContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StakeLockContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StakeLockContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StakeLockContractSession struct {
	Contract     *StakeLockContract // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// StakeLockContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StakeLockContractCallerSession struct {
	Contract *StakeLockContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// StakeLockContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StakeLockContractTransactorSession struct {
	Contract     *StakeLockContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// StakeLockContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type StakeLockContractRaw struct {
	Contract *StakeLockContract // Generic contract binding to access the raw methods on
}

// StakeLockContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StakeLockContractCallerRaw struct {
	Contract *StakeLockContractCaller // Generic read-only contract binding to access the raw methods on
}

// StakeLockContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StakeLockContractTransactorRaw struct {
	Contract *StakeLockContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStakeLockContract creates a new instance of StakeLockContract, bound to a specific deployed contract.
func NewStakeLockContract(address common.Address, backend bind.ContractBackend) (*StakeLockContract, error) {
	contract, err := bindStakeLockContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &StakeLockContract{StakeLockContractCaller: StakeLockContractCaller{contract: contract}, StakeLockContractTransactor: StakeLockContractTransactor{contract: contract}, StakeLockContractFilterer: StakeLockContractFilterer{contract: contract}}, nil
}

// NewStakeLockContractCaller creates a new read-only instance of StakeLockContract, bound to a specific deployed contract.
func NewStakeLockContractCaller(address common.Address, caller bind.ContractCaller) (*StakeLockContractCaller, error) {
	contract, err := bindStakeLockContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StakeLockContractCaller{contract: contract}, nil
}

// NewStakeLockContractTransactor creates a new write-only instance of StakeLockContract, bound to a specific deployed contract.
func NewStakeLockContractTransactor(address common.Address, transactor bind.ContractTransactor) (*StakeLockContractTransactor, error) {
	contract, err := bindStakeLockContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StakeLockContractTransactor{contract: contract}, nil
}

// NewStakeLockContractFilterer creates a new log filterer instance of StakeLockContract, bound to a specific deployed contract.
func NewStakeLockContractFilterer(address common.Address, filterer bind.ContractFilterer) (*StakeLockContractFilterer, error) {
	contract, err := bindStakeLockContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StakeLockContractFilterer{contract: contract}, nil
}

// bindStakeLockContract binds a generic wrapper to an already deployed contract.
func bindStakeLockContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StakeLockContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StakeLockContract *StakeLockContractRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _StakeLockContract.Contract.StakeLockContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StakeLockContract *StakeLockContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakeLockContract.Contract.StakeLockContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StakeLockContract *StakeLockContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StakeLockContract.Contract.StakeLockContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_StakeLockContract *StakeLockContractCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _StakeLockContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_StakeLockContract *StakeLockContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakeLockContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_StakeLockContract *StakeLockContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _StakeLockContract.Contract.contract.Transact(opts, method, params...)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _of) constant returns(uint256 balance)
func (_StakeLockContract *StakeLockContractCaller) BalanceOf(opts *bind.CallOpts, _of common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _StakeLockContract.contract.Call(opts, out, "balanceOf", _of)
	return *ret0, err
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _of) constant returns(uint256 balance)
func (_StakeLockContract *StakeLockContractSession) BalanceOf(_of common.Address) (*big.Int, error) {
	return _StakeLockContract.Contract.BalanceOf(&_StakeLockContract.CallOpts, _of)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _of) constant returns(uint256 balance)
func (_StakeLockContract *StakeLockContractCallerSession) BalanceOf(_of common.Address) (*big.Int, error) {
	return _StakeLockContract.Contract.BalanceOf(&_StakeLockContract.CallOpts, _of)
}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() constant returns(uint256)
func (_StakeLockContract *StakeLockContractCaller) CurrentEpoch(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _StakeLockContract.contract.Call(opts, out, "currentEpoch")
	return *ret0, err
}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() constant returns(uint256)
func (_StakeLockContract *StakeLockContractSession) CurrentEpoch() (*big.Int, error) {
	return _StakeLockContract.Contract.CurrentEpoch(&_StakeLockContract.CallOpts)
}

// CurrentEpoch is a free data retrieval call binding the contract method 0x76671808.
//
// Solidity: function currentEpoch() constant returns(uint256)
func (_StakeLockContract *StakeLockContractCallerSession) CurrentEpoch() (*big.Int, error) {
	return _StakeLockContract.Contract.CurrentEpoch(&_StakeLockContract.CallOpts)
}

// GetUnlockableTokens is a free data retrieval call binding the contract method 0xab4a2eb3.
//
// Solidity: function getUnlockableTokens(address _of) constant returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractCaller) GetUnlockableTokens(opts *bind.CallOpts, _of common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _StakeLockContract.contract.Call(opts, out, "getUnlockableTokens", _of)
	return *ret0, err
}

// GetUnlockableTokens is a free data retrieval call binding the contract method 0xab4a2eb3.
//
// Solidity: function getUnlockableTokens(address _of) constant returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractSession) GetUnlockableTokens(_of common.Address) (*big.Int, error) {
	return _StakeLockContract.Contract.GetUnlockableTokens(&_StakeLockContract.CallOpts, _of)
}

// GetUnlockableTokens is a free data retrieval call binding the contract method 0xab4a2eb3.
//
// Solidity: function getUnlockableTokens(address _of) constant returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractCallerSession) GetUnlockableTokens(_of common.Address) (*big.Int, error) {
	return _StakeLockContract.Contract.GetUnlockableTokens(&_StakeLockContract.CallOpts, _of)
}

// ListLockedAddresses is a free data retrieval call binding the contract method 0x63b12515.
//
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCaller) ListLockedAddresses(opts *bind.CallOpts) (struct {
	LockedAddresses  []common.Address
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	ret := new(struct {
		LockedAddresses  []common.Address
		BlockNums        []*big.Int
		LockPeriodCounts []*big.Int
		Amounts          []*big.Int
	})
	out := ret
	err := _StakeLockContract.contract.Call(opts, out, "listLockedAddresses")
	return *ret, err
}

// ListLockedAddresses is a free data retrieval call binding the contract method 0x63b12515.
//
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// ListLockedAddresses is a free data retrieval call binding the contract method 0x63b12515.
//
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCallerSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns(bool)
func (_StakeLockContract *StakeLockContractTransactor) Lock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakeLockContract.contract.Transact(opts, "lock")
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns(bool)
func (_StakeLockContract *StakeLockContractSession) Lock() (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts)
}

// Lock is a paid mutator transaction binding the contract method 0xf83d08ba.
//
// Solidity: function lock() returns(bool)
func (_StakeLockContract *StakeLockContractTransactorSession) Lock() (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractTransactor) Unlock(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _StakeLockContract.contract.Transact(opts, "unlock")
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractSession) Unlock() (*types.Transaction, error) {
	return _StakeLockContract.Contract.Unlock(&_StakeLockContract.TransactOpts)
}

// Unlock is a paid mutator transaction binding the contract method 0xa69df4b5.
//
// Solidity: function unlock() returns(uint256 unlockableTokens)
func (_StakeLockContract *StakeLockContractTransactorSession) Unlock() (*types.Transaction, error) {
	return _StakeLockContract.Contract.Unlock(&_StakeLockContract.TransactOpts)
}

// StakeLockContractLockedIterator is returned from FilterLocked and is used to iterate over the raw logs and unpacked data for Locked events raised by the StakeLockContract contract.
type StakeLockContractLockedIterator struct {
	Event *StakeLockContractLocked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakeLockContractLockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakeLockContractLocked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakeLockContractLocked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakeLockContractLockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakeLockContractLockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakeLockContractLocked represents a Locked event raised by the StakeLockContract contract.
type StakeLockContractLocked struct {
	Of     common.Address
	Amount *big.Int
	Epoch  *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterLocked is a free log retrieval operation binding the contract event 0xd4665e3049283582ba6f9eba07a5b3e12dab49e02da99e8927a47af5d134bea5.
//
// Solidity: event Locked(address indexed _of, uint256 _amount, uint256 _epoch)
func (_StakeLockContract *StakeLockContractFilterer) FilterLocked(opts *bind.FilterOpts, _of []common.Address) (*StakeLockContractLockedIterator, error) {

	var _ofRule []interface{}
	for _, _ofItem := range _of {
		_ofRule = append(_ofRule, _ofItem)
	}

	logs, sub, err := _StakeLockContract.contract.FilterLogs(opts, "Locked", _ofRule)
	if err != nil {
		return nil, err
	}
	return &StakeLockContractLockedIterator{contract: _StakeLockContract.contract, event: "Locked", logs: logs, sub: sub}, nil
}

// WatchLocked is a free log subscription operation binding the contract event 0xd4665e3049283582ba6f9eba07a5b3e12dab49e02da99e8927a47af5d134bea5.
//
// Solidity: event Locked(address indexed _of, uint256 _amount, uint256 _epoch)
func (_StakeLockContract *StakeLockContractFilterer) WatchLocked(opts *bind.WatchOpts, sink chan<- *StakeLockContractLocked, _of []common.Address) (event.Subscription, error) {

	var _ofRule []interface{}
	for _, _ofItem := range _of {
		_ofRule = append(_ofRule, _ofItem)
	}

	logs, sub, err := _StakeLockContract.contract.WatchLogs(opts, "Locked", _ofRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakeLockContractLocked)
				if err := _StakeLockContract.contract.UnpackLog(event, "Locked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// StakeLockContractUnlockedIterator is returned from FilterUnlocked and is used to iterate over the raw logs and unpacked data for Unlocked events raised by the StakeLockContract contract.
type StakeLockContractUnlockedIterator struct {
	Event *StakeLockContractUnlocked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *StakeLockContractUnlockedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StakeLockContractUnlocked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(StakeLockContractUnlocked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *StakeLockContractUnlockedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StakeLockContractUnlockedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StakeLockContractUnlocked represents a Unlocked event raised by the StakeLockContract contract.
type StakeLockContractUnlocked struct {
	Account common.Address
	Index   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnlocked is a free log retrieval operation binding the contract event 0x0f0bc5b519ddefdd8e5f9e6423433aa2b869738de2ae34d58ebc796fc749fa0d.
//
// Solidity: event Unlocked(address indexed account, uint256 index)
func (_StakeLockContract *StakeLockContractFilterer) FilterUnlocked(opts *bind.FilterOpts, account []common.Address) (*StakeLockContractUnlockedIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _StakeLockContract.contract.FilterLogs(opts, "Unlocked", accountRule)
	if err != nil {
		return nil, err
	}
	return &StakeLockContractUnlockedIterator{contract: _StakeLockContract.contract, event: "Unlocked", logs: logs, sub: sub}, nil
}

// WatchUnlocked is a free log subscription operation binding the contract event 0x0f0bc5b519ddefdd8e5f9e6423433aa2b869738de2ae34d58ebc796fc749fa0d.
//
// Solidity: event Unlocked(address indexed account, uint256 index)
func (_StakeLockContract *StakeLockContractFilterer) WatchUnlocked(opts *bind.WatchOpts, sink chan<- *StakeLockContractUnlocked, account []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}

	logs, sub, err := _StakeLockContract.contract.WatchLogs(opts, "Unlocked", accountRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StakeLockContractUnlocked)
				if err := _StakeLockContract.contract.UnpackLog(event, "Unlocked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}
