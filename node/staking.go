package node

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/core/types"
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
			continue
		}
		currentSender, _ := types.Sender(signerType, txn)
		_, isPresent := node.CurrentStakes[currentSender]
		data := txn.Data()
		switch funcSignature := decodeFuncSign(data); funcSignature {
		case depositFuncSignature: //deposit, currently: 0xd0e30db0
			amount := txn.Value()
			value := amount.Int64()
			if isPresent {
				//This means the node has increased its stake.
				node.CurrentStakes[currentSender] += value
			} else {
				//This means its a new node that is staking the first time.
				node.CurrentStakes[currentSender] = value
			}
		case withdrawFuncSignature: //withdaw, currently: 0x2e1a7d4d
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
