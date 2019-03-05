package drand

import (
	"bytes"
	"time"

	protobuf "github.com/golang/protobuf/proto"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/crypto/vdf"
	"github.com/harmony-one/harmony/crypto/vrf/p256"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
	"github.com/harmony-one/harmony/p2p/host"
)

const (
	vdfDifficulty = 5000000 // This takes about 20s to finish the vdf
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
				pRnd := newBlock.Header().RandPreimage
				zeros := [32]byte{}
				if core.IsEpochBlock(newBlock) && !bytes.Equal(pRnd[:], zeros[:]) {
					// The epoch block should contain the randomness preimage pRnd
					go func() {
						vdf := vdf.New(vdfDifficulty, pRnd)
						outputChannel := vdf.GetOutputChannel()
						start := time.Now()
						vdf.Execute()
						duration := time.Now().Sub(start)
						utils.GetLogInstance().Info("VDF computation finished", "time spent", duration.String())
						output := <-outputChannel

						rndBytes := [64]byte{} // The first 32 bytes are the randomness and the last 32 bytes are the hash of the block where the corresponding pRnd was generated
						copy(rndBytes[:32], output[:])

						blockHash := newBlock.Hash()
						copy(rndBytes[32:], blockHash[:])

						dRand.RndChannel <- rndBytes
					}()
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

	utils.GetLogInstance().Info("[DRG] sent init", "msg", msgToSend, "leader.PubKey", dRand.leader.BlsPubKey)
	dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.GroupIDBeacon}, host.ConstructP2pMessage(byte(17), msgToSend))
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

	dRand.mutex.Lock()
	defer dRand.mutex.Unlock()

	validatorID := message.SenderId
	validatorPeer := dRand.getValidatorPeerByID(validatorID)
	vrfs := dRand.vrfs
	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		utils.GetLogInstance().Debug("Received additional randomness commit message", "validatorID", validatorID)
		return
	}

	// Verify message signature
	err := verifyMessageSig(validatorPeer.BlsPubKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("[DRAND] failed to verify the message signature", "Error", err, "PubKey", validatorPeer.BlsPubKey)
		return
	}

	rand := message.Payload[:32]
	proof := message.Payload[32 : len(message.Payload)-64]
	pubKeyBytes := message.Payload[len(message.Payload)-64:]
	_, pubKey := p256.GenerateKey()
	pubKey.Deserialize(pubKeyBytes)

	expectedRand, err := pubKey.ProofToHash(dRand.blockHash[:], proof)

	if err != nil || !bytes.Equal(expectedRand[:], rand) {
		utils.GetLogInstance().Error("[DRAND] Failed to verify the VRF", "error", err, "validatorID", validatorID, "expectedRand", expectedRand, "receivedRand", rand)
		return
	}

	utils.GetLogInstance().Debug("Received new VRF commit", "numReceivedSoFar", len((*vrfs)), "validatorID", validatorID, "PublicKeys", len(dRand.PublicKeys))

	(*vrfs)[validatorID] = message.Payload
	dRand.bitmap.SetKey(validatorPeer.BlsPubKey, true) // Set the bitmap indicating that this validator signed.

	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		// Construct pRand and initiate consensus on it
		utils.GetLogInstance().Debug("[DRAND] {BINGO} Received enough randomness commit", "numReceivedSoFar", len((*vrfs)), "validatorID", validatorID, "PublicKeys", len(dRand.PublicKeys))

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
