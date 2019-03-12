package node

import (
	"math/big"
	"testing"

	"github.com/harmony-one/harmony/contracts/structs"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/p2pimpl"
)

var (
	amount          = big.NewInt(10)
	blockNum        = big.NewInt(15000)
	lockPeriodCount = big.NewInt(1)
	testAddress     = common.Address{123}
	testBlsAddress  = common.Address{132}.Bytes() // [20]byte
)

func TestUpdateStakingList(t *testing.T) {

	_, pubKey := utils.GenKey("1", "2")
	leader := p2p.Peer{IP: "127.0.0.1", Port: "9882", ConsensusPubKey: pubKey}
	validator := p2p.Peer{IP: "127.0.0.1", Port: "9885"}
	priKey, _, _ := utils.GenKeyP2P("127.0.0.1", "9902")
	host, err := p2pimpl.NewHost(&leader, priKey)
	if err != nil {
		t.Fatalf("newhost failure: %v", err)
	}
	consensus := consensus.New(host, "0", []p2p.Peer{leader, validator}, leader)
	node := New(host, consensus, nil)

	for i := 0; i < 5; i++ {
		selectedTxs := node.getTransactionsForNewBlock(MaxNumberOfTransactionsPerBlock)
		node.Worker.CommitTransactions(selectedTxs)
		block, _ := node.Worker.Commit()

		node.AddNewBlock(block)
	}

	stakeInfo := &structs.StakeInfoReturnValue{
		[]common.Address{testAddress},
		[][20]byte{testAddress},
		[]*big.Int{blockNum},
		[]*big.Int{lockPeriodCount},
		[]*big.Int{amount},
	}

	node.UpdateStakingList(stakeInfo)

	if node.CurrentStakes[testAddress].Amount.Cmp(amount) != 0 {
		t.Error("Stake Info is not updated correctly")
	}
}
