package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/p2p"

	proto_node "github.com/harmony-one/harmony/proto/node"
)

func TestNewNewNode(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil)
	if node.Consensus == nil {
		test.Error("Consensus is not initialized for the node")
	}

	if node.blockchain == nil {
		test.Error("Blockchain is not initialized for the node")
	}

	if len(node.blockchain.Blocks) != 1 {
		test.Error("Genesis block is not initialized for the node")
	}

	if len(node.blockchain.Blocks[0].Transactions) != 1 {
		test.Error("Coinbase TX is not initialized for the node")
	}

	if node.UtxoPool == nil {
		test.Error("Utxo pool is not initialized for the node")
	}
}

func TestCountNumTransactionsInBlockchain(test *testing.T) {
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil)
	node.AddTestingAddresses(1000)
	if node.countNumTransactionsInBlockchain() != 1001 {
		test.Error("Count of transactions in the blockchain is incorrect")
	}
}

func TestAddPeers(test *testing.T) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)

	priKey2 := crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 := pki.GetPublicKeyFromScalar(priKey2)

	peers1 := []p2p.Peer{
		{
			IP:          "127.0.0.1",
			Port:        "8888",
			PubKey:      pubKey1,
			Ready:       true,
			ValidatorID: 1,
		},
		{
			IP:          "127.0.0.1",
			Port:        "9999",
			PubKey:      pubKey2,
			Ready:       false,
			ValidatorID: 2,
		},
	}
	leader := p2p.Peer{IP: "1", Port: "2"}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.NewConsensus("1", "2", "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil)
	r1 := node.AddPeers(peers1)
	e1 := 2
	if r1 != e1 {
		test.Errorf("Add %v peers, expectd %v", r1, e1)
	}
	r2 := node.AddPeers(peers1)
	e2 := 0
	if r2 != e2 {
		test.Errorf("Add %v peers, expectd %v", r2, e2)
	}
}

func sendPingMessage(leader p2p.Peer) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)

	p1 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9999",
		PubKey: pubKey1,
	}

	ping1 := proto_node.NewPingMessage(p1)
	buf1 := ping1.ConstructPingMessage()

	fmt.Println("waiting for 5 seconds ...")
	time.Sleep(5 * time.Second)

	p2p.SendMessage(leader, buf1)
	fmt.Println("sent ping message ...")
}

func sendPongMessage(leader p2p.Peer) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)
	p1 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9998",
		PubKey: pubKey1,
	}
	priKey2 := crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 := pki.GetPublicKeyFromScalar(priKey2)
	p2 := p2p.Peer{
		IP:     "127.0.0.1",
		Port:   "9999",
		PubKey: pubKey2,
	}

	pong1 := proto_node.NewPongMessage([]p2p.Peer{p1, p2}, nil)
	buf1 := pong1.ConstructPongMessage()

	fmt.Println("waiting for 10 seconds ...")
	time.Sleep(10 * time.Second)

	p2p.SendMessage(leader, buf1)
	fmt.Println("sent pong message ...")
}

func exitServer() {
	fmt.Println("wait 15 seconds to terminate the process ...")
	time.Sleep(15 * time.Second)

	os.Exit(0)
}

// func TestPingPongHandler(test *testing.T) {
// 	leader := p2p.Peer{IP: "127.0.0.1", Port: "8881"}
// 	consensus := consensus.NewConsensus("127.0.0.1", "8881", "0", []p2p.Peer{leader}, leader)
// 	node := New(consensus, nil)

// 	//	go sendPingMessage(leader)
// 	go sendPongMessage(leader)
// 	go exitServer()

// 	node.StartServer("8881")
// }
