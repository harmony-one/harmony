package node

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
	"golang.org/x/crypto/sha3"
)

func TestUpdateStakingDeposit(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", BlsPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)

	node := New(host, consensus, nil)
	node.CurrentStakes = make(map[common.Address]int64)

	DepositContractPriKey, _ := crypto.GenerateKey()                                  //DepositContractPriKey is pk for contract
	DepositContractAddress := crypto.PubkeyToAddress(DepositContractPriKey.PublicKey) //DepositContractAddress is the address for the contract
	node.StakingContractAddress = DepositContractAddress
	node.AccountKey, _ = crypto.GenerateKey()
	Address := crypto.PubkeyToAddress(node.AccountKey.PublicKey)
	callingFunction := "0xd0e30db0"
	amount := new(big.Int)
	amount.SetString("10", 10)
	dataEnc := common.FromHex(callingFunction) //Deposit Does not take a argument, stake is transferred via amount.
	tx1, err := types.SignTx(types.NewTransaction(0, DepositContractAddress, node.Consensus.ShardID, amount, params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.AccountKey)

	var txs []*types.Transaction
	txs = append(txs, tx1)
	header := &types.Header{Extra: []byte("hello")}
	block := types.NewBlock(header, txs, nil)
	node.UpdateStakingList(block)
	if len(node.CurrentStakes) == 0 {
		t.Error("New node's stake was not added")
	}
	value, ok := node.CurrentStakes[Address]
	if !ok {
		t.Error("The correct address was not added")
	}
	if value != 10 {
		t.Error("The correct stake value was not added")
	}
}

func TestUpdateStakingWithdrawal(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8882", BlsPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "8885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)

	node := New(host, consensus, nil)
	node.CurrentStakes = make(map[common.Address]int64)

	DepositContractPriKey, _ := crypto.GenerateKey()                                  //DepositContractPriKey is pk for contract
	DepositContractAddress := crypto.PubkeyToAddress(DepositContractPriKey.PublicKey) //DepositContractAddress is the address for the contract
	node.StakingContractAddress = DepositContractAddress
	node.AccountKey, _ = crypto.GenerateKey()
	Address := crypto.PubkeyToAddress(node.AccountKey.PublicKey)
	initialStake := int64(1010)
	node.CurrentStakes[Address] = initialStake //initial stake

	withdrawFnSignature := []byte("withdraw(uint256)")
	hash := sha3.NewLegacyKeccak256()
	hash.Write(withdrawFnSignature)
	methodID := hash.Sum(nil)[:4]

	amount := "10"
	stakeToWithdraw := new(big.Int)
	stakeToWithdraw.SetString(amount, 10)
	paddedAmount := common.LeftPadBytes(stakeToWithdraw.Bytes(), 32)

	remainingStakeShouldBe := initialStake - stakeToWithdraw.Int64()

	var dataEnc []byte
	dataEnc = append(dataEnc, methodID...)
	dataEnc = append(dataEnc, paddedAmount...)
	tx, err := types.SignTx(types.NewTransaction(0, DepositContractAddress, node.Consensus.ShardID, big.NewInt(0), params.TxGasContractCreation*10, nil, dataEnc), types.HomesteadSigner{}, node.AccountKey)

	var txs []*types.Transaction
	txs = append(txs, tx)
	header := &types.Header{Extra: []byte("hello")}
	block := types.NewBlock(header, txs, nil)
	node.UpdateStakingList(block)
	currentStake, ok := node.CurrentStakes[Address]
	if !ok {
		t.Error("The correct address was not present")
	}
	if currentStake != remainingStakeShouldBe {
		t.Error("The correct stake value was not subtracted")
	}
}
