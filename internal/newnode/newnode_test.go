package newnode

import (
	"fmt"
	"testing"
	"time"

	beaconchain "github.com/harmony-one/harmony/internal/beaconchain/libs"
	"github.com/harmony-one/harmony/p2p"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
)

func TestNewNode(t *testing.T) {
	var ip, port string
	ip = "127.0.0.1"
	port = "8088"
	nnode := New(ip, port)

	if nnode.PubK == nil {
		t.Error("new node public key not initialized")
	}
}

func TestBeaconChainConnect(t *testing.T) {
	var ip, beaconport, bcma, nodeport string

	ip = "127.0.0.1"
	beaconport = "8081"
	nodeport = "9081"

	nnode := New(ip, nodeport)
	bc := beaconchain.New(1, ip, beaconport)
	bcma = fmt.Sprintf("/ip4/%s/tcp/%s/ipfs/%s", bc.Self.IP, bc.Self.Port, bc.GetID().Pretty())

	go bc.StartServer()
	time.Sleep(3 * time.Second)

	maddr, err := multiaddr.NewMultiaddr(bcma)
	if err != nil {
		t.Errorf("new multiaddr error: %v", err)
	}

	// Extract the peer ID from the multiaddr.
	info, err2 := peerstore.InfoFromP2pAddr(maddr)
	if err2 != nil {
		t.Errorf("info from p2p addr error: %v", err2)
	}

	BCPeer := &p2p.Peer{IP: ip, Port: beaconport, Addrs: info.Addrs, PeerID: info.ID}

	nnode.AddPeer(BCPeer)

	err3 := nnode.ContactBeaconChain(*BCPeer)

	if err3 != nil {
		t.Errorf("could not read from connection: %v", err3)
	}
}
