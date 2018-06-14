package main

import (
	"log"
	"math/rand"
	"time"
)

type Node struct {
	ip          int
	leader      bool
	reciveBlock chan string
	sendBlock   chan string
}

type Nodes struct {
	Nodes []*Node
}

func randomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// Generate a random string of A-Z chars with len = l
func randomString(len int) string {
	bytes := make([]byte, len)
	for i := 0; i < len; i++ {
		bytes[i] = byte(randomInt(97, 122))
	}
	return string(bytes)
}

func (n Node) send(cin <-chan string, id int) {
	for msg := range cin {
		log.Printf("Leader has sent message %s to %d\n", msg, id)
	}
}

func consume(cin <-chan string, id int) {
	for msg := range cin {
		log.Printf("Leader has sent message %s to %d\n", msg, id)
	}
}

func (n Node) receive() {
	log.Printf("Node: %d received message\n", n.ip)
}

func createNode(ip int, isLeader bool) Node {
	n := Node{ip: ip, leader: isLeader}
	return n
}

func pickLeader(i int) Node {
	if i == 0 {
		return createNode(i, true)
	} else {
		return createNode(i, false)
	}
}

func BufferedTxnQueueWithFanOut(ch <-chan string, size int) []chan string { // This needs
	cs := make([]chan string, size)
	for i, _ := range cs {
		// The size of the channels buffer controls how far behind the recievers
		// of the fanOut channels can lag the other channels.
		cs[i] = make(chan string)
	}
	go func() {
		for txs := range ch {
			for _, c := range cs {
				c <- txs
			}
		}
		for _, c := range cs {
			// close all our fanOut channels when the input channel is exhausted.
			close(c)
		}
	}()
	return cs
}

func TxnGenerator(numOfTxns int, lenOfRandomString int) <-chan string {
	out := make(chan string)
	go func() {
		for i := 0; i < numOfTxns; i++ {
			out <- randomString(lenOfRandomString)
			log.Printf("Transaction Number %d\n", i)
			//time.Sleep(2 * time.Second)
		}
		close(out)
	}()
	return out
}

func main() {
	var (
		//isLeader          Node
		numOfTxns         = 1000
		numOfNodes        = 10
		N                 = make([]Node, 10)
		lenOfRandomString = 10
		node_ips          = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	)
	for i := range node_ips {
		m := pickLeader(i)
		N[i] = m
		// if m.leader {
		// 	isLeader := m
		// }
	}
	txnqueue := TxnGenerator(numOfTxns, lenOfRandomString)
	Txns := BufferedTxnQueueWithFanOut(txnqueue, numOfNodes)
	for num := range Txns {
		txn := Txns[num]
		go consume(txn, num)
	}
	time.Sleep(60 * time.Second)
}
