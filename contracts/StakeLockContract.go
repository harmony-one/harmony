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
const StakeLockContractABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"listLockedAddresses\",\"outputs\":[{\"name\":\"lockedAddresses\",\"type\":\"address[]\"},{\"name\":\"blsAddresses\",\"type\":\"bytes20[]\"},{\"name\":\"blockNums\",\"type\":\"uint256[]\"},{\"name\":\"lockPeriodCounts\",\"type\":\"uint256[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"balance\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentEpoch\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_blsAddress\",\"type\":\"bytes20\"}],\"name\":\"lock\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unlock\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"getUnlockableTokens\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"_of\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"_amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"_epoch\",\"type\":\"uint256\"}],\"name\":\"Locked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"Unlocked\",\"type\":\"event\"}]"

// StakeLockContractBin is the compiled bytecode used for deploying new contracts.
const StakeLockContractBin = `0x6080604052600560005534801561001557600080fd5b50610b21806100256000396000f3fe6080604052600436106100555760003560e01c806363b125151461005a57806370a08231146101d7578063766718081461021c5780639de746a514610231578063a69df4b51461026c578063ab4a2eb314610281575b600080fd5b34801561006657600080fd5b5061006f6102b4565b60405180806020018060200180602001806020018060200186810386528b818151815260200191508051906020019060200280838360005b838110156100bf5781810151838201526020016100a7565b5050505090500186810385528a818151815260200191508051906020019060200280838360005b838110156100fe5781810151838201526020016100e6565b50505050905001868103845289818151815260200191508051906020019060200280838360005b8381101561013d578181015183820152602001610125565b50505050905001868103835288818151815260200191508051906020019060200280838360005b8381101561017c578181015183820152602001610164565b50505050905001868103825287818151815260200191508051906020019060200280838360005b838110156101bb5781810151838201526020016101a3565b505050509050019a505050505050505050505060405180910390f35b3480156101e357600080fd5b5061020a600480360360208110156101fa57600080fd5b50356001600160a01b031661055c565b60408051918252519081900360200190f35b34801561022857600080fd5b5061020a610577565b6102586004803603602081101561024757600080fd5b50356001600160601b03191661058c565b604080519115158252519081900360200190f35b34801561027857600080fd5b5061020a610890565b34801561028d57600080fd5b5061020a600480360360208110156102a457600080fd5b50356001600160a01b0316610a51565b6060806060806060600380548060200260200160405190810160405280929190818152602001828054801561031257602002820191906000526020600020905b81546001600160a01b031681526001909101906020018083116102f4575b50505050509450600380549050604051908082528060200260200182016040528015610348578160200160208202803883390190505b506003546040805182815260208084028201019091529195508015610377578160200160208202803883390190505b5060035460408051828152602080840282010190915291945080156103a6578160200160208202803883390190505b5060035460408051828152602080840282010190915291935080156103d5578160200160208202803883390190505b50905060005b8551811015610554576001600087838151811015156103f657fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060010154848281518110151561043257fe5b60209081029091010152855160019060009088908490811061045057fe5b60209081029091018101516001600160a01b0316825281019190915260400160002060050154855160609190911b9086908390811061048b57fe5b6001600160601b031990921660209283029091019091015285516001906000908890849081106104b757fe5b906020019060200201516001600160a01b03166001600160a01b031681526020019081526020016000206003015483828151811015156104f357fe5b60209081029091010152855160019060009088908490811061051157fe5b60209081029091018101516001600160a01b0316825281019190915260400160002054825183908390811061054257fe5b602090810290910101526001016103db565b509091929394565b6001600160a01b031660009081526001602052604090205490565b600080544381151561058557fe5b0490505b90565b60408051808201909152601f81527f424c5320616464726573732073686f756c64206e6f7420626520656d7074790060208201526000906001600160601b03198316151561065b57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b83811015610620578181015183820152602001610608565b50505050905090810190601f16801561064d5780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b506106653361055c565b60408051808201909152601581527f546f6b656e7320616c7265616479206c6f636b65640000000000000000000000602082015290156106ea57604051600160e51b62461bcd02815260040180806020018281038252838181518152602001915080519060200190808383600083811015610620578181015183820152602001610608565b5060408051808201909152601381527f416d6f756e742063616e206e6f74206265203000000000000000000000000000602082015234151561077157604051600160e51b62461bcd02815260040180806020018281038252838181518152602001915080519060200190808383600083811015610620578181015183820152602001610608565b506040518060c00160405280348152602001438152602001610791610577565b8152600160208083018290526003805480840182557fc2575a0e9e593c00f959f8c92f12db2869c3395a3b0502d05e2516446f71f85b810180546001600160a01b0319908116339081179092556040808801939093526001600160601b03198a16606097880152600082815286865283902088518155948801519585019590955590860151600284015585850151918301919091556080850151600483015560a090940151600590910180549190931c91161790557fd4665e3049283582ba6f9eba07a5b3e12dab49e02da99e8927a47af5d134bea534610870610577565b6040805192835260208301919091528051918290030190a2506001919050565b600061089b33610a51565b60408051808201909152601481527f4e6f20746f6b656e7320756e6c6f636b61626c65000000000000000000000000602082015290915081151561092457604051600160e51b62461bcd02815260040180806020018281038252838181518152602001915080519060200190808383600083811015610620578181015183820152602001610608565b50336000908152600160208190526040822060048101805484835592820184905560028201849055600380830185905593905560050180546001600160a01b03191690558154909190600019810190811061097b57fe5b600091825260209091200154600380546001600160a01b0390921691839081106109a157fe5b6000918252602082200180546001600160a01b0319166001600160a01b039390931692909217909155600380548392600192909160001981019081106109e357fe5b60009182526020808320909101546001600160a01b031683528201929092526040019020600401556003805490610a1e906000198301610aae565b50604051339083156108fc029084906000818181858888f19350505050158015610a4c573d6000803e3d6000fd5b505090565b600080610a5c610577565b6001600160a01b03841660009081526001602052604090206003818101546002909201549293500201811115610aa8576001600160a01b03831660009081526001602052604090205491505b50919050565b815481835581811115610ad257600083815260209020610ad2918101908301610ad7565b505050565b61058991905b80821115610af15760008155600101610add565b509056fea165627a7a72305820de21205eb52d566f7d604e6f7fafe84f5f070db28cf3918cb8a9ab98590b922a0029`

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
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes20[] blsAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCaller) ListLockedAddresses(opts *bind.CallOpts) (struct {
	LockedAddresses  []common.Address
	BlsAddresses     [][20]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	ret := new(struct {
		LockedAddresses  []common.Address
		BlsAddresses     [][20]byte
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
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes20[] blsAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlsAddresses     [][20]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// ListLockedAddresses is a free data retrieval call binding the contract method 0x63b12515.
//
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes20[] blsAddresses, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCallerSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlsAddresses     [][20]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// Lock is a paid mutator transaction binding the contract method 0x9de746a5.
//
// Solidity: function lock(bytes20 _blsAddress) returns(bool)
func (_StakeLockContract *StakeLockContractTransactor) Lock(opts *bind.TransactOpts, _blsAddress [20]byte) (*types.Transaction, error) {
	return _StakeLockContract.contract.Transact(opts, "lock", _blsAddress)
}

// Lock is a paid mutator transaction binding the contract method 0x9de746a5.
//
// Solidity: function lock(bytes20 _blsAddress) returns(bool)
func (_StakeLockContract *StakeLockContractSession) Lock(_blsAddress [20]byte) (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts, _blsAddress)
}

// Lock is a paid mutator transaction binding the contract method 0x9de746a5.
//
// Solidity: function lock(bytes20 _blsAddress) returns(bool)
func (_StakeLockContract *StakeLockContractTransactorSession) Lock(_blsAddress [20]byte) (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts, _blsAddress)
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
