package drand

import (
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/p2p/host"
)

// WaitForEpochBlock waits for the first epoch block to run DRG on
func (dRand *DRand) WaitForEpochBlock(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				// keep waiting for new blocks
				newBlock := <-blockChannel
				// TODO: think about potential race condition

				dRand.init(newBlock)
			case <-stopChan:
				return
			}
		}
	}()
}

func (dRand *DRand) init(epochBlock *types.Block) {
	// Copy over block hash and block header data
	blockHash := epochBlock.Hash()
	copy(dRand.blockHash[:], blockHash[:])

	msgToSend := dRand.constructInitMessage()

	// Leader commit vrf itself
	(*dRand.vrfs)[dRand.nodeID], _ = dRand.vrf()

	host.BroadcastMessageFromLeader(dRand.host, dRand.GetValidatorPeers(), msgToSend, nil)
}
