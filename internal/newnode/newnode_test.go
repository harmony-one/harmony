package newnode

import (
	"testing"

	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/libs"
	"github.com/harmony-one/harmony/p2p"
)

func TestNewNode(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8080"
	nnode := New(ip, port)

	if nnode.PubK == nil {
		t.Error("new node public key not initialized")
	}

	if nnode.SetInfo {
		t.Error("new node setinfo initialized to true! (should be false)")
	}
}

func TestBeaconChainConnect(t *testing.T) {
	var ip, beaconport, nodeport string
	ip = "127.0.0.1"
	beaconport = "8080"
	nodeport = "8081"
	nnode := New(ip, nodeport)
	bc := beaconchain.New(1, ip, beaconport)
	go bc.StartServer()
	BCPeer := p2p.Peer{IP: ip, Port: beaconport}
	err := nnode.ContactBeaconChain(BCPeer)
	if err != nil {
		t.Error("could not read from connection")
	}

}
