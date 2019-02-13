package host

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	libp2p_peer "github.com/libp2p/go-libp2p-peer"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestSendMessage(test *testing.T) {
	peer1 := p2p.Peer{IP: "127.0.0.1", Port: "9000"}
	selfAddr1, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", peer1.Port))
	peer1.Addrs = append(peer1.Addrs, selfAddr1)
	priKey1, pubKey1, _ := utils.GenKeyP2P(peer1.IP, peer1.Port)
	peerID1, _ := libp2p_peer.IDFromPublicKey(pubKey1)
	peer1.PeerID = peerID1
	host1, _ := p2pimpl.NewHost(&peer1, priKey1)

	peer2 := p2p.Peer{IP: "127.0.0.1", Port: "9001"}
	selfAddr2, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%s", peer2.Port))
	peer2.Addrs = append(peer2.Addrs, selfAddr2)
	priKey2, pubKey2, _ := utils.GenKeyP2P(peer2.IP, peer2.Port)
	peerID2, _ := libp2p_peer.IDFromPublicKey(pubKey2)
	peer2.PeerID = peerID2
	host2, _ := p2pimpl.NewHost(&peer2, priKey2)

	msg := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
	if err := host1.AddPeer(&peer2); err != nil {
		test.Fatalf("cannot add peer2 to host1: %v", err)
	}

	go host2.BindHandlerAndServe(handler)
	SendMessage(host1, peer2, msg, nil)
	time.Sleep(3 * time.Second)
}

func handler(s p2p.Stream) {
	defer func() {
		if err := s.Close(); err != nil {
			panic(fmt.Sprintf("Close(%v) failed: %v", s, err))
		}
	}()
	content, err := p2p.ReadMessageContent(s)
	if err != nil {
		panic("Read p2p data failed")
	}
	golden := []byte{0x00, 0x01, 0x02, 0x03, 0x04}

	if !reflect.DeepEqual(content, golden) {
		panic("received message not equal original message")
	}
}
