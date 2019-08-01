package node

import (
	"encoding/hex"
	"math/big"

	"github.com/harmony-one/harmony/core/types"

	"github.com/harmony-one/harmony/contracts/structs"

	"github.com/harmony-one/harmony/core"

	"github.com/ethereum/go-ethereum/common"
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
	utils.Logger().Str("contractState", stakeInfoReturnValue).Msg("Updating staking list")
	if stakeInfoReturnValue == nil {
		return
	}
	node.CurrentStakes = make(map[common.Address]*structs.StakeInfo)
	for i, addr := range stakeInfoReturnValue.LockedAddresses {
		blockNum := stakeInfoReturnValue.BlockNums[i]
		lockPeriodCount := stakeInfoReturnValue.LockPeriodCounts[i]

		startEpoch := core.GetEpochFromBlockNumber(blockNum.Uint64())
		curEpoch := core.GetEpochFromBlockNumber(node.Blockchain().CurrentBlock().NumberU64())

		if startEpoch == curEpoch {
			continue // The token are counted into stakes at the beginning of next epoch.
		}
		// True if the token is still staked within the locking period.
		if curEpoch-startEpoch <= lockPeriodCount.Uint64()*lockPeriodInEpochs {
			blsPubKey := types.BlsPublicKey{}
			copy(blsPubKey[:32], stakeInfoReturnValue.BlsPubicKeys1[i][:])
			copy(blsPubKey[32:48], stakeInfoReturnValue.BlsPubicKeys2[i][:16])
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
	utils.Logger().Info().Msg("\n")
	utils.Logger().Info().Msg("CURRENT STAKING INFO [START] ------------------------------------")
	for addr, stakeInfo := range node.CurrentStakes {
		utils.Logger().Info().
		Str("Address", addr).
		Str("BlsPubKey", hex.EncodeToString(stakeInfo.BlsPublicKey[:])).
		Uint64("BlockNum", stakeInfo.BlockNum).
		Int("lockPeriodCount", stakeInfo.LockPeriodCount).
		Int("amount", stakeInfo.Amount).
		Msg("")
	}
	utils.Logger().Info().Msg("CURRENT STAKING INFO [END}   ------------------------------------")
	utils.Logger().Info().Msg("\n")
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
