package node

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/contracts/structs"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	amount          = big.NewInt(10)
	blockNum        = big.NewInt(15000)
	lockPeriodCount = big.NewInt(1)
	testAddress     = common.Address{123}
	testBlsPubKey1  = [32]byte{}
	testBlsPubKey2  = [32]byte{}
	testBlsPubKey3  = [32]byte{}
)

func TestUpdateStakingList(t *testing.T) {
	pubKey := bls.RandPrivateKey().GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, nil)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)

	for i := 0; i < 5; i++ {
		selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock, common.Address{})
		node.Worker.CommitTransactions(selectedTxs, common.Address{})
		block, _ := node.Worker.Commit([]byte{}, []byte{}, 0, common.Address{})

		err := node.AddNewBlock(block)
		if err != nil {
			t.Errorf("newhost failure: %v", err)
		}
	}

	stakeInfo := &structs.StakeInfoReturnValue{
		LockedAddresses:  []common.Address{testAddress},
		BlsPubicKeys1:    [][32]byte{testBlsPubKey1},
		BlsPubicKeys2:    [][32]byte{testBlsPubKey2},
		BlsPubicKeys3:    [][32]byte{testBlsPubKey3},
		BlockNums:        []*big.Int{blockNum},
		LockPeriodCounts: []*big.Int{lockPeriodCount},
		Amounts:          []*big.Int{amount},
	}

	node.UpdateStakingList(stakeInfo)

	if node.CurrentStakes[testAddress].Amount.Cmp(amount) != 0 {
		t.Error("Stake Info is not updated correctly")
	}
}
