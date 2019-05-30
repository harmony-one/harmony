package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	eth_types "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/core/types"
)

var (
	fromPrivateKey = flag.String("private", "fad9c8855b740a0b7ed4c221dbad0f33a83a49cad6b3fe8d5817ac83d38b6a19", "private key")
	toAddress      = flag.String("to_address", "0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d", "private key")
	shardID        = flag.Int64("shard_id", 0, "shard id")
)

func generateTxnHarmony(PrivateKeyFrom string, ToAddress string, chainID uint32, amount int64) *types.Transaction {
	privateKey, _ := crypto.HexToECDSA(PrivateKeyFrom)
	nonce := uint64(0)
	value := big.NewInt(1000000000000000000 * amount)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1000000)
	toAddress := common.HexToAddress(ToAddress)
	var data []byte

	signedTx, _ := types.SignTx(types.NewTransaction(nonce,
		toAddress,
		chainID,
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

func generateTxnEth(PrivateKeyFrom string, ToAddress string, chainID int64, amount int64) *eth_types.Transaction {
	privateKey, _ := crypto.HexToECDSA(PrivateKeyFrom)
	nonce := uint64(0)
	value := big.NewInt(1000000000000000000 * amount)
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1000000)
	toAddress := common.HexToAddress(ToAddress)
	var data []byte
	tx := eth_types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	ID := big.NewInt(chainID)

	signedTx, err := eth_types.SignTx(tx, eth_types.NewEIP155Signer(ID), privateKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("signedTx.Hash().Hex(): ", signedTx.Hash().Hex())

	ts := eth_types.Transactions{signedTx}
	rawTxBytes := ts.GetRlp(0)
	rawTxHex := hex.EncodeToString(rawTxBytes)
	fmt.Println("serialized: ", rawTxHex)

	return signedTx
}

func main() {
	flag.Parse()

	fmt.Println("Generate using ethereum NewEIP155Signer with chain ID")
	generateTxnEth(*fromPrivateKey, *toAddress, *shardID, 1)

	fmt.Println("Generate using harmonye/core/types with chain ID")
	generateTxnHarmony(*fromPrivateKey, *toAddress, uint32(*shardID), 1)
}
