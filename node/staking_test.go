package node

import (
	"math/big"
	"testing"
	"time"

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
	blsKey := bls.RandPrivateKey()
	pubKey := blsKey.GetPublicKey()
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus, err := consensus.New(host, 0, leader, blsKey)
	if err != nil {
		t.Fatalf("Cannot craeate consensus: %v", err)
	}
	node := New(host, consensus, testDBFactory, false)
	node.BlockPeriod = 8 * time.Second

	for i := 0; i < 1; i++ {
		selectedTxs, selectedStakingTxs := node.getTransactionsForNewBlock(common.Address{})
		node.Worker.CommitTransactions(selectedTxs, selectedStakingTxs, common.Address{})
		block, err := node.Worker.FinalizeNewBlock([]byte{}, []byte{}, 0, common.Address{}, nil, nil)

		// The block must first be finalized before being added to the blockchain.
		if err != nil {
			t.Errorf("Error when finalizing block: %v", err)
		}
		block.Header()
		err = node.AddNewBlock(block)
		if err != nil {
			t.Errorf("Error when adding new block: %v", err)
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

	/*
		if node.CurrentStakes[testAddress].Amount.Cmp(amount) != 0 {
			t.Error("Stake Info is not updated correctly")
		}
	*/
}
