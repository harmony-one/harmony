package main

import (
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/crypto/hash"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core"
	core_state "github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	pkgworker "github.com/harmony-one/harmony/node/worker"
)

const (
	//StakingContractBinary is binary for staking contract.
	StakingContractBinary = "0x608060405234801561001057600080fd5b506103f7806100206000396000f3fe608060405260043610610067576000357c01000000000000000000000000000000000000000000000000000000009004806317437a2c1461006c5780632e1a7d4d146100975780638da5cb5b146100e6578063b69ef8a81461013d578063d0e30db014610168575b600080fd5b34801561007857600080fd5b50610081610186565b6040518082815260200191505060405180910390f35b3480156100a357600080fd5b506100d0600480360360208110156100ba57600080fd5b81019080803590602001909291905050506101a5565b6040518082815260200191505060405180910390f35b3480156100f257600080fd5b506100fb6102cd565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34801561014957600080fd5b506101526102f3565b6040518082815260200191505060405180910390f35b610170610339565b6040518082815260200191505060405180910390f35b60003073ffffffffffffffffffffffffffffffffffffffff1631905090565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054821115156102c757816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055503373ffffffffffffffffffffffffffffffffffffffff166108fc839081150290604051600060405180830381858888f19350505050158015610280573d6000803e3d6000fd5b506000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490506102c8565b5b919050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b60008060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905090565b6000346000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825401925050819055506000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490509056fea165627a7a723058204acf95662eb95006df1e0b8ba32316211039c7872bc6eb99d12689c1624143d80029"
	//FaucetContractBinary is binary for faucet contract.
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

	chainConfig = params.TestChainConfig

	//StakingPriKey is the keys for the deposit contract.
	StakingPriKey, _ = crypto.GenerateKey()
	//StakingAddress is the address of the deposit contract.
	StakingAddress = crypto.PubkeyToAddress(StakingPriKey.PublicKey)

	// Test transactions
	pendingTxs []*types.Transaction
	newTxs     []*types.Transaction

	database = ethdb.NewMemDatabase()
	gspec    = core.Genesis{
		Config:  chainConfig,
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
	stakeContractAddress  common.Address
	chain                 *core.BlockChain
	err                   error
	block                 *types.Block
	state                 *core_state.DB
	stakingtxns           []*types.Transaction
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

func fundFaucetContract(chain *core.BlockChain) {
	fmt.Println()
	fmt.Println("--------- Funding addresses for Faucet Contract Call ---------")
	fmt.Println()

	contractworker = pkgworker.New(params.TestChainConfig, chain, consensus.NewFaker(), 0)
	nonce = contractworker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(FaucetPriKey.PublicKey))
	dataEnc = common.FromHex(FaucetContractBinary)
	ftx, _ := types.SignTx(types.NewContractCreation(nonce, 0, big.NewInt(7000000000000000000), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, FaucetPriKey)
	faucetContractAddress = crypto.CreateAddress(FaucetAddress, nonce)
	state := contractworker.GetCurrentState()
	txs = append(txs, ftx)
	//Funding the user addressed
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
	tx, _ := types.SignTx(types.NewTransaction(nonce+uint64(4), StakingAddress, 0, big.NewInt(int64(amount)), params.TxGas, nil, nil), types.HomesteadSigner{}, FaucetPriKey)
	txs = append(txs, tx)
	err := contractworker.CommitTransactions(txs, testUserAddress)
	if err != nil {
		fmt.Println(err)
	}
	block, _ := contractworker.Commit([]byte{}, []byte{}, 0, common.Address{})
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}

	state = contractworker.GetCurrentState()
	fmt.Println("Balances before call of faucet contract")
	fmt.Println("contract balance:")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println("staking address balance")
	fmt.Println(state.GetBalance(StakingAddress))
	fmt.Println()
	fmt.Println("--------- Funding addresses for Faucet Contract Call DONE ---------")
}

func callFaucetContractToFundAnAddress(chain *core.BlockChain) {
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

	err = contractworker.CommitTransactions(types.Transactions{callfaucettx}, testUserAddress)
	if err != nil {
		fmt.Println(err)
	}
	if err != nil {
		fmt.Println(err)
	}
	block, _ := contractworker.Commit([]byte{}, []byte{}, 0, common.Address{})
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}
	state = contractworker.GetCurrentState()
	fmt.Println("Balances AFTER CALL of faucet contract")
	fmt.Println("contract balance:")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println("staking address balance")
	fmt.Println(state.GetBalance(StakingAddress))
	fmt.Println()
	fmt.Println("--------- Faucet Contract Call DONE ---------")
}

func playFaucetContract(chain *core.BlockChain) {
	fundFaucetContract(chain)
	callFaucetContractToFundAnAddress(chain)
}

func playSetupStakingContract(chain *core.BlockChain) {
	fmt.Println()
	fmt.Println("--------- ************************** ---------")
	fmt.Println()
	fmt.Println("--------- Now Setting up Staking Contract ---------")
	fmt.Println()
	//worker := pkgworker.New(params.TestChainConfig, chain, consensus.NewFaker(), crypto.PubkeyToAddress(StakingPriKey.PublicKey), 0)

	state = contractworker.GetCurrentState()
	fmt.Println("Before Staking Balances")
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println("The balances for 2 more users:")
	fmt.Println(state.GetBalance(allRandomUserAddress[1]))
	fmt.Println(state.GetBalance(allRandomUserAddress[2]))

	nonce = contractworker.GetCurrentState().GetNonce(crypto.PubkeyToAddress(StakingPriKey.PublicKey))
	dataEnc = common.FromHex(StakingContractBinary)
	stx, _ := types.SignTx(types.NewContractCreation(nonce, 0, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, StakingPriKey)
	stakingtxns = append(stakingtxns, stx)

	stakeContractAddress = crypto.CreateAddress(StakingAddress, nonce+uint64(0))

	state = contractworker.GetCurrentState()
	fmt.Println("stake contract balance :")
	fmt.Println(state.GetBalance(stakeContractAddress))
	fmt.Println("stake address balance :")
	fmt.Println(state.GetBalance(StakingAddress))
}

func playStaking(chain *core.BlockChain) {
	depositFnSignature := []byte("deposit()")

	fnHash := hash.Keccak256(depositFnSignature)
	methodID := fnHash[:4]

	var callEncl []byte
	callEncl = append(callEncl, methodID...)
	stake := 100000
	fmt.Println()
	fmt.Println("--------- Staking Contract added to txns ---------")
	fmt.Println()
	fmt.Printf("-- Now Staking with stake: %d --\n", stake)
	fmt.Println()
	for i := 0; i <= 2; i++ {
		//Deposit Does not take a argument, stake is transferred via amount.
		tx, _ := types.SignTx(types.NewTransaction(0, stakeContractAddress, 0, big.NewInt(int64(stake)), params.TxGas*5, nil, callEncl), types.HomesteadSigner{}, allRandomUserKey[i])
		stakingtxns = append(stakingtxns, tx)
	}
	err = contractworker.CommitTransactions(stakingtxns, common.Address{})

	if err != nil {
		fmt.Println(err)
	}
	block, _ := contractworker.Commit([]byte{}, []byte{}, 0, common.Address{})
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}
	// receipts := contractworker.GetCurrentReceipts()
	// fmt.Println(receipts[len(receipts)-4].ContractAddress)
	state = contractworker.GetCurrentState()

	fmt.Printf("After Staking Balances (should be less by %d)\n", stake)
	fmt.Println("user address balance")
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println("The balances for 2 more users:")
	fmt.Println(state.GetBalance(allRandomUserAddress[1]))
	fmt.Println(state.GetBalance(allRandomUserAddress[2]))
	fmt.Println("faucet contract balance (unchanged):")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Println("stake contract balance :")
	fmt.Println(state.GetBalance(stakeContractAddress))
	fmt.Println("stake address balance :")
	fmt.Println(state.GetBalance(StakingAddress))
}

func playWithdrawStaking(chain *core.BlockChain) {
	fmt.Println()
	fmt.Println("--------- Now Setting up Withdrawing Stakes ---------")

	withdrawFnSignature := []byte("withdraw(uint256)")

	fnHash := hash.Keccak256(withdrawFnSignature)
	methodID := fnHash[:4]

	withdraw := "5000"
	withdrawstake := new(big.Int)
	withdrawstake.SetString(withdraw, 10)
	paddedAmount := common.LeftPadBytes(withdrawstake.Bytes(), 32)

	var dataEncl []byte
	dataEncl = append(dataEncl, methodID...)
	dataEncl = append(dataEncl, paddedAmount...)

	var withdrawstakingtxns []*types.Transaction

	fmt.Println()
	fmt.Printf("-- Withdrawing Stake by amount: %s --\n", withdraw)
	fmt.Println()

	for i := 0; i <= 2; i++ {
		cnonce := contractworker.GetCurrentState().GetNonce(allRandomUserAddress[i])
		tx, _ := types.SignTx(types.NewTransaction(cnonce, stakeContractAddress, 0, big.NewInt(0), params.TxGas*5, nil, dataEncl), types.HomesteadSigner{}, allRandomUserKey[i])
		withdrawstakingtxns = append(withdrawstakingtxns, tx)
	}

	err = contractworker.CommitTransactions(withdrawstakingtxns, common.Address{})
	if err != nil {
		fmt.Println("error:")
		fmt.Println(err)
	}

	block, _ := contractworker.Commit([]byte{}, []byte{}, 0, common.Address{})
	_, err = chain.InsertChain(types.Blocks{block})
	if err != nil {
		fmt.Println(err)
	}
	state = contractworker.GetCurrentState()
	fmt.Printf("Withdraw Staking Balances (should be up by %s)\n", withdraw)
	fmt.Println(state.GetBalance(allRandomUserAddress[0]))
	fmt.Println(state.GetBalance(allRandomUserAddress[1]))
	fmt.Println(state.GetBalance(allRandomUserAddress[2]))
	fmt.Println("faucet contract balance (unchanged):")
	fmt.Println(state.GetBalance(faucetContractAddress))
	fmt.Printf("stake contract balance (should downup by %s)\n", withdraw)
	fmt.Println(state.GetBalance(stakeContractAddress))
	fmt.Println("stake address balance :")
	fmt.Println(state.GetBalance(StakingAddress))
}

func playStakingContract(chain *core.BlockChain) {
	playSetupStakingContract(chain)
	playStaking(chain)
	playWithdrawStaking(chain)
}

func main() {
	genesis := gspec.MustCommit(database)
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
			gen.SetShardID(0)
			gen.AddTx(pendingTxs[i])
		})
		if _, err := chain.InsertChain(blocks); err != nil {
			log.Fatal(err)
		}
	}

	playFaucetContract(chain)
	playStakingContract(chain)
}
