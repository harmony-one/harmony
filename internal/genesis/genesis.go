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
	Address string       // account address
	Public  string       // account public key
   BLSKey  string       // account private BLS key (To be removed)
   ShardID uint32       // shardID of the account
}

func (d DeployAccount) String() string {
   return fmt.Sprintf("%s/%s/%s:%d", d.Address, d.Public, d.BLSKey, d.ShardID)
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
// the account address could be from GenesisAccounts or from GenesisFNAccounts
// the account index can be used to determin the shard of the account
func FindAccount(address string) (int, *DeployAccount) {
   for i, acc := range GenesisAccounts {
      if address == acc.Address {
         return i, &acc
      }
   }
   for i, acc := range GenesisFNAccounts {
      if address == acc.Address {
         return i, &acc
      }
   }
   return 0, nil
}

// GenesisBeaconAccountPriKey is the private key of genesis beacon account.
var GenesisBeaconAccountPriKey = BeaconAccountPriKey()

// GenesisBeaconAccountPublicKey is the private key of genesis beacon account.
var GenesisBeaconAccountPublicKey = GenesisBeaconAccountPriKey.PublicKey

// DeployedContractAddress is the deployed contract address of the staking smart contract in beacon chain.
var DeployedContractAddress = crypto.CreateAddress(crypto.PubkeyToAddress(GenesisBeaconAccountPublicKey), uint64(0))

// GenesisAccounts are the accounts for the initial genesis nodes hosted by Harmony.
var GenesisAccounts = [...]DeployAccount{
   // 0-9
	{Address: "0x007579ED2Fe889C5255C36d4978Ac94d25811771", Public: "0x007579ED2Fe889C5255C36d4978Ac94d25811771", BLSKey: "66acb3a7c990be4b06709058fdef8122b7ecdbaf023e56ccf8cdf671c5333646"},
	{Address: "0x00F98965458a35f3788C45A095582AB18A5ae79c", Public: "0x00F98965458a35f3788C45A095582AB18A5ae79c", BLSKey: "5e9e2fffbf7cfad085d7b0147d2acd680cfd8b8d62daa9c39370185ba0207920"},
	{Address: "0x0102B41674C3ac2634f404d8c25C25Bb959fE952", Public: "0x0102B41674C3ac2634f404d8c25C25Bb959fE952", BLSKey: "56714bb94188c335d1243fa3d17fd50ff63a1a9bf740faecd97996f3a0737e87"},
	{Address: "0x0178A7bE4399c1968156edE4f52ae91953ab9B63", Public: "0x0178A7bE4399c1968156edE4f52ae91953ab9B63", BLSKey: "3de4fbe27f453b254014094108bf11b0a2ba4144585bf4cb10332155476339e9"},
	{Address: "0x0215c51A3d67Eb1e949bD1Df8b74D3aef097e92d", Public: "0x0215c51A3d67Eb1e949bD1Df8b74D3aef097e92d", BLSKey: "18c62ae8cd2a37e50b8fc89c53458ad6d9482f413d50086199d309ad1062684c"},
	{Address: "0x021983eA41fbeeB39F82a9CAf1A83476F0cFeEDC", Public: "0x021983eA41fbeeB39F82a9CAf1A83476F0cFeEDC", BLSKey: "2ca85157154a7df9fcdb4a404c7a8ac0675bbff7e841237b0de4645a9dcaca1c"},
	{Address: "0x03d1a55eA1246efB257D49D9286f7D370bd09c97", Public: "0x03d1a55eA1246efB257D49D9286f7D370bd09c97", BLSKey: "1dc77bb20378c7cacf82dd6fb7d5dedbb2c0855d85e0eb2d7d6df05bbb5da65d"},
	{Address: "0x055b95d5205B5711099C32626Ea61481779a2233", Public: "0x055b95d5205B5711099C32626Ea61481779a2233", BLSKey: "185bcaede728332c088645b31b988404512eeeb02413360cac6e30c9ca002661"},
	{Address: "0x0566729A6FCDda16287777baB5D4425AA93bB0Fc", Public: "0x0566729A6FCDda16287777baB5D4425AA93bB0Fc", BLSKey: "18b5d0d89b4575e002e4fd41e46fa628a248e1caa55fa887acd3c446b89058e3"},
	{Address: "0x05bA7FcC4c1d7286f7A3d5552DdF305677338c22", Public: "0x05bA7FcC4c1d7286f7A3d5552DdF305677338c22", BLSKey: "2221983e3f69897d54fcaa3cb131f4c729592f1b10faf247c0772e7b6476c1fd"},

	// 10-19
	{Address: "0x063893E8EfA148E29B914702AD6A9930d41C8F13", Public: "0x063893E8EfA148E29B914702AD6A9930d41C8F13", BLSKey: "4000ba626d897e76a9878922deb081cfd52af3fefc6f353a51fa32dad88a6cc4"},
	{Address: "0x06693dEE3d72a30075E7447b18c6f3ed8AE62174", Public: "0x06693dEE3d72a30075E7447b18c6f3ed8AE62174", BLSKey: "60b6b4326239eb330207d9b751cfcea620a41b02b9ca270ed8f1670a1ffa3f51"},
	{Address: "0x066B40c45D06eEFE8Bb8677fdaFdaC5C8dF9d09C", Public: "0x066B40c45D06eEFE8Bb8677fdaFdaC5C8dF9d09C", BLSKey: "605234f8ea10fd73c0588f0e1fb40821efb341eea20e76d8350a1029e5af2dbf"},
	{Address: "0x079C1FFEaa70Ebdd2F3235b2F82BeE0b1101f092", Public: "0x079C1FFEaa70Ebdd2F3235b2F82BeE0b1101f092", BLSKey: "4505d95f6045c8d2a00b923c583e82979e75a7b0c920ddf6f2af902c1ae37432"},
	{Address: "0x07Fe4B973008c53528142b719BdfaC428F81905b", Public: "0x07Fe4B973008c53528142b719BdfaC428F81905b", BLSKey: "3b5fcadb39db34f9bbd4381836dc43a8629b6e14a3eac0942fcb70540538467b"},
	{Address: "0x09531Cea52595bCe55329Be07f11Ad033B9814Ee", Public: "0x09531Cea52595bCe55329Be07f11Ad033B9814Ee", BLSKey: "3a315d1c8ede4b7f18ef51f8249ab2a414c910196f1b9995f9cce2d6d8aa2201"},
	{Address: "0x0B4B626c913a46138feD9d7201E187A751DFF485", Public: "0x0B4B626c913a46138feD9d7201E187A751DFF485", BLSKey: "13a71379a7cb13aefa0433428b080d1f2ed3ee475f30bf093c1bfd278c07d0d0"},
	{Address: "0x0CCa9111F4588EDB3c9a282faE233B830dE21A0D", Public: "0x0CCa9111F4588EDB3c9a282faE233B830dE21A0D", BLSKey: "6d543bbc84d0b4daa45c077ddfaaffa43343558e5cc5a1efde846c630170747f"},
	{Address: "0x0F595ed534b6464eB2C80A037FFba02D23AfdfD2", Public: "0x0F595ed534b6464eB2C80A037FFba02D23AfdfD2", BLSKey: "1283b83fe50dc548eaa5cf850adb8940f5eaaff30816ecd46e1c0a7a548f52fc"},
	{Address: "0x0a0b8c48e42c540078fD99004915Be265f380dB7", Public: "0x0a0b8c48e42c540078fD99004915Be265f380dB7", BLSKey: "488678864c4f2b7f3c80d83564f4f0521b26a81bb96be4fcf207a2d4b8dd2d7c"},
}
