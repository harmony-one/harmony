package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/db"
	"github.com/harmony-one/harmony/node/worker"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(1000000000000000000)

	testUserKey, _  = crypto.GenerateKey()
	testUserAddress = crypto.PubkeyToAddress(testUserKey.PublicKey)

	chainConfig = params.TestChainConfig

	// Test transactions
	pendingTxs []*types.Transaction
	newTxs     []*types.Transaction
)

func init() {
	tx1, _ := types.SignTx(types.NewTransaction(0, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	tx2, _ := types.SignTx(types.NewTransaction(1, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	tx3, _ := types.SignTx(types.NewTransaction(2, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	pendingTxs = append(pendingTxs, tx1)
	pendingTxs = append(pendingTxs, tx2)
	pendingTxs = append(pendingTxs, tx3)
	tx4, _ := types.SignTx(types.NewTransaction(1, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	newTxs = append(newTxs, tx4)
}

type testWorkerBackend struct {
	db     db.Database
	txPool *core.TxPool
	chain  *core.BlockChain
}

func main() {
	var (
		database = db.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Alloc:   core.GenesisAlloc{testBankAddress: {Balance: testBankFunds}},
			ShardID: 10,
		}
	)

	genesis := gspec.MustCommit(database)
	_ = genesis
	chain, _ := core.NewBlockChain(database, nil, gspec.Config, consensus.NewFaker(), vm.Config{}, nil)

	txpool := core.NewTxPool(core.DefaultTxPoolConfig, chainConfig, chain)

	backend := &testWorkerBackend{
		db:     database,
		chain:  chain,
		txPool: txpool,
	}
	backend.txPool.AddLocals(pendingTxs)

	//// Generate a small n-block chain and an uncle block for it
	n := 3
	if n > 0 {
		blocks, _ := core.GenerateChain(chainConfig, genesis, consensus.NewFaker(), database, n, func(i int, gen *core.BlockGen) {
			gen.SetCoinbase(testBankAddress)
			gen.SetShardID(types.EncodeShardID(10))
			gen.AddTx(pendingTxs[i])
		})
		if _, err := chain.InsertChain(blocks); err != nil {
			log.Fatal(err)
		}
	}

	txs := make([]*types.Transaction, 10)
	worker := worker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(testBankKey.PublicKey))
	nonce := worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(testBankKey.PublicKey))
	for i := range txs {
		randomUserKey, _ := crypto.GenerateKey()
		randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i), randomUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
		txs[i] = tx
	}

	// Add a contract deployment transaction
	contractData := "0x608060405234801561001057600080fd5b506040516020806104c08339810180604052602081101561003057600080fd5b505160008054600160a060020a0319163317808255600160a060020a031681526001602081905260409091205560ff811661006c600282610073565b50506100bd565b8154818355818111156100975760008381526020902061009791810190830161009c565b505050565b6100ba91905b808211156100b657600081556001016100a2565b5090565b90565b6103f4806100cc6000396000f3fe60806040526004361061005b577c010000000000000000000000000000000000000000000000000000000060003504635c19a95c8114610060578063609ff1bd146100955780639e7b8d61146100c0578063b3f98adc146100f3575b600080fd5b34801561006c57600080fd5b506100936004803603602081101561008357600080fd5b5035600160a060020a0316610120565b005b3480156100a157600080fd5b506100aa610280565b6040805160ff9092168252519081900360200190f35b3480156100cc57600080fd5b50610093600480360360208110156100e357600080fd5b5035600160a060020a03166102eb565b3480156100ff57600080fd5b506100936004803603602081101561011657600080fd5b503560ff16610348565b3360009081526001602081905260409091209081015460ff1615610144575061027d565b5b600160a060020a0382811660009081526001602081905260409091200154620100009004161580159061019c5750600160a060020a0382811660009081526001602081905260409091200154620100009004163314155b156101ce57600160a060020a039182166000908152600160208190526040909120015462010000900490911690610145565b600160a060020a0382163314156101e5575061027d565b6001818101805460ff1916821775ffffffffffffffffffffffffffffffffffffffff0000191662010000600160a060020a0386169081029190911790915560009081526020829052604090209081015460ff16156102725781546001820154600280549091610100900460ff1690811061025b57fe5b60009182526020909120018054909101905561027a565b815481540181555b50505b50565b600080805b60025460ff821610156102e6578160028260ff168154811015156102a557fe5b906000526020600020016000015411156102de576002805460ff83169081106102ca57fe5b906000526020600020016000015491508092505b600101610285565b505090565b600054600160a060020a0316331415806103215750600160a060020a0381166000908152600160208190526040909120015460ff165b1561032b5761027d565b600160a060020a0316600090815260016020819052604090912055565b3360009081526001602081905260409091209081015460ff1680610371575060025460ff831610155b1561037c575061027d565b6001818101805460ff191690911761ff00191661010060ff8516908102919091179091558154600280549192909181106103b257fe5b600091825260209091200180549091019055505056fea165627a7a72305820164189ef302b4648e01e22456b0a725191604cb63ee472f230ef6a2d17d702f900290000000000000000000000000000000000000000000000000000000000000002"
	_ = contractData
	dataEnc := common.FromHex(contractData)

	tx, _ := types.SignTx(types.NewContractCreation(nonce+uint64(10), 0, big.NewInt(1000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, testBankKey)
	// Figure out the contract address which is determined by sender.address and nonce
	contractAddress := crypto.CreateAddress(testBankAddress, nonce+uint64(10))

	state := worker.GetCurrentState()
	// Before the contract is deployed the code is empty
	fmt.Println(state.GetCodeHash(contractAddress))

	txs = append(txs, tx)
	err := worker.CommitTransactions(txs)
	if err != nil {
		fmt.Println(err)
	}
	block, _ := worker.Commit()
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(contractAddress)

	receipts := worker.GetCurrentReceipts()
	fmt.Println(receipts[len(receipts)-1].ContractAddress)
	fmt.Println(receipts[len(receipts)-1])
	fmt.Println(state.GetNonce(testBankAddress))
	fmt.Println(state.GetNonce(contractAddress))
	fmt.Println(state.GetBalance(contractAddress))

	//tx, _ = types.SignTx(types.NewTransaction(nonce+uint64(11), contractAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
	//
	//err = worker.CommitTransactions(types.Transactions{tx})
	//if err != nil {
	//	fmt.Println(err)
	//}
	fmt.Println(state.GetCodeHash(contractAddress))
}
