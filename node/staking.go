package node

import (
	"crypto/ecdsa"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/internal/utils"
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
	node.CurrentStakesByNode = make(map[[20]byte]*structs.StakeInfo)
	utils.GetLogInstance().Info("UpdateStakingList",
		"numStakeInfos", len(stakeInfoReturnValue.LockedAddresses))
	for i, addr := range stakeInfoReturnValue.LockedAddresses {
		blockNum := stakeInfoReturnValue.BlockNums[i]
		blsAddr := stakeInfoReturnValue.BlsAddresses[i]
		lockPeriodCount := stakeInfoReturnValue.LockPeriodCounts[i]
		startEpoch := core.GetEpochFromBlockNumber(blockNum.Uint64())
		curEpoch := core.GetEpochFromBlockNumber(node.blockchain.CurrentBlock().NumberU64())

		if startEpoch >= curEpoch {
			continue // The token are counted into stakes at the beginning of next epoch.
		}
		// True if the token is still staked within the locking period.
		if curEpoch-startEpoch <= lockPeriodCount.Uint64()*lockPeriodInEpochs {
			stakeInfo := &structs.StakeInfo{
				addr,
				blsAddr,
				blockNum,
				lockPeriodCount,
				stakeInfoReturnValue.Amounts[i],
			}
			node.CurrentStakes[addr] = stakeInfo
			node.CurrentStakesByNode[blsAddr] = stakeInfo
		}
	}
}

// UpdateStakingListWithInitShardState updates the current stakes list with
// initial hardcoded stakes.
//
// Initial stakers are assumed to be active since ever and until forever,
// but with zero stake.
//
// TODO ek – this is a hack,
//  until we can streamline initial stakers without hardcoded accounts.
func (node *Node) UpdateStakingListWithInitShardState() {
	initShardState := core.GetInitShardState()
	shardID := node.Consensus.ShardID
	if shardID >= uint32(len(initShardState)) {
		utils.GetLogInstance().Debug("UpdateStakingListWithInitShardState: "+
			"shard did not exist initially, nothing to do",
			"shardID", shardID)
		return
	}
	initCommittee := initShardState[shardID]
	utils.GetLogInstance().Debug("UpdateStakingListWithInitShardState: "+
		"adding initial shard members",
		"shardID", shardID,
		"committeeSize", len(initCommittee.NodeList))
	for _, member := range initCommittee.NodeList {
		ecdsaAddress := common.HexToAddress(member.EcdsaAddress)
		blsAddress := common.HexToAddress(member.BlsAddress)
		stakeInfo := &structs.StakeInfo{
			Address:         ecdsaAddress,
			BlsAddress:      blsAddress,
			BlockNum:        big.NewInt(0),             // since ever
			LockPeriodCount: big.NewInt(math.MaxInt64), // until forever
			Amount:          big.NewInt(0),             // TODO ek
		}
		node.CurrentStakes[ecdsaAddress] = stakeInfo
		node.CurrentStakesByNode[blsAddress] = stakeInfo
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
