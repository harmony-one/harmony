package main

import (
	"fmt"
)

var node_ips = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

type Node struct {
	ip     int
	leader bool
}
type Nodes struct {
	Nodes []*Node
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
}
