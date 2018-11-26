package p2pv2

import (
	"bufio"
	"context"
	"fmt"
	"net"

	"github.com/harmony-one/harmony/log"

	host "github.com/libp2p/go-libp2p-host"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// TODO:
// 1. send/receive data to/from peer
// 1. add peer to store

const MaxBroadCast = 20

type Peer struct {
	Host        host.Host
	ValidatorID int // Legacy field
	PubKey      string
}

func Send(ip, port string, message []byte) error {
	addr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%s", port)
	targetAddr, err := multiaddr.NewMultiaddr(addr)

	priv := portToPrivKey(port)
	peerID, _ := peer.IDFromPrivateKey(priv)
	log.Debug("Send", "port", port, "id", peerID)
	myHost.Peerstore().AddAddrs(peerID, []multiaddr.Multiaddr{targetAddr}, peerstore.PermanentAddrTTL)
	s, err := myHost.NewStream(context.Background(), peerID, "/harmony/0.0.1")
	catchError(err)

	// Create a buffered stream so that read and writes are non blocking.
	w := bufio.NewWriter(bufio.NewWriter(s))

	// Create a thread to read and write data.
	go writeData(w, message)
	return nil
}

func Read(conn net.Conn) ([]byte, error) {
	// myHost.SetStreamHandler("/harmony/0.0.1", handleStream)
	var data []byte
	return data, nil
}
