package node

import (
	"crypto/ecdsa"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/internal/utils/contract"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/types"
)

//constants related to staking
//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html

const (
	// DepositFuncSignature is the func signature for deposit method
	DepositFuncSignature  = "0xd0e30db0"
	withdrawFuncSignature = "0x2e1a7d4d"
	funcSingatureBytes    = 4
)

// UpdateStakingList updates the stakes of every node.
// TODO: read directly from smart contract, or at least check the receipt also for incompleted transaction.
func (node *Node) UpdateStakingList(block *types.Block) error {
	signerType := types.HomesteadSigner{}
	txns := block.Transactions()
	for i := range txns {
		txn := txns[i]
		toAddress := txn.To()
		if toAddress != nil && *toAddress != node.StakingContractAddress { //Not a address aimed at the staking contract.
			utils.GetLogInstance().Info("Mismatched Staking Contract Address", "expected", node.StakingContractAddress.Hex(), "got", toAddress.Hex())
			continue
		}
		currentSender, _ := types.Sender(signerType, txn)
		_, isPresent := node.CurrentStakes[currentSender]
		data := txn.Data()
		switch funcSignature := decodeFuncSign(data); funcSignature {
		case DepositFuncSignature: //deposit, currently: 0xd0e30db0
			amount := txn.Value()
			value := amount.Int64()
			if isPresent {
				//This means the node has increased its stake.
				node.CurrentStakes[currentSender] += value
			} else {
				//This means its a new node that is staking the first time.
				node.CurrentStakes[currentSender] = value
			}
		case withdrawFuncSignature: //withdraw, currently: 0x2e1a7d4d
			value := decodeStakeCall(data)
			if isPresent {
				if node.CurrentStakes[currentSender] > value {
					node.CurrentStakes[currentSender] -= value
				} else if node.CurrentStakes[currentSender] == value {
					delete(node.CurrentStakes, currentSender)
				} else {
					continue //Overdraft protection.
				}
			} else {
				continue //no-op: a node that is not staked cannot withdraw stake.
			}
		default:
			continue //no-op if its not deposit or withdaw
		}
	}
	return nil
}

func (node *Node) printStakingList() {
	utils.GetLogInstance().Info("\n")
	utils.GetLogInstance().Info("CURRENT STAKING INFO [START] ------------------------------------")
	for addr, stake := range node.CurrentStakes {
		utils.GetLogInstance().Info("", "Address", addr, "Stake", stake)
	}
	utils.GetLogInstance().Info("CURRENT STAKING INFO [END}   ------------------------------------")
	utils.GetLogInstance().Info("\n")
}

//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html
func decodeStakeCall(getData []byte) int64 {
	value := new(big.Int)
	value.SetBytes(getData[funcSingatureBytes:]) //Escape the method call.
	return value.Int64()
}

//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html
//gets the function signature from data.
func decodeFuncSign(data []byte) string {
	funcSign := hexutil.Encode(data[:funcSingatureBytes]) //The function signature is first 4 bytes of data in ethereum
	return funcSign
}

// LoadStakingKeyFromFile load staking private key from keyfile
// If the private key is not loadable or no file, it will generate
// a new random private key
// Currently for deploy_newnode.sh, we hard-coded the first fake account as staking account and
// it is minted in genesis block. See genesis_node.go
func LoadStakingKeyFromFile(keyfile string) *ecdsa.PrivateKey {
	// contract.FakeAccounts[0] gets minted tokens in genesis block of beacon chain.
	key, err := crypto.HexToECDSA(contract.FakeAccounts[0].Private)
	if err != nil {
		utils.GetLogInstance().Error("Unable to get staking key")
		os.Exit(1)
	}
	if err := crypto.SaveECDSA(keyfile, key); err != nil {
		utils.GetLogInstance().Error("Unable to save the private key", "error", err)
		os.Exit(1)
	}
	// TODO(minhdoan): Enable this back.
	// key, err := crypto.LoadECDSA(keyfile)
	// if err != nil {
	// 	GetLogInstance().Error("no key file. Let's create a staking private key")
	// 	key, err = crypto.GenerateKey()
	// 	if err != nil {
	// 		GetLogInstance().Error("Unable to generate the private key")
	// 		os.Exit(1)
	// 	}
	// 	if err = crypto.SaveECDSA(keyfile, key); err != nil {
	// 		GetLogInstance().Error("Unable to save the private key", "error", err)
	// 		os.Exit(1)
	// 	}
	// }
	return key
}
