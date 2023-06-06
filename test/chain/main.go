package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/harmony-one/harmony/core/rawdb"
	chain2 "github.com/harmony-one/harmony/test/chain/chain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/core"
	core_state "github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/internal/params"
	pkgworker "github.com/harmony-one/harmony/node/worker"
)

const (
	// FaucetContractBinary is binary for faucet contract.
	FaucetContractBinary = "0x60806040526706f05b59d3b2000060015560028054600160a060020a031916331790556101aa806100316000396000f3fe608060405260043610610045577c0100000000000000000000000000000000000000000000000000000000600035046327c78c42811461004a578063b69ef8a81461008c575b600080fd5b34801561005657600080fd5b5061008a6004803603602081101561006d57600080fd5b503573ffffffffffffffffffffffffffffffffffffffff166100b3565b005b34801561009857600080fd5b506100a1610179565b60408051918252519081900360200190f35b60025473ffffffffffffffffffffffffffffffffffffffff1633146100d757600080fd5b600154303110156100e757600080fd5b73ffffffffffffffffffffffffffffffffffffffff811660009081526020819052604090205460ff161561011a57600080fd5b73ffffffffffffffffffffffffffffffffffffffff8116600081815260208190526040808220805460ff1916600190811790915554905181156108fc0292818181858888f19350505050158015610175573d6000803e3d6000fd5b5050565b30319056fea165627a7a723058206b894c1f3badf3b26a7a2768ab8141b1e6fa1c1ddc4622f4f44a7d5041edc9350029"
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
	chainConfig     = params.TestChainConfig
	blockFactory    = blockfactory.ForTest
	// Test transactions
	pendingTxs []types.PoolTransaction
	database   = rawdb.NewMemoryDatabase()
	gspec      = core.Genesis{
		Config:  chainConfig,
		Factory: blockFactory,
		Alloc:   core.GenesisAlloc{FaucetAddress: {Balance: FaucetInitFunds}},
		ShardID: 0,
	}
	txs                   []*types.Transaction
	contractworker        *pkgworker.Worker
	nonce                 uint64
	dataEnc               []byte
	allRandomUserAddress  []common.Address
	allRandomUserKey      []*ecdsa.PrivateKey
	faucetContractAddress common.Address
	chain                 core.BlockChain
	err                   error
	state                 *core_state.DB
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
	chain  *core.BlockChainImpl
}

func fundFaucetContract(chain core.BlockChain) {
	fmt.Println()
	fmt.Println("--------- Funding addresses for Faucet Contract Call ---------")
	fmt.Println()

	contractworker = pkgworker.New(params.TestChainConfig, chain, nil, chain.Engine())
	nonce = contractworker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(FaucetPriKey.PublicKey))
	dataEnc = common.FromHex(FaucetContractBinary)
	ftx, _ := types.SignTx(
		types.NewContractCreation(
			nonce, 0, big.NewInt(7000000000000000000),
			params.TxGasContractCreation*10, nil, dataEnc),
		types.HomesteadSigner{}, FaucetPriKey,
	)
	faucetContractAddress = crypto.CreateAddress(FaucetAddress, nonce)
	txs = append(txs, ftx)
	// Funding the user addressed
	for i := 1; i <= 3; i++ {
		randomUserKey, _ := crypto.GenerateKey()
		randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
		amount := i*100000 + 37 // Put different amount in each  account.
		tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(i), randomUserAddress, 0, big.NewInt(int64(amount)), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
		allRandomUserAddress = append(allRandomUserAddress, randomUserAddress)
		allRandomUserKey = append(allRandomUserKey, randomUserKey)
		txs = append(txs, tx)
	}
	amount := 720000
	randomUserKey, _ := crypto.GenerateKey()
	randomUserAddress := crypto.PubkeyToAddress(randomUserKey.PublicKey)
	tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(4), randomUserAddress, 0, big.NewInt(int64(amount)), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	txs = append(txs, tx)

	txmap := make(map[common.Address]types.Transactions)
	txmap[FaucetAddress] = txs
	err := contractworker.CommitTransactions(
		txmap, nil, testUserAddress,
	)
	if err != nil {
		fmt.Println(err)
	}
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()
	block, _ := contractworker.
		FinalizeNewBlock(commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil)
	_, err = chain.InsertChain(types.Blocks{block}, true /* verifyHeaders */)
	if err != nil {
		fmt.Println(err)
	}

	state = contractworker.GetCurrentState()
	fmt.Println("Balances before call of faucet contract")
	fmt.Println("contract balance:")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println()
	fmt.Println("--------- Funding addresses for Faucet Contract Call DONE ---------")
}

func callFaucetContractToFundAnAddress(chain core.BlockChain) {
	// Send Faucet Contract Transaction ///
	fmt.Println("--------- Now Setting up Faucet Contract Call ---------")
	fmt.Println()

	paddedAddress := common.LeftPadBytes(allRandomUserAddress[0].Bytes(), 32)
	transferFnSignature := []byte("request(address)")

	fnHash := hash.Keccak256(transferFnSignature)
	callFuncHex := fnHash[:4]

	var callEnc []byte
	callEnc = append(callEnc, callFuncHex...)
	callEnc = append(callEnc, paddedAddress...)
	callfaucettx, _ := types.SignTx(types.NewTransaction(nonce+uint64(5), faucetContractAddress, 0, big.NewInt(0), params.TxGasContractCreation*10, nil, callEnc), types.HomesteadSigner{}, FaucetPriKey)

	txmap := make(map[common.Address]types.Transactions)
	txmap[FaucetAddress] = types.Transactions{callfaucettx}

	err = contractworker.CommitTransactions(
		txmap, nil, testUserAddress,
	)
	if err != nil {
		fmt.Println(err)
	}
	if err != nil {
		fmt.Println(err)
	}
	commitSigs := make(chan []byte)
	go func() {
		commitSigs <- []byte{}
	}()
	block, _ := contractworker.FinalizeNewBlock(
		commitSigs, func() uint64 { return 0 }, common.Address{}, nil, nil,
	)
	_, err = chain.InsertChain(types.Blocks{block}, true /* verifyHeaders */)
	if err != nil {
		fmt.Println(err)
	}
	state = contractworker.GetCurrentState()
	fmt.Println("Balances AFTER CALL of faucet contract")
	fmt.Println("contract balance:")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println()
	fmt.Println("--------- Faucet Contract Call DONE ---------")
}

func playFaucetContract(chain core.BlockChain) {
	fundFaucetContract(chain)
	callFaucetContractToFundAnAddress(chain)
}

func main() {
	genesis := gspec.MustCommit(database)
	chain, _ := core.NewBlockChain(database, nil, nil, nil, gspec.Config, chain.Engine(), vm.Config{})
	txpool := core.NewTxPool(core.DefaultTxPoolConfig, chainConfig, chain, types.NewTransactionErrorSink())

	backend := &testWorkerBackend{
		db:     database,
		chain:  chain,
		txPool: txpool,
	}
	poolPendingTx := types.PoolTransactions{}
	for _, tx := range pendingTxs {
		poolPendingTx = append(poolPendingTx, tx.(types.PoolTransaction))
	}
	backend.txPool.AddLocals(poolPendingTx)

	//// Generate a small n-block chain and an uncle block for it
	n := 3
	if n > 0 {
		blocks, _ := chain2.GenerateChain(chainConfig, genesis.Header(), chain.Engine(), database, n, func(i int, gen *chain2.BlockGen) {
			gen.SetCoinbase(FaucetAddress)
			gen.SetShardID(0)
			gen.AddTx(pendingTxs[i].(*types.Transaction))
		})
		if _, err := chain.InsertChain(blocks, true /* verifyHeaders */); err != nil {
			log.Fatal(err)
		}
	}

	playFaucetContract(chain)
}
