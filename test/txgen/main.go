package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/common"
)

// {Address: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8", Private: "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", Public: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"},
// {Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},
// curl 'http://127.0.0.1:30000/balance?key=0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8'

var (
	fromPrivateKey = flag.String("private", "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", "private key")
	toAddress      = flag.String("to_address", "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", "address of to account.")
	shardID        = flag.Int64("shard_id", 0, "shard id")
)

// DeployAccount is the accounts used for development.
type DeployAccount struct {
	Address string
	Private string
	Public  string
}

// DemoAccounts is the accounts used for lottery demo.
var DemoAccounts = [...]DeployAccount{
	{Address: "0x1a3e7a44ee21101d7D64FBf29B0F6F1fc295F723", Private: "27978f895b11d9c737e1ab1623fde722c04b4f9ccb4ab776bf15932cc72d7c66", Public: "0x1a3e7a44ee21101d7D64FBf29B0F6F1fc295F723"},
	{Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},
	{Address: "0x3e881F6C36A3A14a2D1816b0A5471d1caBB16F33", Private: "3f8af52063c6648be37d4b33559f784feb16d8e5ffaccf082b3657ea35b05977", Public: "0x3e881F6C36A3A14a2D1816b0A5471d1caBB16F33"},
	{Address: "0x9d72989b68777a1f3FfD6F1DB079f1928373eE52", Private: "df77927961152e6a080ac299e7af2135fc0fb02eb044d0d7bbb1e8c5ad523809", Public: "0x9d72989b68777a1f3FfD6F1DB079f1928373eE52"},
	{Address: "0x67957240b6eB045E17B47dcE98102f09aaC03435", Private: "fcff43741ad2dd0b232efb159dc47736bbb16f11a79aaeec39b388d06f91116d", Public: "0x67957240b6eB045E17B47dcE98102f09aaC03435"},
	{Address: "0xf70fBDB1AD002baDF19024785b1a4bf6F841F558", Private: "916d3d78b7f413452434e89f9c1f1d136995ef02d7dc8038e84cc9cef4a02b96", Public: "0xf70fBDB1AD002baDF19024785b1a4bf6F841F558"},
	{Address: "0x3f1A559be93C9456Ca75712535Fd522f5EC22c6B", Private: "f5967bd87fd2b9dbf51855a2a75ef0a811c84953b3b300ffe90c430a5c856303", Public: "0x3f1A559be93C9456Ca75712535Fd522f5EC22c6B"},
	{Address: "0xedD257B4e0F5e7d632c737f4277e93b64DC268FC", Private: "f02f7b3bb5aa03aa97f9e030020dd9ca306b209742fafe018104a3207a70a3c9", Public: "0xedD257B4e0F5e7d632c737f4277e93b64DC268FC"},
	{Address: "0x66A74477FC1dd0F4924ed943C1d2F1Dece3Ab138", Private: "0436864cc15772448f88dd40554592ff6c91a6c1a389d965ad26ee143db1234d", Public: "0x66A74477FC1dd0F4924ed943C1d2F1Dece3Ab138"},
	{Address: "0x04178CdbCe3a9Ff9Ea385777aFc4b78B3E745281", Private: "dea956e530073ab23d9cae704f5d068482b1977c3173c9efd697c48a7fd3ce83", Public: "0x04178CdbCe3a9Ff9Ea385777aFc4b78B3E745281"},
	{Address: "0x46C61d50874A7A06D29FF89a710AbBD0856265be", Private: "af539d4ace07a9f601a8d3a6ca6f914d5a9fabe09cfe7d62ebc2348fc95f03a4", Public: "0x46C61d50874A7A06D29FF89a710AbBD0856265be"},
	{Address: "0xfE9BABE6904C28E31971337738FBCBAF8c72873e", Private: "7d24797eeba0cdac9bf943f0d82c4b18eb206108d6e1b7f610471594c0c94306", Public: "0xfE9BABE6904C28E31971337738FBCBAF8c72873e"},
	{Address: "0x3f78622de8D8f87EAa0E8b28C2851e2450E91250", Private: "4fa2fecce1becfaf7e5fba5394caacb318333b04071462b5ca850ee5a406dcfe", Public: "0x3f78622de8D8f87EAa0E8b28C2851e2450E91250"},
	{Address: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8", Private: "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", Public: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"},
	{Address: "0x2676e6dd2d7618be14cb4c18a355c81bf7aac647", Private: "bf29f6a33b2c24a8b5182ef44cc35ce87534ef827c8dfbc1e6bb536aa52f8563", Public: "0x2676e6dd2d7618be14cb4c18a355c81bf7aac647"},
}

func generateTxnHarmony(PrivateKeyFrom string, ToAddress string, shardID uint32, amount int64) (*types.Transaction, *types.Transaction) {
	privateKey, _ := crypto.HexToECDSA(PrivateKeyFrom)
	nonce := uint64(0)
	value := big.NewInt(1000000000000000000 * amount)
	gasLimit := uint64(21000)
	toAddress := common.ParseAddr(ToAddress)
	var data []byte

	unsignedTx := types.NewTransaction(nonce,
		toAddress,
		shardID,
		value,
		gasLimit, nil, data)
	signedTx, _ := types.SignTx(
		unsignedTx,
		types.HomesteadSigner{},
		privateKey,
	)

	// fmt.Println("signedTx.Hash().Hex(): ", signedTx.Hash().Hex())
	// ts := types.Transactions{signedTx}
	// rawTxBytes := ts.GetRlp(0)
	// fmt.Println(rawTxBytes)
	// rawTxHex := hex.EncodeToString(rawTxBytes)
	// fmt.Println("serialized: ", rawTxHex)
	return unsignedTx, signedTx
}

func serialized(tx *types.Transaction) string {
	ts := types.Transactions{tx}
	rawTxBytes := ts.GetRlp(0)
	rawTxHex := hex.EncodeToString(rawTxBytes)
	return rawTxHex
}

func main() {
	flag.Parse()

	generateTxnHarmony(*fromPrivateKey, *toAddress, uint32(*shardID), 1)

	for i := 0; i < len(DemoAccounts); i++ {
		j := (i + 1) % len(DemoAccounts)
		shardID := i % 4
		unsignedTx, signedTx := generateTxnHarmony(DemoAccounts[i].Private, DemoAccounts[j].Address, uint32(shardID), int64(shardID))
		fmt.Printf(
			"{ \"fromPrivate\": \"%s\",  \"toAddress\":  \"%s\", \"shardID\": %d,  \"amount\": %d, \"nonce\": 0, \"gasLimit\": 21000, \"unsignedTx\": \"%s\",  \"signedTx\": \"%s\"	},\n",
			DemoAccounts[i].Private,
			DemoAccounts[j].Address,
			shardID,
			shardID,
			serialized(unsignedTx),
			serialized(signedTx),
		)
	}

}
