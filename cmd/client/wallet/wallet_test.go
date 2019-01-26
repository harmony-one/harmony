package main

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/api/client"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
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
	walletNode.Client = client.NewClient(walletNode.GetHost(), &map[uint32]p2p.Peer{0: p2p.Peer{IP: "1", Port: "2"}})

	SubmitTransaction(&types.Transaction{}, walletNode, 0)

	time.Sleep(1 * time.Second)
}
