package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/node/worker"
)

var (
	// Test accounts
	testBankKey, _  = crypto.GenerateKey()
	testBankAddress = crypto.PubkeyToAddress(testBankKey.PublicKey)
	testBankFunds   = big.NewInt(8000000000000000000)

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
	db     ethdb.Database
	txPool *core.TxPool
	chain  *core.BlockChain
}

func main() {
	var (
		database = ethdb.NewMemDatabase()
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
	worker := worker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(testBankKey.PublicKey), 0)
	nonce := worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(testBankKey.PublicKey))
	for i := range txs {
		randomUserKey, _ := crypto.GenerateKey()
		randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i), randomUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, testBankKey)
		txs[i] = tx
	}

	// Add a contract deployment transaction
	//pragma solidity >=0.4.22 <0.6.0;
	//
	//contract Faucet {
	//	mapping(address => bool) processed;
	//	uint quota = 0.5 ether;
	//	address owner;
	//	constructor() public payable {
	//	owner = msg.sender;
	//}
	//	function request(address payable requestor) public {
	//	require(msg.sender == owner);
	//	require(quota <= address(this).balance);
	//	require(!processed[requestor]);
	//	processed[requestor] = true;
	//	requestor.transfer(quota);
	//}
	//	function money() public view returns(uint) {
	//	return address(this).balance;
	//}
	//}
	contractData := "0x60806040526802b5e3af16b188000060015560028054600160a060020a031916331790556101aa806100326000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a7230582003d799bcee73e96e0f40ca432d9c3d2aa9c00a1eba8d00877114a0d7234790ce0029"
	_ = contractData
	dataEnc := common.FromHex(contractData)

	tx, _ := types.SignTx(types.NewContractCreation(nonce+uint64(10), 0, big.NewInt(7000000000000000000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, testBankKey)
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

	var address common.Address
	bytes, err := hex.DecodeString("ac51b997a3a17fa18e46baceb32fcab9acc93917")
	address.SetBytes(bytes)
	receipts := worker.GetCurrentReceipts()
	fmt.Println(receipts[len(receipts)-1].ContractAddress)
	fmt.Println(receipts[len(receipts)-1])
	fmt.Println(state.GetNonce(testBankAddress))
	fmt.Println(testBankKey)
	fmt.Println(state.GetBalance(contractAddress))
	fmt.Println(state.GetBalance(address))
	fmt.Println(state.GetCodeHash(contractAddress))

	callData := "0x27c78c42000000000000000000000000ac51b997a3a17fa18e46baceb32fcab9acc93917" //24182601fe6e2e5da0b831496cc0489b7173b44f"
	callEnc := common.FromHex(callData)
	tx, _ = types.SignTx(types.NewTransaction(nonce+uint64(11), contractAddress, 0, big.NewInt(0), params.TxGasContractCreation*10, nil, callEnc), types.HomesteadSigner{}, testBankKey)

	err = worker.CommitTransactions(types.Transactions{tx})

	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(receipts[len(receipts)-1].ContractAddress)
	fmt.Println(receipts[len(receipts)-1])
	fmt.Println(state.GetNonce(testBankAddress))
	fmt.Println(state.GetBalance(contractAddress))
	fmt.Println(state.GetBalance(address))
	fmt.Println(state.GetCodeHash(contractAddress))
}
