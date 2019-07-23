package main

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/harmony-one/harmony/hmyclient"
)

// newRPCClient creates a rpc client with specified node URL.
func newRPCClient(url string) *rpc.Client {
	client, err := rpc.Dial(url)
	if err != nil {
		fmt.Errorf("Failed to connect to Ethereum node: %v", err)
	}
	return client
}

func main() {
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()
	rpcClient := newRPCClient("http://localhost:9500")
	if rpcClient == nil {
		fmt.Errorf("Failed to create rpc client")
	}
	client := hmyclient.NewClient(rpcClient)
	if client == nil {
		fmt.Errorf("Failed to create client")
	}

	networkID, err := client.NetworkID(ctx)
	if err != nil {
		fmt.Errorf("Failed to get net_version: %v", err)
	}
	fmt.Printf("net_version: %v\n", networkID)

	blockNumber, err := client.BlockNumber(ctx)
	if err != nil {
		fmt.Errorf("Failed to get hmy_blockNumber: %v", err)
	}
	fmt.Printf("hmy_blockNumber: %v\n", blockNumber)

	block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(uint64(blockNumber)))
	if err != nil {
		fmt.Errorf("Failed to get hmy_getBlockByNumber %v: %v", blockNumber, err)
	}
	fmt.Printf("hmy_getBlockByNumber(%v):\n", blockNumber)
	fmt.Printf("number: %v\n", block.Number().Text(16))
	fmt.Printf("hash: %v\n", block.Hash().String())
	fmt.Printf("parentHash: %v\n", block.ParentHash().String())
	fmt.Printf("timestamp: %v\n", block.Time().Text(16))
	fmt.Printf("size: %v\n", block.Size())
	fmt.Printf("miner: %v\n", block.Coinbase().String())
	fmt.Printf("receiptsRoot: %v\n", block.ReceiptHash().String())
	fmt.Printf("transactionsRoot: %v\n", block.TxHash().String())

	block, err = client.BlockByNumber(ctx, nil)
	if err != nil {
		fmt.Errorf("Failed to get block: %v", err)
	}
	fmt.Printf("hmy_getBlockByNumber(latest): %v", block)
}
