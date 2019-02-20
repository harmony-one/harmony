package drand

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strconv"
	"sync"

	"github.com/harmony-one/harmony/crypto/vrf"
	"github.com/harmony-one/harmony/crypto/vrf/p256"

	"github.com/harmony-one/harmony/core/types"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// DRand is the main struct which contains state for the distributed randomness protocol.
type DRand struct {
	vrfs                  *map[uint32][]byte
	bitmap                *bls_cosi.Mask
	pRand                 *[32]byte
	rand                  *[32]byte
	ConfirmedBlockChannel chan *types.Block // Channel to receive confirmed blocks
	PRndChannel           chan []byte       // Channel to send pRnd (preimage of randomness resulting from combined vrf randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.

	// global consensus mutex
	mutex sync.Mutex

	// map of nodeID to validator Peer object
	// FIXME: should use PubKey of p2p.Peer as the hashkey
	validators sync.Map // key is uint16, value is p2p.Peer

	// Leader's address
	Leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys []*bls.PublicKey
	pubKeyLock sync.Mutex

	// private/public keys of current node
	priKey *bls.SecretKey
	pubKey *bls.PublicKey
	// VRF private and public key
	// TODO: directly use signature signing key (BLS) for vrf
	vrfPriKey *vrf.PrivateKey
	vrfPubKey *vrf.PublicKey

	// Whether I am leader. False means I am validator
	IsLeader bool

	// Leader or validator Id - 4 byte
	nodeID uint32

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Shard Id which this node belongs to
	ShardID uint32

	// Blockhash - 32 byte
	blockHash [32]byte
}

// New creates a new dRand object
func New(host p2p.Host, ShardID string, peers []p2p.Peer, leader p2p.Peer, confirmedBlockChannel chan *types.Block) *DRand {
	dRand := DRand{}
	dRand.host = host

	if confirmedBlockChannel != nil {
		dRand.ConfirmedBlockChannel = confirmedBlockChannel
	}

	dRand.PRndChannel = make(chan []byte)

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		dRand.IsLeader = true
	} else {
		dRand.IsLeader = false
	}

	dRand.Leader = leader
	for _, peer := range peers {
		dRand.validators.Store(utils.GetUniqueIDFromPeer(peer), peer)
	}

	dRand.vrfs = &map[uint32][]byte{}

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
	for _, validatorPeer := range peers {
		allPublicKeys = append(allPublicKeys, validatorPeer.PubKey)
	}
	allPublicKeys = append(allPublicKeys, leader.PubKey)

	dRand.PublicKeys = allPublicKeys

	bitmap, _ := bls_cosi.NewMask(dRand.PublicKeys, dRand.Leader.PubKey)
	dRand.bitmap = bitmap

	dRand.pRand = nil
	dRand.rand = nil

	// For now use socket address as ID
	// TODO: populate Id derived from address
	dRand.nodeID = utils.GetUniqueIDFromPeer(selfPeer)

	// Set private key for myself so that I can sign messages.
	nodeIDBytes := make([]byte, 32)
	binary.LittleEndian.PutUint32(nodeIDBytes, dRand.nodeID)
	privateKey := bls.SecretKey{}
	err := privateKey.SetLittleEndian(nodeIDBytes)
	dRand.priKey = &privateKey
	dRand.pubKey = privateKey.GetPublicKey()

	// VRF keys
	priKey, pubKey := p256.GenerateKey()
	dRand.vrfPriKey = &priKey
	dRand.vrfPubKey = &pubKey

	myShardID, err := strconv.Atoi(ShardID)
	if err != nil {
		panic("Unparseable shard Id" + ShardID)
	}
	dRand.ShardID = uint32(myShardID)

	return &dRand
}

// AddPeers adds new peers into the validator map of the consensus
// and add the public keys
func (dRand *DRand) AddPeers(peers []*p2p.Peer) int {
	count := 0

	for _, peer := range peers {
		_, ok := dRand.validators.Load(utils.GetUniqueIDFromPeer(*peer))
		if !ok {
			dRand.validators.Store(utils.GetUniqueIDFromPeer(*peer), *peer)
			dRand.pubKeyLock.Lock()
			dRand.PublicKeys = append(dRand.PublicKeys, peer.PubKey)
			dRand.pubKeyLock.Unlock()
			utils.GetLogInstance().Debug("[DRAND]", "AddPeers", *peer)
		}
		count++
	}
	return count
}

// Sign on the drand message signature field.
func (dRand *DRand) signDRandMessage(message *drand_proto.Message) error {
	message.Signature = nil
	// TODO: use custom serialization method rather than protobuf
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	hash := sha256.Sum256(marshaledMessage)
	signature := dRand.priKey.SignHash(hash[:])

	message.Signature = signature.Serialize()
	return nil
}

// Signs the drand message and returns the marshaled message.
func (dRand *DRand) signAndMarshalDRandMessage(message *drand_proto.Message) ([]byte, error) {
	err := dRand.signDRandMessage(message)
	if err != nil {
		return []byte{}, err
	}

	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return []byte{}, err
	}
	return marshaledMessage, nil
}

func (dRand *DRand) vrf(blockHash [32]byte) (rand [32]byte, proof []byte) {
	rand, proof = (*dRand.vrfPriKey).Evaluate(blockHash[:])
	return
}

// GetValidatorPeers returns list of validator peers.
func (dRand *DRand) GetValidatorPeers() []p2p.Peer {
	validatorPeers := make([]p2p.Peer, 0)

	dRand.validators.Range(func(k, v interface{}) bool {
		if peer, ok := v.(p2p.Peer); ok {
			validatorPeers = append(validatorPeers, peer)
			return true
		}
		return false
	})

	return validatorPeers
}

// Verify the signature of the message are valid from the signer's public key.
func verifyMessageSig(signerPubKey *bls.PublicKey, message drand_proto.Message) error {
	signature := message.Signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(&message)
	if err != nil {
		return err
	}

	msgSig := bls.Sign{}
	err = msgSig.Deserialize(signature)
	if err != nil {
		return err
	}
	msgHash := sha256.Sum256(messageBytes)
	if !msgSig.VerifyHash(signerPubKey, msgHash[:]) {
		return errors.New("failed to verify the signature")
	}
	return nil
}

// Gets the validator peer based on validator ID.
func (dRand *DRand) getValidatorPeerByID(validatorID uint32) *p2p.Peer {
	v, ok := dRand.validators.Load(validatorID)
	if !ok {
		utils.GetLogInstance().Warn("Unrecognized validator", "validatorID", validatorID, "dRand", dRand)
		return nil
	}
	value, ok := v.(p2p.Peer)
	if !ok {
		utils.GetLogInstance().Warn("Invalid validator", "validatorID", validatorID, "dRand", dRand)
		return nil
	}
	return &value
}

// ResetState resets the state of the randomness protocol
func (dRand *DRand) ResetState() {
	dRand.vrfs = &map[uint32][]byte{}

	bitmap, _ := bls_cosi.NewMask(dRand.PublicKeys, dRand.Leader.PubKey)
	dRand.bitmap = bitmap
	dRand.pRand = nil
	dRand.rand = nil
}
