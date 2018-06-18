package main

import (
	"bufio"
	"flag"
	"fmt"
	"harmony-benchmark/blockchain"
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

func getPeers(Ip, Port, config string) []p2p.Peer {
	file, _ := os.Open(config)
	fscanner := bufio.NewScanner(file)
	var peerList []p2p.Peer
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		ip, port, status := p[0], p[1], p[2]
		if status == "leader" || ip == Ip && port == Port {
			continue
		}
		peer := p2p.Peer{Port: port, Ip: ip}
		peerList = append(peerList, peer)
	}
	return peerList
}
func main() {

	ip := flag.String("ip", "127.0.0.1", "IP of the leader")
	port := flag.String("port", "9000", "port of the leader.")
	configFile := flag.String("config_file", "local_config.txt", "file containing all ip addresses and config")
	//getLeader to get ip,port and get totaltime I want to run
	start := time.Now()
	totalTime := 60.0
	txs := make([]blockchain.Transaction, 10)
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
		p2p.SendMessage(p2p.Peer{*ip, *port, "n/a"}, msg)
		time.Sleep(1 * time.Second) // 10 transactions per second
	}
	msg := node.ConstructStopMessage()
	var leaderPeer p2p.Peer
	leaderPeer.Ip = *ip
	leaderPeer.Port = *port
	peers := append(getPeers(*ip, *port, *configFile), leaderPeer)
	p2p.BroadcastMessage(peers, msg)
}
