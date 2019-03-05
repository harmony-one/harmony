package lib

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/api/client"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/node"
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

	walletNode := node.New(m, nil, nil)
	walletNode.Client = client.NewClient(walletNode.GetHost(), []uint32{0})

	SubmitTransaction(&types.Transaction{}, walletNode, 0)

	time.Sleep(1 * time.Second)
}
