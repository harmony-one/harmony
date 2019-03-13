package networkinfo

import (
	"testing"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

func TestService(t *testing.T) {
	nodePriKey, _, err := utils.LoadKeyFromFile("/tmp/127.0.0.1.12345.key")
	if err != nil {
		t.Fatal(err)
	}
	peerPriKey, peerPubKey := utils.GenKey("127.0.0.1", "12345")
	if peerPriKey == nil || peerPubKey == nil {
		t.Fatal("generate key error")
	}
	selfPeer := p2p.Peer{IP: "127.0.0.1", Port: "12345", ValidatorID: -1, ConsensusPubKey: peerPubKey}

	host, err := p2pimpl.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		t.Fatal("unable to new host in harmony")
	}

	s := New(host, p2p.GroupIDBeaconClient, nil, nil)

	s.StartService()

	time.Sleep(2 * time.Second)

	s.StopService()
}
