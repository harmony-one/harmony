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

// {Address: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8", Private: "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", Public: "0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8"},
// {Address: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", Private: "371cb68abe6a6101ac88603fc847e0c013a834253acee5315884d2c4e387ebca", Public: "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E"},
// curl 'http://127.0.0.1:30000/balance?key=0xd2Cb501B40D3a9a013A38267a4d2A4Cf6bD2CAa8'

var (
	fromPrivateKey = flag.String("private", "3c8642f7188e05acc4467d9e2aa7fd539e82aa90a5497257cf0ecbb98ed3b88f", "private key")
	toAddress      = flag.String("to_address", "0x10A02A0a6e95a676AE23e2db04BEa3D1B8b7ca2E", "address of to account.")
	shardID        = flag.Int64("shard_id", 0, "shard id")
)

func generateTxnHarmony(PrivateKeyFrom string, ToAddress string, shardID uint32, amount int64) *types.Transaction {
	privateKey, _ := crypto.HexToECDSA(PrivateKeyFrom)
	nonce := uint64(0)
	value := big.NewInt(1000000000000000000 * amount)
	gasLimit := uint64(21000)
	toAddress := common.HexToAddress(ToAddress)
	var data []byte

	signedTx, _ := types.SignTx(types.NewTransaction(nonce,
		toAddress,
		shardID,
		value,
		gasLimit, nil, data),
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
