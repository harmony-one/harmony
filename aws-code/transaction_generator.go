package main

import (
	"bufio"
	"flag"
	"fmt"
	"harmony-benchmark/blockchain"
	"harmony-benchmark/log"
	"harmony-benchmark/node"
	"harmony-benchmark/p2p"
	"math/rand"
	"os"
	"strings"
	"time"
)

func newRandTransaction() blockchain.Transaction {
	txin := blockchain.TXInput{[]byte{}, rand.Intn(100), string(rand.Uint64())}
	txout := blockchain.TXOutput{rand.Intn(100), string(rand.Uint64())}
	tx := blockchain.Transaction{nil, []blockchain.TXInput{txin}, []blockchain.TXOutput{txout}}
	tx.SetID()

	return tx
}

func getValidators(config string) []p2p.Peer {
	file, _ := os.Open(config)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}

func getLeaders(config *[][]string) []p2p.Peer {
	var peerList []p2p.Peer
	for _, node := range *config {
		ip, port, status := node[0], node[1], node[2]
		if status == "leader" {
			peerList = append(peerList, p2p.Peer{Ip: ip, Port: port})
		}
	}
	return peerList
}

func readConfigFile(configFile string) [][]string {
	file, _ := os.Open(configFile)
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	return result
}

func main() {
	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	numTxsPerBatch := flag.Int("num_txs_per_batch", 100, "number of transactions to send per message")
	flag.Parse()
	config := readConfigFile(*configFile)

	start := time.Now()
	totalTime := 60.0
	txs := make([]blockchain.Transaction, *numTxsPerBatch)
	leaders := getLeaders(&config)
	for true {
		t := time.Now()
		if t.Sub(start).Seconds() >= totalTime {
			fmt.Println(int(t.Sub(start)), start, totalTime)
			break
		}
		for i := range txs {
			txs[i] = newRandTransaction()

		}
		msg := node.ConstructTransactionListMessage(txs)
		log.Debug("[Generator] Sending txs to leader[s]", "numOfLeader", len(leaders))
		p2p.BroadcastMessage(leaders, msg)
		time.Sleep(1 * time.Second) // 10 transactions per second
	}
	msg := node.ConstructStopMessage()
	peers := append(getValidators(*configFile), leaders...)
	p2p.BroadcastMessage(peers, msg)
}
