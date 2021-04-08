package blsvrf

import (
	"crypto/sha256"
	"errors"

	"github.com/harmony-one/harmony/crypto/bls"
)

var (
	// ErrInvalidVRF occurs when the VRF does not validate.
	ErrInvalidVRF = errors.New("invalid VRF proof")
)

// Evaluate returns the verifiable unpredictable function evaluated using alpha
// verifiable unpredictable function using BLS
// reference:  https://tools.ietf.org/html/draft-goldbe-vrf-01
// properties of VRF-BLS:
// 1) Full Uniqueness : satisfied, it is deterministic
// 2) Full Pseudorandomness : satisfied through sha256
// 3) Full Collison Resistance : satisfied through sha256
func Evaluate(k bls.SecretKey, alpha []byte) ([32]byte, []byte) {
	//get the BLS signature of the message
	//pi = VRF_prove(SK, alpha)
	msgHash := sha256.Sum256(alpha)
	pi := k.Sign(msgHash[:])

	//hash the signature and output as VRF beta
	//beta = VRF_proof2hash(pi)
	beta := sha256.Sum256(pi.ToBytes())

	return beta, pi.ToBytes()
}

// ProofToHash asserts that proof is correct for input alpha and output VRF hash
func ProofToHash(pk bls.PublicKey, alpha, pi []byte) ([32]byte, error) {
	nilIndex := [32]byte{}

	if len(pi) == 0 {
		return nilIndex, ErrInvalidVRF
	}

	msgSig, err := bls.SignatureFromBytes(pi)
	if err != nil {
		return nilIndex, err
	}

	msgHash := sha256.Sum256(alpha)
	if !msgSig.Verify(pk, msgHash[:]) {
		return nilIndex, ErrInvalidVRF
	}

	return sha256.Sum256(pi), nil
}
