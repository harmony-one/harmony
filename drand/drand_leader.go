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
				// TODO: use real vrf
				pRnd := [32]byte{} //newBlock.Header().Vrf
				zeros := [32]byte{}
				if core.IsEpochBlock(newBlock) && !bytes.Equal(pRnd[:], zeros[:]) {
					// The epoch block should contain the randomness preimage pRnd
					go func() {
						vdf := vdf.New(vdfDifficulty, pRnd)
						outputChannel := vdf.GetOutputChannel()
						start := time.Now()
						vdf.Execute()
						duration := time.Now().Sub(start)
						utils.Logger().Info().Dur("duration", duration).Msg("VDF computation finished")
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
	utils.Logger().Debug().Msg("INITING DRAND")
	dRand.ResetState()
	// Copy over block hash and block header data
	blockHash := epochBlock.Hash()
	copy(dRand.blockHash[:], blockHash[:])

	msgToSend := dRand.constructInitMessage()

	// Leader commit vrf itself
	rand, proof := dRand.vrf(dRand.blockHash)

	(*dRand.vrfs)[dRand.SelfAddress] = append(rand[:], proof...)

	utils.Logger().Info().
		Hex("msg", msgToSend).
		Str("leader.PubKey", dRand.leader.ConsensusPubKey.SerializeToHexStr()).
		Msg("[DRG] sent init")
	dRand.host.SendMessageToGroups([]p2p.GroupID{p2p.NewGroupIDByShardID(p2p.ShardID(dRand.ShardID))}, host.ConstructP2pMessage(byte(17), msgToSend))
}

// ProcessMessageLeader dispatches messages for the leader to corresponding processors.
func (dRand *DRand) ProcessMessageLeader(payload []byte) {
	message := &msg_pb.Message{}
	err := protobuf.Unmarshal(payload, message)

	if err != nil {
		utils.Logger().Error().Err(err).Interface("dRand", dRand).Msg("Failed to unmarshal message payload")
	}

	if message.GetDrand().ShardId != dRand.ShardID {
		utils.Logger().Warn().
			Uint32("myShardId", dRand.ShardID).
			Uint32("receivedShardId", message.GetDrand().ShardId).
			Msg("Received drand message from different shard")
		return
	}

	switch message.Type {
	case msg_pb.MessageType_DRAND_COMMIT:
		dRand.processCommitMessage(message)
	default:
		utils.Logger().Error().
			Uint32("msgType", uint32(message.Type)).
			Interface("dRand", dRand).
			Msg("Unexpected message type")
	}
}

// ProcessMessageValidator dispatches validator's consensus message.
func (dRand *DRand) processCommitMessage(message *msg_pb.Message) {
	utils.Logger().Info().Msg("[DRG] Leader received commit")
	if message.Type != msg_pb.MessageType_DRAND_COMMIT {
		utils.Logger().Error().
			Uint32("expected", uint32(msg_pb.MessageType_DRAND_COMMIT)).
			Uint32("got", uint32(message.Type)).
			Msg("Wrong message type received")
		return
	}

	dRand.mutex.Lock()
	defer dRand.mutex.Unlock()

	drandMsg := message.GetDrand()

	senderPubKey, err := bls.BytesToBlsPublicKey(drandMsg.SenderPubkey)
	if err != nil {
		utils.Logger().Debug().Err(err).Msg("Failed to deserialize BLS public key")
		return
	}
	validatorAddress := senderPubKey.SerializeToHexStr()

	if !dRand.IsValidatorInCommittee(validatorAddress) {
		utils.Logger().Error().Str("validatorAddress", validatorAddress).Msg("Invalid validator")
		return
	}

	vrfs := dRand.vrfs
	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		utils.Logger().Debug().
			Str("validatorAddress", validatorAddress).Msg("Received additional randomness commit message")
		return
	}

	// Verify message signature
	err = verifyMessageSig(senderPubKey, message)
	if err != nil {
		utils.Logger().Debug().
			Err(err).Str("PubKey", senderPubKey.SerializeToHexStr()).Msg("[DRAND] failed to verify the message signature")
		return
	}

	rand := drandMsg.Payload[:32]
	proof := drandMsg.Payload[32 : len(drandMsg.Payload)-64]
	pubKeyBytes := drandMsg.Payload[len(drandMsg.Payload)-64:]
	_, vrfPubKey := p256.GenerateKey()
	vrfPubKey.Deserialize(pubKeyBytes)

	expectedRand, err := vrfPubKey.ProofToHash(dRand.blockHash[:], proof)

	if err != nil || !bytes.Equal(expectedRand[:], rand) {
		utils.Logger().Error().
			Err(err).
			Str("validatorAddress", validatorAddress).
			Hex("expectedRand", expectedRand[:]).
			Hex("receivedRand", rand[:]).
			Msg("[DRAND] Failed to verify the VRF")
		return
	}

	utils.Logger().Debug().
		Int("numReceivedSoFar", len((*vrfs))).
		Str("validatorAddress", validatorAddress).
		Int("PublicKeys", len(dRand.PublicKeys)).
		Msg("Received new VRF commit")

	(*vrfs)[validatorAddress] = drandMsg.Payload
	dRand.bitmap.SetKey(senderPubKey, true) // Set the bitmap indicating that this validator signed.

	if len((*vrfs)) >= ((len(dRand.PublicKeys))/3 + 1) {
		// Construct pRand and initiate consensus on it
		utils.Logger().Debug().
			Int("numReceivedSoFar", len((*vrfs))).
			Str("validatorAddress", validatorAddress).
			Int("PublicKeys", len(dRand.PublicKeys)).
			Msg("[DRAND] {BINGO} Received enough randomness commit")

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
