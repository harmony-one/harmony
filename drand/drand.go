package drand

import (
	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"sync"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	drand_proto "github.com/harmony-one/harmony/api/drand"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/p2p"
)

// DRand is the main struct which contains state for the distributed randomness protocol.
type DRand struct {
	vrfs   *map[uint32][32]byte
	bitmap *bls_cosi.Mask
	pRand  *[32]byte
	rand   *[32]byte

	// map of nodeID to validator Peer object
	// FIXME: should use PubKey of p2p.Peer as the hashkey
	validators sync.Map // key is uint16, value is p2p.Peer

	// Leader's address
	leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys []*bls.PublicKey

	// private/public keys of current node
	priKey *bls.SecretKey
	pubKey *bls.PublicKey

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
func New(host p2p.Host, ShardID string, peers []p2p.Peer, leader p2p.Peer) *DRand {
	dRand := DRand{}
	dRand.host = host

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		dRand.IsLeader = true
	} else {
		dRand.IsLeader = false
	}

	dRand.leader = leader
	for _, peer := range peers {
		dRand.validators.Store(utils.GetUniqueIDFromPeer(peer), peer)
	}

	dRand.vrfs = &map[uint32][32]byte{}

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
	for _, validatorPeer := range peers {
		allPublicKeys = append(allPublicKeys, validatorPeer.PubKey)
	}
	allPublicKeys = append(allPublicKeys, leader.PubKey)

	dRand.PublicKeys = allPublicKeys

	bitmap, _ := bls_cosi.NewMask(dRand.PublicKeys, dRand.leader.PubKey)
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

	myShardID, err := strconv.Atoi(ShardID)
	if err != nil {
		panic("Unparseable shard Id" + ShardID)
	}
	dRand.ShardID = uint32(myShardID)

	return &dRand
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

func (dRand *DRand) vrf() (rand [32]byte, proof []byte) {
	// TODO: implement vrf
	return [32]byte{}, []byte{}
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
