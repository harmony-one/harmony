package wallet

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/harmony-one/harmony/api/client"
	proto_node "github.com/harmony-one/harmony/api/proto/node"
	"github.com/harmony-one/harmony/api/service/networkinfo"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/node"
	"github.com/harmony-one/harmony/p2p"
	p2p_host "github.com/harmony-one/harmony/p2p/host"
	mock_host "github.com/harmony-one/harmony/p2p/host/mock"
)

func TestCreateWalletNode(test *testing.T) {
	// shorten the retry time
	networkinfo.ConnectionRetry = 3

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

	tx := &types.Transaction{}
	msg := proto_node.ConstructTransactionListMessageAccount(types.Transactions{tx})
	m.EXPECT().SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeaconClient}, p2p_host.ConstructP2pMessage(byte(0), msg))

	SubmitTransaction(tx, walletNode, 0, nil)

	time.Sleep(1 * time.Second)
}
