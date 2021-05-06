package blsvrf

import (
	"crypto"
	"crypto/sha256"
	"errors"

	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/vrf"
)

var (
	// ErrInvalidVRF occurs when the VRF does not validate.
	ErrInvalidVRF = errors.New("invalid VRF proof")
)

// PublicKey holds a public VRF key.
type PublicKey struct {
	*bls.PublicKey
}

// PrivateKey holds a private VRF key.
type PrivateKey struct {
	*bls.SecretKey
}

func init() {
	bls.Init(bls.BLS12_381)
}

// Public returns the corresponding public key as bytes.
func (k *PrivateKey) Public() crypto.PublicKey {
	return *k.SecretKey.GetPublicKey()
}

// Serialize serialize the public key into bytes
func (pk *PublicKey) Serialize() []byte {
	return pk.Serialize()
}

// Deserialize de-serialize bytes into public key
func (pk *PublicKey) Deserialize(data []byte) {
	pk.Deserialize(data)
}

// NewVRFVerifier creates a verifier object from a public key.
func NewVRFVerifier(pubkey *bls.PublicKey) vrf.PublicKey {
	return &PublicKey{pubkey}
}

// NewVRFSigner creates a signer object from a private key.
func NewVRFSigner(seck *bls.SecretKey) vrf.PrivateKey {
	return &PrivateKey{seck}
}

// Evaluate returns the verifiable unpredictable function evaluated using alpha
// verifiable unpredictable function using BLS
// reference:  https://tools.ietf.org/html/draft-goldbe-vrf-01
// properties of VRF-BLS:
// 1) Full Uniqueness : satisfied, it is deterministic
// 2) Full Pseudorandomness : satisfied through sha256
// 3) Full Collison Resistance : satisfied through sha256
func (k *PrivateKey) Evaluate(alpha []byte) ([32]byte, []byte) {
	//get the BLS signature of the message
	//pi = VRF_prove(SK, alpha)
	msgHash := sha256.Sum256(alpha)
	pi := k.SignHash(msgHash[:])
	if pi == nil {
		return [32]byte{}, nil
	}

	//hash the signature and output as VRF beta
	//beta = VRF_proof2hash(pi)
	beta := sha256.Sum256(pi.Serialize())

	return beta, pi.Serialize()
}

// ProofToHash asserts that proof is correct for input alpha and output VRF hash
func (pk *PublicKey) ProofToHash(alpha, pi []byte) ([32]byte, error) {
	nilIndex := [32]byte{}

	if len(pi) == 0 {
		return nilIndex, ErrInvalidVRF
	}

	msgSig := bls.Sign{}

	err := msgSig.Deserialize(pi)

	if err != nil {
		return nilIndex, err
	}

	msgHash := sha256.Sum256(alpha)
	if !msgSig.VerifyHash(pk.PublicKey, msgHash[:]) {
		return nilIndex, ErrInvalidVRF
	}

	return sha256.Sum256(pi), nil
}
