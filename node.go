package main

import (
	"fmt"
	"time"
	"math/rand"
)

var node_ips = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

type Node struct {
	ip     int
	leader bool
}
type Nodes struct {
	Nodes []*Node
}
type txn struct {
	tx string
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

func (n Node) send() {
	fmt.Printf("Leader with id: %d has sent message\n", n.ip)
}
func (n Node) receive() {
	fmt.Printf("Node: %d received message\n", n.ip)
}
func createNode(ip int, isLeader bool) Node {
	n := Node{ip: ip, leader: isLeader}
	return n
}
func pickLeader(i int) bool {
	if i == 0 {
		return true
	} else {
		return false
	}
}

var N = make([]Node, 10)

func main() {
	for i, id := range node_ips {
		isLeader := pickLeader(i)
		m := createNode(id, isLeader)
		N[i] = m
	}
	for i, _ := range N {
		m := N[i]
		if m.leader {
			m.send()
		} else {
			m.receive()
		}
	}
	for i := 0; i < 100; i++ {
	rand.Seed(time.Now().UnixNano())
    fmt.Println(randomString(10)) // print 10 chars
}
}
