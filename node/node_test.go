package node

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/harmony-one/harmony/utils"

	"github.com/harmony-one/harmony/crypto"
	"github.com/harmony-one/harmony/crypto/pki"

	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/p2p"

	proto_node "github.com/harmony-one/harmony/proto/node"
)

func TestNewNewNode(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "1", Port: "2", PubKey: pubKey}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.New(leader, "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil, leader)
	if node.Consensus == nil {
		t.Error("Consensus is not initialized for the node")
	}

	if node.blockchain == nil {
		t.Error("Blockchain is not initialized for the node")
	}

	if len(node.blockchain.Blocks) != 1 {
		t.Error("Genesis block is not initialized for the node")
	}

	if len(node.blockchain.Blocks[0].Transactions) != 1 {
		t.Error("Coinbase TX is not initialized for the node")
	}

	if node.UtxoPool == nil {
		t.Error("Utxo pool is not initialized for the node")
	}
}

func TestCountNumTransactionsInBlockchain(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "1", Port: "2", PubKey: pubKey}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.New(leader, "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil, leader)
	node.AddTestingAddresses(1000)
	if node.countNumTransactionsInBlockchain() != 1001 {
		t.Error("Count of transactions in the blockchain is incorrect")
	}
}

func TestGetSyncingPeers(t *testing.T) {
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "1", Port: "2", PubKey: pubKey}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.New(leader, "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil, leader)
	peer := p2p.Peer{IP: "1.1.1.1", Port: "2000"}
	peer2 := p2p.Peer{IP: "2.1.1.1", Port: "2000"}
	node.Neighbors.Store("minh", peer)
	node.Neighbors.Store("mark", peer2)
	res := node.GetSyncingPeers()
	if len(res) != 2 || !((res[0].IP == peer.IP && res[1].IP == peer2.IP) || (res[1].IP == peer.IP && res[0].IP == peer2.IP)) {
		t.Error("GetSyncingPeers should return list of {peer, peer2}")
	}
	if len(res) != 2 || res[0].Port != "1000" || res[1].Port != "1000" {
		t.Error("Syncing ports should be 1000")
	}
}

func TestAddPeers(t *testing.T) {
	priKey1 := crypto.Ed25519Curve.Scalar().SetInt64(int64(333))
	pubKey1 := pki.GetPublicKeyFromScalar(priKey1)

	priKey2 := crypto.Ed25519Curve.Scalar().SetInt64(int64(999))
	pubKey2 := pki.GetPublicKeyFromScalar(priKey2)

	peers1 := []*p2p.Peer{
		&p2p.Peer{
			IP:          "127.0.0.1",
			Port:        "8888",
			PubKey:      pubKey1,
			Ready:       true,
			ValidatorID: 1,
		},
		&p2p.Peer{
			IP:          "127.0.0.1",
			Port:        "9999",
			PubKey:      pubKey2,
			Ready:       false,
			ValidatorID: 2,
		},
	}
	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "1", Port: "2", PubKey: pubKey}
	validator := p2p.Peer{IP: "3", Port: "5"}
	consensus := consensus.New(leader, "0", []p2p.Peer{leader, validator}, leader)

	node := New(consensus, nil, leader)
	r1 := node.AddPeers(peers1)
	e1 := 2
	if r1 != e1 {
		t.Errorf("Add %v peers, expectd %v", r1, e1)
	}
	r2 := node.AddPeers(peers1)
	e2 := 0
	if r2 != e2 {
		t.Errorf("Add %v peers, expectd %v", r2, e2)
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

func TestPingPongHandler(test *testing.T) {
	_, pubKey := utils.GenKey("127.0.0.1", "8881")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "8881", PubKey: pubKey}
	//   validator := p2p.Peer{IP: "127.0.0.1", Port: "9991"}
	consensus := consensus.New(leader, "0", []p2p.Peer{leader}, leader)
	node := New(consensus, nil, leader)
	//go sendPingMessage(leader)
	go sendPongMessage(leader)
	go exitServer()
	node.StartServer("8881")
}
