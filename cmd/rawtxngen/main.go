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

var (
	fromPrivateKey = flag.String("private", "fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", "private key")
	toAddress      = flag.String("to_address", "0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d", "private key")
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
	rawTxHex := hex.EncodeToString(rawTxBytes)
	fmt.Println("serialized: ", rawTxHex)

	return signedTx
}

func main() {
	flag.Parse()

	generateTxnHarmony(*fromPrivateKey, *toAddress, uint32(*shardID), 1)
}
