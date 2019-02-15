package lib

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/api/client"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
	peer "github.com/libp2p/go-libp2p-peer"
)

func TestCreateWalletNode(test *testing.T) {
	walletNode := CreateWalletNode()

	if walletNode.Client == nil {
		test.Errorf("Wallet node's client is not initialized")
	}
}

func TestSubmitTransaction(test *testing.T) {
	ctrl := gomock.NewController(test)
	defer ctrl.Finish()

	m := mock_host.NewMockHost(ctrl)

	m.EXPECT().GetSelfPeer().AnyTimes()
	m.EXPECT().SendMessage(gomock.Any(), gomock.Any()).Times(1)

	walletNode := node.New(m, nil, nil)
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9990")
	peerID, _ := peer.IDFromPrivateKey(priKey)
	walletNode.Client = client.NewClient(walletNode.GetHost(), map[uint32]p2p.Peer{0: p2p.Peer{IP: "127.0.0.1", Port: "9990", PeerID: peerID}})

	SubmitTransaction(&types.Transaction{}, walletNode, 0)

	time.Sleep(1 * time.Second)
}
