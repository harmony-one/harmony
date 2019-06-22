package drand

import (
	"errors"
	"sync"

	"github.com/harmony-one/harmony/crypto/hash"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/harmony-one/bls/ffi/go/bls"
	msg_pb "github.com/harmony-one/harmony/api/proto/message"
	"github.com/harmony-one/harmony/core/types"
	bls_cosi "github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/crypto/vrf"
	"github.com/harmony-one/harmony/crypto/vrf/p256"
	"github.com/harmony-one/harmony/p2p"
)

// DRand is the main struct which contains state for the distributed randomness protocol.
type DRand struct {
	vrfs                  *map[string][]byte // Key is the address hex
	bitmap                *bls_cosi.Mask
	pRand                 *[32]byte
	rand                  *[32]byte
	ConfirmedBlockChannel chan *types.Block // Channel to receive confirmed blocks
	PRndChannel           chan []byte       // Channel to send pRnd (preimage of randomness resulting from combined vrf randomnesses) to consensus. The first 32 bytes are randomness, the rest is for bitmap.
	RndChannel            chan [64]byte     // Channel for DRG protocol to send the final randomness to consensus. The first 32 bytes are the randomness and the last 32 bytes are the hash of the block where the corresponding pRnd was generated

	// global consensus mutex
	mutex sync.Mutex

	// Leader's address
	leader p2p.Peer

	// Public keys of the committee including leader and validators
	PublicKeys []*bls.PublicKey
	// The addresses of my committee
	CommitteePublicKeys map[string]bool
	pubKeyLock          sync.Mutex

	// private/public keys of current node
	priKey *bls.SecretKey
	pubKey *bls.PublicKey
	// VRF private and public key
	// TODO: directly use signature signing key (BLS) for vrf
	vrfPriKey *vrf.PrivateKey
	vrfPubKey *vrf.PublicKey

	// Whether I am leader. False means I am validator
	IsLeader bool

	// Leader or validator address
	SelfAddress string

	// The p2p host used to send/receive p2p messages
	host p2p.Host

	// Shard Id which this node belongs to
	ShardID uint32

	// Blockhash - 32 byte
	blockHash [32]byte
}

// New creates a new dRand object
func New(host p2p.Host, ShardID uint32, peers []p2p.Peer, leader p2p.Peer, confirmedBlockChannel chan *types.Block, blsPriKey *bls.SecretKey) *DRand {
	dRand := DRand{}
	dRand.host = host

	if confirmedBlockChannel != nil {
		dRand.ConfirmedBlockChannel = confirmedBlockChannel
	}

	dRand.PRndChannel = make(chan []byte)
	dRand.RndChannel = make(chan [64]byte)

	selfPeer := host.GetSelfPeer()
	if leader.Port == selfPeer.Port && leader.IP == selfPeer.IP {
		dRand.IsLeader = true
	} else {
		dRand.IsLeader = false
	}

	dRand.leader = leader
	dRand.CommitteePublicKeys = map[string]bool{}
	for _, peer := range peers {
		dRand.CommitteePublicKeys[peer.ConsensusPubKey.SerializeToHexStr()] = true
	}

	dRand.vrfs = &map[string][]byte{}

	// Initialize cosign bitmap
	allPublicKeys := make([]*bls.PublicKey, 0)
	for _, validatorPeer := range peers {
		allPublicKeys = append(allPublicKeys, validatorPeer.ConsensusPubKey)
	}
	allPublicKeys = append(allPublicKeys, leader.ConsensusPubKey)

	dRand.PublicKeys = allPublicKeys

	bitmap, _ := bls_cosi.NewMask(dRand.PublicKeys, dRand.leader.ConsensusPubKey)
	dRand.bitmap = bitmap

	dRand.pRand = nil
	dRand.rand = nil

	// For now use socket address as ID
	dRand.SelfAddress = selfPeer.ConsensusPubKey.SerializeToHexStr()

	// Set private key for myself so that I can sign messages.
	if blsPriKey != nil {
		dRand.priKey = blsPriKey
		dRand.pubKey = blsPriKey.GetPublicKey()
	}

	// VRF keys
	priKey, pubKey := p256.GenerateKey()
	dRand.vrfPriKey = &priKey
	dRand.vrfPubKey = &pubKey
	dRand.ShardID = ShardID

	return &dRand
}

// Sign on the drand message signature field.
func (dRand *DRand) signDRandMessage(message *msg_pb.Message) error {
	message.Signature = nil
	marshaledMessage, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}
	// 64 byte of signature on previous data
	hash := hash.Keccak256(marshaledMessage)
	signature := dRand.priKey.SignHash(hash[:])

	message.Signature = signature.Serialize()
	return nil
}

// Signs the drand message and returns the marshaled message.
func (dRand *DRand) signAndMarshalDRandMessage(message *msg_pb.Message) ([]byte, error) {
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

// Verify the signature of the message are valid from the signer's public key.
func verifyMessageSig(signerPubKey *bls.PublicKey, message *msg_pb.Message) error {
	signature := message.Signature
	message.Signature = nil
	messageBytes, err := protobuf.Marshal(message)
	if err != nil {
		return err
	}

	msgSig := bls.Sign{}
	err = msgSig.Deserialize(signature)
	if err != nil {
		return err
	}
	msgHash := hash.Keccak256(messageBytes)
	if !msgSig.VerifyHash(signerPubKey, msgHash[:]) {
		return errors.New("failed to verify the signature")
	}
	return nil
}

// IsValidatorInCommittee returns whether the given validator BLS address is part of my committee
func (dRand *DRand) IsValidatorInCommittee(validatorBlsPubKey string) bool {
	_, ok := dRand.CommitteePublicKeys[validatorBlsPubKey]
	return ok
}

// ResetState resets the state of the randomness protocol
func (dRand *DRand) ResetState() {
	dRand.vrfs = &map[string][]byte{}

	bitmap, _ := bls_cosi.NewMask(dRand.PublicKeys, dRand.leader.ConsensusPubKey)
	dRand.bitmap = bitmap
	dRand.pRand = nil
	dRand.rand = nil
}

// SetLeaderPubKey deserialize the public key of drand leader
func (dRand *DRand) SetLeaderPubKey(k []byte) error {
	dRand.leader.ConsensusPubKey = &bls.PublicKey{}
	return dRand.leader.ConsensusPubKey.Deserialize(k)
}

// UpdatePublicKeys updates the PublicKeys variable, protected by a mutex
func (dRand *DRand) UpdatePublicKeys(pubKeys []*bls.PublicKey) int {
	dRand.pubKeyLock.Lock()
	dRand.PublicKeys = append(pubKeys[:0:0], pubKeys...)
	dRand.CommitteePublicKeys = map[string]bool{}
	for _, pubKey := range dRand.PublicKeys {
		dRand.CommitteePublicKeys[pubKey.SerializeToHexStr()] = true
	}
	// TODO: use pubkey to identify leader rather than p2p.Peer.
	dRand.leader = p2p.Peer{ConsensusPubKey: pubKeys[0]}
	dRand.pubKeyLock.Unlock()

	return len(dRand.PublicKeys)
}
