package drand

import (
	"bytes"
	"time"

	"github.com/harmony-one/harmony/crypto/bls"

	protobuf "github.com/golang/protobuf/proto"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
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

	(*dRand.vrfs)[dRand.SelfAddress] = append(rand[:], proof...)

	utils.GetLogInstance().Info("[DRG] sent init", "msg", msgToSend, "leader.PubKey", dRand.leader.ConsensusPubKey)
	dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(dRand.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
}

// ProcessMessageLeader dispatches messages for the leader to corresponding processors.
func (dRand *DRand) ProcessMessageLeader(payload []byte) {
	message := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, message)

	if err != nil {
		utils.GetLogInstance().Error("Failed to unmarshal message payload.", "err", err, "dRand", dRand)
	}

	switch message.Type {
	case msg_pb.MessageType_DRAND_COMMIT:
		dRand.processCommitMessage(message)
	default:
		utils.GetLogInstance().Error("Unexpected message type", "msgType", message.Type, "dRand", dRand)
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processCommitMessage(message *msg_pb.Message) {
	utils.GetLogInstance().Info("[DRG] Leader received commit")
	if message.Type != msg_pb.MessageType_DRAND_COMMIT {
		utils.GetLogInstance().Error("Wrong message type received", "expected", msg_pb.MessageType_DRAND_COMMIT, "got", message.Type)
		return
	}

	dRand.mutex.Lock()
	defer dRand.mutex.Unlock()

	drandMsg := message.GetDrand()

	senderPubKey, err := bls.BytesToBlsPublicKey(drandMsg.SenderPubkey)
	if err != nil {
		utils.GetLogInstance().Debug("Failed to deserialize BLS public key", "error", err)
		return
	}
	validatorAddress := utils.GetBlsAddress(senderPubKey)

	if !dRand.IsValidatorInCommittee(validatorAddress) {
		utils.GetLogInstance().Error("Invalid validator", "validatorAddress", validatorAddress)
		return
	}

	vrfs := dRand.vrfs
	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		utils.GetLogInstance().Debug("Received additional randomness commit message", "validatorAddress", validatorAddress)
		return
	}

	// Verify message signature
	err = verifyMessageSig(senderPubKey, message)
	if err != nil {
		utils.GetLogInstance().Warn("[DRAND] failed to verify the message signature", "Error", err, "PubKey", senderPubKey)
		return
	}

	rand := drandMsg.Payload[:32]
	proof := drandMsg.Payload[32 : len(drandMsg.Payload)-64]
	pubKeyBytes := drandMsg.Payload[len(drandMsg.Payload)-64:]
	_, vrfPubKey := p256.GenerateKey()
	vrfPubKey.Deserialize(pubKeyBytes)

	expectedRand, err := vrfPubKey.ProofToHash(dRand.blockHash[:], proof)

	if err != nil || !bytes.Equal(expectedRand[:], rand) {
		utils.GetLogInstance().Error("[DRAND] Failed to verify the VRF", "error", err, "validatorAddress", validatorAddress, "expectedRand", expectedRand, "receivedRand", rand)
		return
	}

	utils.GetLogInstance().Debug("Received new VRF commit", "numReceivedSoFar", len((*vrfs)), "validatorAddress", validatorAddress, "PublicKeys", len(dRand.PublicKeys))

	(*vrfs)[validatorAddress] = drandMsg.Payload
	dRand.bitmap.SetKey(senderPubKey, true) // Set the bitmap indicating that this validator signed.

	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		// Construct pRand and initiate consensus on it
		utils.GetLogInstance().Debug("[DRAND] {BINGO} Received enough randomness commit", "numReceivedSoFar", len((*vrfs)), "validatorAddress", validatorAddress, "PublicKeys", len(dRand.PublicKeys))

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
