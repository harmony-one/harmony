package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
)

// {Address: "0x1a3e7a44ee21101d7D64FBf29B0F6F1fc295F723", Private: "27978f895b11d9c737e1ab1623fde722c04b4f9ccb4ab776bf15932cc72d7c66", Public: "0x1a3e7a44ee21101d7D64FBf29B0F6F1fc295F723"},
// {Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},

var (
	fromPrivateKey = flag.String("private", "27978f895b11d9c737e1ab1623fde722c04b4f9ccb4ab776bf15932cc72d7c66", "private key")
	toAddress      = flag.String("to_address", "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", "address of to account.")
	shardID        = flag.Int64("shard_id", 0, "shard id")
)

func generateTxnHarmony(PrivateKeyFrom string, ToAddress string, shardID uint32, amount int64) *types.Transaction {
	privateKey, _ := crypto.HexToECDSA(PrivateKeyFrom)
	nonce := uint64(0)
	value := big.NewInt(1000000000000000000 * amount)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1000000)
	toAddress := common.HexToAddress(ToAddress)
	var data []byte

	signedTx, _ := types.SignTx(types.NewTransaction(nonce,
		toAddress,
		shardID,
		value,
		gasLimit, gasPrice, data),
		// params.TxGas, nil, nil),
		types.HomesteadSigner{},
		privateKey)

	fmt.Println("signedTx.Hash().Hex(): ", signedTx.Hash().Hex())

	ts := types.Transactions{signedTx}
	rawTxBytes := ts.GetRlp(0)
	fmt.Println(rawTxBytes)
	rawTxHex := hex.EncodeToString(rawTxBytes)
	fmt.Println("serialized: ", rawTxHex)

	return signedTx
}

func main() {
	flag.Parse()

	generateTxnHarmony(*fromPrivateKey, *toAddress, uint32(*shardID), 1)
}
