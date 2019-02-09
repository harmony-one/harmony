package main

import (
	"crypto/ecdsa"
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
	pkgworker "github.com/harmony-one/harmony/node/worker"
)

const (
	//StakingContractBinary is binary for staking contract.
	StakingContractBinary = "0x608060405234801561001057600080fd5b50610b51806100206000396000f3fe608060405260043610610072576000357c01000000000000000000000000000000000000000000000000000000009004806325ca4c9c146100775780632e1a7d4d146100e05780634c1b64cb1461012f578063a98e4e7714610194578063d0e30db0146101bf578063e27fd057146101dd575b600080fd5b34801561008357600080fd5b506100c66004803603602081101561009a57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610249565b604051808215151515815260200191505060405180910390f35b3480156100ec57600080fd5b506101196004803603602081101561010357600080fd5b8101908080359060200190929190505050610310565b6040518082815260200191505060405180910390f35b34801561013b57600080fd5b5061017e6004803603602081101561015257600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506104cb565b6040518082815260200191505060405180910390f35b3480156101a057600080fd5b506101a96106f3565b6040518082815260200191505060405180910390f35b6101c7610700565b6040518082815260200191505060405180910390f35b3480156101e957600080fd5b506101f2610a46565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561023557808201518184015260208101905061021a565b505050509050019250505060405180910390f35b6000806002805490501415610261576000905061030b565b8173ffffffffffffffffffffffffffffffffffffffff166002600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548154811015156102c657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161490505b919050565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020548211151561048457816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055503373ffffffffffffffffffffffffffffffffffffffff166108fc839081150290604051600060405180830381858888f193505050501580156103eb573d6000803e3d6000fd5b5060008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054141561043e5761043c336104cb565b505b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506104c6565b6000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b600080600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506000600260016002805490500381548110151561052957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690508060028381548110151561056657fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555081600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555060028054809190600190036106079190610ad4565b508373ffffffffffffffffffffffffffffffffffffffff167e1fab73a76dc2de66330e055b1c1e3319c77b736bb4478cc706497f318a4ad7836040518082815260200191505060405180910390a28073ffffffffffffffffffffffffffffffffffffffff167f6095abd20e12b7e743432b409b7879ac77a0b927f89ae330f59c15b32dce0b69836000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a28192505050919050565b6000600280549050905090565b600061070b33610249565b1561087657346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055503373ffffffffffffffffffffffffffffffffffffffff167f6095abd20e12b7e743432b409b7879ac77a0b927f89ae330f59c15b32dce0b69600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a2600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050610a43565b346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550600160023390806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555003600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055503373ffffffffffffffffffffffffffffffffffffffff167fd2ad617bb539c9a6219058035b15d87478e478eb0f74164eae890a0c70fa3f40600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020546000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054604051808381526020018281526020019250505060405180910390a260016002805490500390505b90565b60606002805480602002602001604051908101604052809291908181526020018280548015610aca57602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610a80575b5050505050905090565b815481835581811115610afb57818360005260206000209182019101610afa9190610b00565b5b505050565b610b2291905b80821115610b1e576000816000905550600101610b06565b5090565b9056fea165627a7a7230582032eb9f748231d6ef2fd0ac6bd603cc3e381b6a6596e87beb1f6ce117e5237f4b0029"
	//FaucetContractBinary is binary for faucet contract.
	FaucetContractBinary = "0x60806040526802b5e3af16b188000060015560028054600160a060020a031916331790556101aa806100326000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a5780634ddd108a1461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a7230582003d799bcee73e96e0f40ca432d9c3d2aa9c00a1eba8d00877114a0d7234790ce0029"
)

var (
	//FaucetPriKey for the faucet contract Test accounts
	FaucetPriKey, _ = crypto.GenerateKey()
	//FaucetAddress generated via the key.
	FaucetAddress = crypto.PubkeyToAddress(FaucetPriKey.PublicKey)
	//FaucetInitFunds initial funds in facuet contract
	FaucetInitFunds = big.NewInt(8000000000000000000)

	testUserKey, _  = crypto.GenerateKey()
	testUserAddress = crypto.PubkeyToAddress(testUserKey.PublicKey)

	chainConfig = params.TestChainConfig

	//StakingPriKey is the keys for the deposit contract.
	StakingPriKey, _ = crypto.GenerateKey()
	//StakingAddress is the address of the deposit contract.
	StakingAddress = crypto.PubkeyToAddress(StakingPriKey.PublicKey)

	// Test transactions
	pendingTxs []*types.Transaction
	newTxs     []*types.Transaction
)

func init() {

	firstRandomUserKey, _ := crypto.GenerateKey()
	firstRandomUserAddress := crypto.PubkeyToAddress(firstRandomUserKey.PublicKey)

	secondRandomUserKey, _ := crypto.GenerateKey()
	secondRandomUserAddress := crypto.PubkeyToAddress(secondRandomUserKey.PublicKey)

	thirdRandomUserKey, _ := crypto.GenerateKey()
	thirdRandomUserAddress := crypto.PubkeyToAddress(thirdRandomUserKey.PublicKey)

	//Transactions by first, second and third user with different staking amounts.
	tx1, _ := types.SignTx(types.NewTransaction(0, firstRandomUserAddress, 0, big.NewInt(10), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	tx2, _ := types.SignTx(types.NewTransaction(1, secondRandomUserAddress, 0, big.NewInt(20), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	tx3, _ := types.SignTx(types.NewTransaction(2, thirdRandomUserAddress, 0, big.NewInt(30), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	pendingTxs = append(pendingTxs, tx1)
	pendingTxs = append(pendingTxs, tx2)
	pendingTxs = append(pendingTxs, tx3)
	//tx4, _ := types.SignTx(types.NewTransaction(1, testUserAddress, 0, big.NewInt(1000), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	//newTxs = append(newTxs, tx4)
}

type testWorkerBackend struct {
	db     ethdb.Database
	txPool *core.TxPool
	chain  *core.BlockChain
}

func main() {
	fmt.Println("--------- Setting up faucet txns ---------")

	//** COULD WE UNDERSTAND THESE SETUP FUNCTIONS.
	var (
		database = ethdb.NewMemDatabase()
		gspec    = core.Genesis{
			Config:  chainConfig,
			Alloc:   core.GenesisAlloc{FaucetAddress: {Balance: FaucetInitFunds}},
			ShardID: 0,
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
			gen.SetCoinbase(FaucetAddress)
			gen.SetShardID(types.EncodeShardID(0))
			gen.AddTx(pendingTxs[i])
		})
		if _, err := chain.InsertChain(blocks); err != nil {
			log.Fatal(err)
		}
	}

	// Setup Faucet Contract  //

	NumTxns := 3

	var txs []*types.Transaction
	fworker := pkgworker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(FaucetPriKey.PublicKey), 0)
	nonce := fworker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(FaucetPriKey.PublicKey))

	dataEnc := common.FromHex(FaucetContractBinary)
	ftx, _ := types.SignTx(types.NewContractCreation(nonce, 0, big.NewInt(7000000000000000000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, FaucetPriKey)
	// Figure out the contract address which is determined by sender.address and nonce
	FaucetContractAddress := crypto.CreateAddress(FaucetAddress, nonce)
	state := fworker.GetCurrentState()
	txs = append(txs, ftx)

	nonce = fworker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(FaucetPriKey.PublicKey))
	var AllRandomUserAddress []common.Address
	var AllRandomUserKey []*ecdsa.PrivateKey

	//**** WHY DO WE DO THESE TRANSACTIONS? THOUGH THEY ARE THE ONLY ONES THAT FUND THESE ACCOUNTS?
	for i := 0; i < 3; i++ {
		randomUserKey, _ := crypto.GenerateKey()
		randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
		amount := i*1000 + 37 //Put different amount in each  account.
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i+1), randomUserAddress, 0, big.NewInt(int64(amount)), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
		AllRandomUserAddress = append(AllRandomUserAddress, randomUserAddress)
		AllRandomUserKey = append(AllRandomUserKey, randomUserKey)
		txs = append(txs, tx)
	}

	//****
	// Before the contract is deployed the code is empty

	fmt.Println(state.GetCodeHash(FaucetContractAddress))
	err := fworker.CommitTransactions(txs)

	if err != nil {
		fmt.Println(err)
	}
	block, _ := fworker.Commit()
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}

	receipts := fworker.GetCurrentReceipts()
	state = fworker.GetCurrentState()
	fmt.Println(receipts[len(receipts)-1].ContractAddress)
	fmt.Println(receipts[len(receipts)-1])
	fmt.Println("Faucet Address")
	fmt.Println(state.GetNonce(FaucetAddress))
	fmt.Println(FaucetPriKey)
	fmt.Println("Faucet Contract Address")
	fmt.Println(state.GetBalance(FaucetContractAddress))
	fmt.Println(state.GetBalance(AllRandomUserAddress[0]))
	fmt.Println("--------- Setting up faucet txns DONE ---------")

	// Send Faucet Contract Transaction ///
	fmt.Println("--------- Now Setting up faucet contract Call ---------")

	callData := "0x27c78c42000000000000000000000000" + AllRandomUserAddress[0].Hex() //24182601fe6e2e5da0b831496cc0489b7173b44f"
	callEnc := common.FromHex(callData)
	callfaucettx, _ := types.SignTx(types.NewTransaction(nonce+uint64(NumTxns+1), FaucetContractAddress, 0, big.NewInt(0), params.TxGasContractCreation*10, nil, callEnc), types.HomesteadSigner{}, FaucetPriKey)

	err = fworker.CommitTransactions(types.Transactions{callfaucettx})

	if err != nil {
		fmt.Println(err)
	}

	if err != nil {
		fmt.Println(err)
	}
	block, _ = fworker.Commit()
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}

	receipts = fworker.GetCurrentReceipts()
	fmt.Println(receipts[len(receipts)-1].ContractAddress)
	fmt.Println(receipts[len(receipts)-1])
	fmt.Println(state.GetNonce(FaucetAddress))
	fmt.Println(state.GetBalance(FaucetContractAddress))
	fmt.Println(state.GetBalance(AllRandomUserAddress[0]))
	fmt.Println(state.GetBalance(AllRandomUserAddress[1]))
	fmt.Println(state.GetBalance(AllRandomUserAddress[2]))
	fmt.Println(state.GetCodeHash(FaucetContractAddress))

	fmt.Println("--------- Setting up faucet Call DONE ---------")

	// Setup Staking Contract and Deposit Transactions //
	fmt.Println("--------- Now Setting up Staking Contract ---------")
	// //Add a staking contract deployment transaction.

	NumTxns = 3

	txs = make([]*types.Transaction, NumTxns)
	worker := pkgworker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(StakingPriKey.PublicKey), 0)
	nonce = worker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(StakingPriKey.PublicKey))
	dataEnc = common.FromHex(StakingContractBinary)
	stx, _ := types.SignTx(types.NewContractCreation(nonce, 0, big.NewInt(7000000000000000000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, StakingPriKey)
	// // // Figure out the contract address which is determined by sender.address and nonce
	StakeContractAddress := crypto.CreateAddress(StakingAddress, nonce+uint64(0))
	state = worker.GetCurrentState()

	var stakingtxns []*types.Transaction
	stakingtxns = append(stakingtxns, stx)
	for i := 0; i <= 2; i++ {
		stake := i*10 + 17
		callingFunction := "0xd0e30db0"
		dataEnc := common.FromHex(callingFunction) //Deposit Does not take a argument, stake is transferred via amount.
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i), StakeContractAddress, 0, big.NewInt(int64(stake)), params.TxGas, nil, dataEnc), types.HomesteadSigner{}, AllRandomUserKey[i])
		stakingtxns = append(stakingtxns, tx)
	}

	fmt.Println(state.GetCodeHash(StakeContractAddress))

	err = worker.CommitTransactions(stakingtxns)
	if err != nil {
		fmt.Println(err)
	}
	block, _ = worker.Commit()
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(StakeContractAddress)
	state = worker.GetCurrentState()
	receipts = worker.GetCurrentReceipts()

	fmt.Println(receipts)
	fmt.Println(state.GetNonce(StakingAddress))
	fmt.Println(state.GetBalance(AllRandomUserAddress[0]))
	fmt.Println(state.GetBalance(AllRandomUserAddress[1]))
	fmt.Println(state.GetBalance(AllRandomUserAddress[2]))
	fmt.Println(state.GetCodeHash(StakeContractAddress))
}
