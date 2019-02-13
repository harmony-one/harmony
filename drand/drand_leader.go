package drand

import (
	protobuf "github.com/golang/protobuf/proto"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p/host"
)

// WaitForEpochBlock waits for the first epoch block to run DRG on
func (dRand *DRand) WaitForEpochBlock(blockChannel chan *types.Block, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		defer close(stoppedChan)
		for {
			select {
			default:
				// keep waiting for epoch block
				newBlock := <-blockChannel
				if core.IsEpochLastBlock(newBlock) {
					dRand.init(newBlock)
				}
			case <-stopChan:
				return
			}
		}
	}()
}

func (dRand *DRand) init(epochBlock *types.Block) {
	utils.GetLogInstance().Debug("INITING DRAND")
	dRand.ResetState()
	// Copy over block hash and block header data
	blockHash := epochBlock.Hash()
	copy(dRand.blockHash[:], blockHash[:])

	msgToSend := dRand.constructInitMessage()

	// Leader commit vrf itself
	rand, proof := dRand.vrf(dRand.blockHash)

	(*dRand.vrfs)[dRand.nodeID] = append(rand[:], proof...)

	host.BroadcastMessageFromLeader(dRand.host, dRand.GetValidatorPeers(), msgToSend, nil)
}

// ProcessMessageLeader dispatches messages for the leader to corresponding processors.
func (dRand *DRand) ProcessMessageLeader(payload []byte) {
	message := drand_proto.Message{}
	err := protobuf.Unmarshal(payload, &message)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "dRand", dRand)
	}

	switch message.Type {
	case drand_proto.MessageType_COMMIT:
		dRand.processCommitMessage(message)
	default:
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "dRand", dRand)
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processCommitMessage(message drand_proto.Message) {
	if message.Type != drand_proto.MessageType_COMMIT {
		utils.GetLogInstance().Error("Wrong message type received", "expected", drand_proto.MessageType_COMMIT, "got", message.Type)
		return
	}

	validatorID := message.SenderId
	validatorPeer := dRand.getValidatorPeerByID(validatorID)
	vrfs := dRand.vrfs
	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		utils.GetLogInstance().Debug("Received additional randomness commit message", "validatorID", validatorID)
		return
	}

	// Verify message signature
	err := verifyMessageSig(validatorPeer.PubKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("Failed to verify the message signature", "Error", err)
		return
	}

	rand := message.Payload[:32]
	proof := message.Payload[32:]
	_ = rand
	_ = proof
	// TODO: check the validity of the vrf commit

	utils.GetLogInstance().Debug("Received new commit", "numReceivedSoFar", len((*vrfs)), "validatorID", validatorID, "PublicKeys", len(dRand.PublicKeys))

	(*vrfs)[validatorID] = message.Payload
	dRand.bitmap.SetKey(validatorPeer.PubKey, true) // Set the bitmap indicating that this validator signed.

	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		// Construct pRand and initiate consensus on it
		utils.GetLogInstance().Debug("Received enough randomness commit", "numReceivedSoFar", len((*vrfs)), "validatorID", validatorID, "PublicKeys", len(dRand.PublicKeys))
		// TODO: communicate the pRand to consensus

		pRnd := [32]byte{}
		// Bitwise XOR on all the submitted vrfs
		for _, vrf := range *vrfs {
			for i := 0; i < len(pRnd); i++ {
				pRnd[i] = pRnd[i] ^ vrf[i]
			}
		}
		dRand.PRndChannel <- append(pRnd[:], dRand.bitmap.Bitmap...)
	}
}
