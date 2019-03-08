package node

import (
	"crypto/ecdsa"
	"math/big"
	"os"

	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/internal/utils/contract"

	"github.com/harmony-one/harmony/internal/utils"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

//constants related to staking
//The first four bytes of the call data for a function call specifies the function to be called.
//It is the first (left, high-order in big-endian) four bytes of the Keccak-256 (SHA-3)
//Refer: https://solidity.readthedocs.io/en/develop/abi-spec.html

const (
	funcSingatureBytes = 4
	lockPeriodInEpochs = 3 // This should be in sync with contracts/StakeLockContract.sol
)

// StakeInfo is the struct for the return value of listLockedAddresses func in stake contract.
type StakeInfo struct {
	LockedAddresses  []common.Address
	BlockNums        []*big.Int
	LockPeriodCounts []*big.Int  // The number of locking period the token will be locked.
	Amounts          []*big.Int
}

// UpdateStakingList updates staking information by querying the staking smart contract.
func (node *Node) UpdateStakingList(stakeInfo *StakeInfo) {
	node.CurrentStakes = make(map[common.Address]*big.Int)
	if stakeInfo != nil {
		for i, addr := range stakeInfo.LockedAddresses {
			blockNum := stakeInfo.BlockNums[i]
			lockPeriodCount := stakeInfo.LockPeriodCounts[i]
			amount := stakeInfo.Amounts[i]

			startEpoch := core.GetEpochFromBlockNumber(blockNum.Uint64())
			curEpoch := core.GetEpochFromBlockNumber(node.blockchain.CurrentBlock().NumberU64())

			if startEpoch == curEpoch {
				continue // The token are counted into stakes at the beginning of next epoch.
			}
			if curEpoch-startEpoch <= lockPeriodCount.Uint64()*lockPeriodInEpochs {
				node.CurrentStakes[addr] = amount
			}
		}
	}
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
func LoadStakingKeyFromFile(keyfile string, accountIndex int) *ecdsa.PrivateKey {
	// contract.FakeAccounts[0] gets minted tokens in genesis block of beacon chain.
	key, err := crypto.HexToECDSA(contract.StakingAccounts[accountIndex].Private)
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
