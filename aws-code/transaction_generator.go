package main

import (
	"harmony-benchmark/blockchain"
	"math/rand"
	"time"
	"flag"
	"harmony-benchmark/node"
	"harmony-benchmark/p2p"
)

func newRandTransaction() blockchain.Transaction {
	txin := blockchain.TXInput{[]byte{}, rand.Intn(100), string(rand.Uint64())}
	txout := blockchain.TXOutput{rand.Intn(100), string(rand.Uint64())}
	tx := blockchain.Transaction{nil, []blockchain.TXInput{txin}, []blockchain.TXOutput{txout}}
	tx.SetID()

	return tx
}

func main() {

	ip := flag.String("ip", "127.0.0.1", "IP of the leader")
	port := flag.String("port", "9000", "port of the leader.")

	txs := make([]blockchain.Transaction, 10)
	for true {
		for i := range txs {
			txs[i] = newRandTransaction()

		}
		msg := node.ConstructTransactionListMessage(txs)
		p2p.SendMessage(p2p.Peer{*ip, *port, "n/a"}, msg)
		time.Sleep(1 * time.Second)  // 10 transactions per second
	}
}
