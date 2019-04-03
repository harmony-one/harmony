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
const StakeLockContractABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"listLockedAddresses\",\"outputs\":[{\"name\":\"lockedAddresses\",\"type\":\"address[]\"},{\"name\":\"blsPubicKeys1\",\"type\":\"bytes32[]\"},{\"name\":\"blsPubicKeys2\",\"type\":\"bytes32[]\"},{\"name\":\"blsPubicKeys3\",\"type\":\"bytes32[]\"},{\"name\":\"blockNums\",\"type\":\"uint256[]\"},{\"name\":\"lockPeriodCounts\",\"type\":\"uint256[]\"},{\"name\":\"amounts\",\"type\":\"uint256[]\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"name\":\"balance\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"currentEpoch\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_blsPublicKey1\",\"type\":\"bytes32\"},{\"name\":\"_blsPublicKey2\",\"type\":\"bytes32\"},{\"name\":\"_blsPublicKey3\",\"type\":\"bytes32\"}],\"name\":\"lock\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":true,\"stateMutability\":\"payable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unlock\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_of\",\"type\":\"address\"}],\"name\":\"getUnlockableTokens\",\"outputs\":[{\"name\":\"unlockableTokens\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"_of\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"_amount\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"_epoch\",\"type\":\"uint256\"}],\"name\":\"Locked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"account\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"Unlocked\",\"type\":\"event\"}]"

// StakeLockContractBin is the compiled bytecode used for deploying new contracts.
const StakeLockContractBin = `0x6080604052600560005534801561001557600080fd5b50610c3c806100256000396000f3fe6080604052600436106100555760003560e01c806363b125151461005a57806370a082311461026157806376671808146102a6578063872588ba146102bb578063a69df4b5146102f8578063ab4a2eb31461030d575b600080fd5b34801561006657600080fd5b5061006f610340565b604051808060200180602001806020018060200180602001806020018060200188810388528f818151815260200191508051906020019060200280838360005b838110156100c75781810151838201526020016100af565b5050505090500188810387528e818151815260200191508051906020019060200280838360005b838110156101065781810151838201526020016100ee565b5050505090500188810386528d818151815260200191508051906020019060200280838360005b8381101561014557818101518382015260200161012d565b5050505090500188810385528c818151815260200191508051906020019060200280838360005b8381101561018457818101518382015260200161016c565b5050505090500188810384528b818151815260200191508051906020019060200280838360005b838110156101c35781810151838201526020016101ab565b5050505090500188810383528a818151815260200191508051906020019060200280838360005b838110156102025781810151838201526020016101ea565b50505050905001888103825289818151815260200191508051906020019060200280838360005b83811015610241578181015183820152602001610229565b505050509050019e50505050505050505050505050505060405180910390f35b34801561026d57600080fd5b506102946004803603602081101561028457600080fd5b50356001600160a01b03166106f2565b60408051918252519081900360200190f35b3480156102b257600080fd5b5061029461070d565b6102e4600480360360608110156102d157600080fd5b5080359060208101359060400135610722565b604080519115158252519081900360200190f35b34801561030457600080fd5b506102946109a4565b34801561031957600080fd5b506102946004803603602081101561033057600080fd5b50356001600160a01b0316610b6c565b606080606080606080606060038054806020026020016040519081016040528092919081815260200182805480156103a157602002820191906000526020600020905b81546001600160a01b03168152600190910190602001808311610383575b505050505096506003805490506040519080825280602002602001820160405280156103d7578160200160208202803883390190505b506003546040805182815260208084028201019091529197508015610406578160200160208202803883390190505b506003546040805182815260208084028201019091529196508015610435578160200160208202803883390190505b506003546040805182815260208084028201019091529195508015610464578160200160208202803883390190505b506003546040805182815260208084028201019091529194508015610493578160200160208202803883390190505b5060035460408051828152602080840282010190915291935080156104c2578160200160208202803883390190505b50905060005b87518110156106e8576001600089838151811015156104e357fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060010154848281518110151561051f57fe5b6020908102909101015287516001906000908a908490811061053d57fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060050154878281518110151561057957fe5b6020908102909101015287516001906000908a908490811061059757fe5b906020019060200201516001600160a01b03166001600160a01b031681526020019081526020016000206006015486828151811015156105d357fe5b6020908102909101015287516001906000908a90849081106105f157fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060070154858281518110151561062d57fe5b6020908102909101015287516001906000908a908490811061064b57fe5b906020019060200201516001600160a01b03166001600160a01b0316815260200190815260200160002060030154838281518110151561068757fe5b6020908102909101015287516001906000908a90849081106106a557fe5b60209081029091018101516001600160a01b031682528101919091526040016000205482518390839081106106d657fe5b602090810290910101526001016104c8565b5090919293949596565b6001600160a01b031660009081526001602052604090205490565b600080544381151561071b57fe5b0490505b90565b600061072d336106f2565b60408051808201909152601581527f546f6b656e7320616c7265616479206c6f636b65640000000000000000000000602082015290156107ee57604051600160e51b62461bcd0281526004018080602001828103825283818151815260200191508051906020019080838360005b838110156107b357818101518382015260200161079b565b50505050905090810190601f1680156107e05780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b5060408051808201909152601381527f416d6f756e742063616e206e6f74206265203000000000000000000000000000602082015234151561087557604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156107b357818101518382015260200161079b565b5060405180610100016040528034815260200143815260200161089661070d565b8152600160208083018290526003805480840182557fc2575a0e9e593c00f959f8c92f12db2869c3395a3b0502d05e2516446f71f85b810180546001600160a01b0319163390811790915560408087019290925260608087018c905260808088018c905260a09788018b90526000838152878752849020895181559589015196860196909655918701516002850155908601519183019190915591840151600482015591830151600583015560c0830151600683015560e0909201516007909101557fd4665e3049283582ba6f9eba07a5b3e12dab49e02da99e8927a47af5d134bea53461098261070d565b6040805192835260208301919091528051918290030190a25060019392505050565b60006109af33610b6c565b60408051808201909152601481527f4e6f20746f6b656e7320756e6c6f636b61626c650000000000000000000000006020820152909150811515610a3857604051600160e51b62461bcd028152600401808060200182810382528381815181526020019150805190602001908083836000838110156107b357818101518382015260200161079b565b50336000908152600160208190526040822060048101805484835592820184905560028201849055600380830185905590849055600582018490556006820184905560079091019290925581549091906000198101908110610a9657fe5b600091825260209091200154600380546001600160a01b039092169183908110610abc57fe5b6000918252602082200180546001600160a01b0319166001600160a01b03939093169290921790915560038054839260019290916000198101908110610afe57fe5b60009182526020808320909101546001600160a01b031683528201929092526040019020600401556003805490610b39906000198301610bc9565b50604051339083156108fc029084906000818181858888f19350505050158015610b67573d6000803e3d6000fd5b505090565b600080610b7761070d565b6001600160a01b03841660009081526001602052604090206003818101546002909201549293500201811115610bc3576001600160a01b03831660009081526001602052604090205491505b50919050565b815481835581811115610bed57600083815260209020610bed918101908301610bf2565b505050565b61071f91905b80821115610c0c5760008155600101610bf8565b509056fea165627a7a72305820a5a16a162731a647a892a79ba1d8589b81a9855944e0e2dbced079c0211a8a720029`

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
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes32[] blsPubicKeys1, bytes32[] blsPubicKeys2, bytes32[] blsPubicKeys3, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCaller) ListLockedAddresses(opts *bind.CallOpts) (struct {
	LockedAddresses  []common.Address
	BlsPubicKeys1    [][32]byte
	BlsPubicKeys2    [][32]byte
	BlsPubicKeys3    [][32]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	ret := new(struct {
		LockedAddresses  []common.Address
		BlsPubicKeys1    [][32]byte
		BlsPubicKeys2    [][32]byte
		BlsPubicKeys3    [][32]byte
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
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes32[] blsPubicKeys1, bytes32[] blsPubicKeys2, bytes32[] blsPubicKeys3, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlsPubicKeys1    [][32]byte
	BlsPubicKeys2    [][32]byte
	BlsPubicKeys3    [][32]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// ListLockedAddresses is a free data retrieval call binding the contract method 0x63b12515.
//
// Solidity: function listLockedAddresses() constant returns(address[] lockedAddresses, bytes32[] blsPubicKeys1, bytes32[] blsPubicKeys2, bytes32[] blsPubicKeys3, uint256[] blockNums, uint256[] lockPeriodCounts, uint256[] amounts)
func (_StakeLockContract *StakeLockContractCallerSession) ListLockedAddresses() (struct {
	LockedAddresses  []common.Address
	BlsPubicKeys1    [][32]byte
	BlsPubicKeys2    [][32]byte
	BlsPubicKeys3    [][32]byte
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int
	Amounts          []*big.Int
}, error) {
	return _StakeLockContract.Contract.ListLockedAddresses(&_StakeLockContract.CallOpts)
}

// Lock is a paid mutator transaction binding the contract method 0x872588ba.
//
// Solidity: function lock(bytes32 _blsPublicKey1, bytes32 _blsPublicKey2, bytes32 _blsPublicKey3) returns(bool)
func (_StakeLockContract *StakeLockContractTransactor) Lock(opts *bind.TransactOpts, _blsPublicKey1 [32]byte, _blsPublicKey2 [32]byte, _blsPublicKey3 [32]byte) (*types.Transaction, error) {
	return _StakeLockContract.contract.Transact(opts, "lock", _blsPublicKey1, _blsPublicKey2, _blsPublicKey3)
}

// Lock is a paid mutator transaction binding the contract method 0x872588ba.
//
// Solidity: function lock(bytes32 _blsPublicKey1, bytes32 _blsPublicKey2, bytes32 _blsPublicKey3) returns(bool)
func (_StakeLockContract *StakeLockContractSession) Lock(_blsPublicKey1 [32]byte, _blsPublicKey2 [32]byte, _blsPublicKey3 [32]byte) (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts, _blsPublicKey1, _blsPublicKey2, _blsPublicKey3)
}

// Lock is a paid mutator transaction binding the contract method 0x872588ba.
//
// Solidity: function lock(bytes32 _blsPublicKey1, bytes32 _blsPublicKey2, bytes32 _blsPublicKey3) returns(bool)
func (_StakeLockContract *StakeLockContractTransactorSession) Lock(_blsPublicKey1 [32]byte, _blsPublicKey2 [32]byte, _blsPublicKey3 [32]byte) (*types.Transaction, error) {
	return _StakeLockContract.Contract.Lock(&_StakeLockContract.TransactOpts, _blsPublicKey1, _blsPublicKey2, _blsPublicKey3)
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
