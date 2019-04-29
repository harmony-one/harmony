package node

import (
	"crypto/ecdsa"
	"math/big"
	"os"

	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/contracts/structs"

	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

// UpdateStakingList updates staking list from the given StakeInfo query result.
func (node *Node) UpdateStakingList(stakeInfoReturnValue *structs.StakeInfoReturnValue) {
	if stakeInfoReturnValue == nil {
		return
	}
	node.CurrentStakes = make(map[common.Address]*structs.StakeInfo)
	for i, addr := range stakeInfoReturnValue.LockedAddresses {
		blockNum := stakeInfoReturnValue.BlockNums[i]
		lockPeriodCount := stakeInfoReturnValue.LockPeriodCounts[i]

		startEpoch := core.GetEpochFromBlockNumber(blockNum.Uint64())
		curEpoch := core.GetEpochFromBlockNumber(node.blockchain.CurrentBlock().NumberU64())

		if startEpoch == curEpoch {
			continue // The token are counted into stakes at the beginning of next epoch.
		}
		// True if the token is still staked within the locking period.
		if curEpoch-startEpoch <= lockPeriodCount.Uint64()*lockPeriodInEpochs {
			blsPubKey := types.BlsPublicKey{}
			copy(blsPubKey[:32], stakeInfoReturnValue.BlsPubicKeys1[i][:])
			copy(blsPubKey[32:64], stakeInfoReturnValue.BlsPubicKeys2[i][:])
			copy(blsPubKey[64:96], stakeInfoReturnValue.BlsPubicKeys2[i][:])
			node.CurrentStakes[addr] = &structs.StakeInfo{
				Account:         addr,
				BlsPublicKey:    blsPubKey,
				BlockNum:        blockNum,
				LockPeriodCount: lockPeriodCount,
				Amount:          stakeInfoReturnValue.Amounts[i],
			}
		}
	}
}

func (node *Node) printStakingList() {
	utils.GetLogInstance().Info("\n")
	utils.GetLogInstance().Info("CURRENT STAKING INFO [START] ------------------------------------")
	for addr, stakeInfo := range node.CurrentStakes {
		utils.GetLogInstance().Info("", "Address", addr, "StakeInfo", stakeInfo)
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

// StoreStakingKeyFromFile load the staking private key and store it in local keyfile
func StoreStakingKeyFromFile(keyfile string, priKey string) *ecdsa.PrivateKey {
	// contract.FakeAccounts[0] gets minted tokens in genesis block of beacon chain.
	key, err := crypto.HexToECDSA(priKey)
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
