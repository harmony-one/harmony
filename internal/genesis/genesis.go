package genesis

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
)

const genesisString = "https://harmony.one 'Open Consensus for 10B' 2019.06.01 $ONE"

// DeployAccount is the account used in genesis
type DeployAccount struct {
	Index        string // index
	Address      string // account address
	BlsPublicKey string // account public BLS key
	ShardID      uint32 // shardID of the account
}

func (d DeployAccount) String() string {
	return fmt.Sprintf("%s/%s:%d", d.Address, d.BlsPublicKey, d.ShardID)
}

// BeaconAccountPriKey is the func which generates a constant private key.
func BeaconAccountPriKey() *ecdsa.PrivateKey {
	prikey, err := ecdsa.GenerateKey(crypto.S256(), strings.NewReader(genesisString))
	if err != nil && prikey == nil {
		utils.GetLogInstance().Error("Failed to generate beacon chain contract deployer account")
		os.Exit(111)
	}
	return prikey
}

// FindAccount find the DeployAccount based on the account address, and the account index
// the account address could be from HarmonyAccounts or from FoundationalNodeAccounts
// the account index can be used to determin the shard of the account
func FindAccount(address string) (int, *DeployAccount) {
	addr := common.ParseAddr(address)
	for i, acc := range HarmonyAccounts {
		if addr == common.ParseAddr(acc.Address) {
			return i, &acc
		}
	}
	for i, acc := range FoundationalNodeAccounts {
		if addr == common.ParseAddr(acc.Address) {
			return i + 8, &acc
		}
	}
	return 0, nil
}

// IsBlsPublicKeyIndex returns index and DeployAccount.
func IsBlsPublicKeyIndex(blsPublicKey string) (int, *DeployAccount) {
	for i, item := range HarmonyAccounts {
		if item.BlsPublicKey == blsPublicKey {
			return i, &item
		}
	}
	for i, item := range FoundationalNodeAccounts {
		if item.BlsPublicKey == blsPublicKey {
			return i + 8, &item
		}
	}
	return -1, nil
}

// GenesisBeaconAccountPriKey is the private key of genesis beacon account.
var GenesisBeaconAccountPriKey = BeaconAccountPriKey()

// GenesisBeaconAccountPublicKey is the private key of genesis beacon account.
var GenesisBeaconAccountPublicKey = GenesisBeaconAccountPriKey.PublicKey

// DeployedContractAddress is the deployed contract address of the staking smart contract in beacon chain.
var DeployedContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(GenesisBeaconAccountPublicKey), uint64(0))
