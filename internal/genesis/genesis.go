package genesis

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
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
		utils.Logger().Error().Msg("Failed to generate beacon chain contract deployer account")
		os.Exit(111)
	}
	return prikey
}

// GenesisBeaconAccountPriKey is the private key of genesis beacon account.
var GenesisBeaconAccountPriKey = BeaconAccountPriKey()

// GenesisBeaconAccountPublicKey is the private key of genesis beacon account.
var GenesisBeaconAccountPublicKey = GenesisBeaconAccountPriKey.PublicKey

// DeployedContractAddress is the deployed contract address of the staking smart contract in beacon chain.
var DeployedContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(GenesisBeaconAccountPublicKey), uint64(0))
