package contracts

import (
	"sync"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/harmony-one/harmony/internal/params"

	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
)

// ContractCaller is used to call smart contract locally
type ContractCaller struct {
	blockchain *core.BlockChain // Ethereum blockchain to handle the consensus

	mu     sync.Mutex
	config *params.ChainConfig
}

// NewContractCaller initialize a new contract caller.
func NewContractCaller(bc *core.BlockChain, config *params.ChainConfig) *ContractCaller {
	cc := ContractCaller{}
	cc.blockchain = bc
	cc.mu = sync.Mutex{}
	cc.config = config
	return &cc
}

// CallContract calls a contracts with the specified transaction.
func (cc *ContractCaller) CallContract(tx *types.Transaction) ([]byte, error) {
	currBlock := cc.blockchain.CurrentBlock()
	msg, err := tx.AsMessage(types.MakeSigner(cc.config, currBlock.Header().Epoch))
	if err != nil {
		utils.GetLogInstance().Error("[ABI] Failed to convert transaction to message", "error", err)
		return []byte{}, err
	}
	evmContext := core.NewEVMContext(msg, currBlock.Header(), cc.blockchain, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	stateDB, err := cc.blockchain.State()
	if err != nil {
		utils.GetLogInstance().Error("[ABI] Failed to retrieve state db", "error", err)
		return []byte{}, err
	}
	vmenv := vm.NewEVM(evmContext, stateDB, cc.config, vm.Config{})
	gaspool := new(core.GasPool).AddGas(math.MaxUint64)

	returnValue, _, failed, err := core.NewStateTransition(vmenv, msg, gaspool).TransitionDb()
	if err != nil || failed {
		utils.GetLogInstance().Error("[ABI] Failed executing the transaction", "error", err)
		return []byte{}, err
	}
	return returnValue, nil
}
