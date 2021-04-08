package bls

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	blst "github.com/supranational/blst/bindings/go"
)

type blstPublicKey = blst.P1Affine
type _blstPublicKeyAggregator = blst.P1Aggregate
type blstSecretKey = blst.SecretKey
type blstSignature = blst.P2Affine

type BLSTPublicKey struct {
	p *blstPublicKey
}

type blstPublicKeyAggregator struct {
	p *_blstPublicKeyAggregator
}

type BLSTSecretKey struct {
	s *blstSecretKey
}

type BLSTSignature struct {
	p *blstSignature
}

var blstOptionCheckSignatureSubgroupInVerification = false
var blstOptionValidatePublicKeyInVerification = false

func initBLST() {
	// do nothing
}

func randBLSTSecretKey() SecretKey {
	var t [32]byte
	_, _ = rand.Read(t[:])
	secretKey := blst.KeyGen(t[:])
	return &BLSTSecretKey{secretKey}
}

func blstSecretKeyFromBytes(in []byte) (SecretKey, error) {
	secretKey := new(blst.SecretKey)
	secretKey = secretKey.Deserialize(in)
	if secretKey == nil {
		return nil, errInvalidSecretKey
	}
	return &BLSTSecretKey{secretKey}, nil
}

func (secretKey *BLSTSecretKey) equal(other SecretKey) bool {
	return secretKey.s.Equals(other.(*BLSTSecretKey).s)
}

func (secretKey *BLSTSecretKey) PublicKey() PublicKey {
	return &BLSTPublicKey{new(blstPublicKey).From(secretKey.s)}
}

func (secretKey *BLSTSecretKey) Sign(message []byte) Signature {
	blstSignature := new(blstSignature).Sign(secretKey.s, message, dst)
	return &BLSTSignature{blstSignature}
}

func (secretKey *BLSTSecretKey) ToBytes() []byte {
	return secretKey.s.Serialize()
}

func (publicKey *BLSTPublicKey) FromBytes(serialized []byte) (PublicKey, error) {

	if len(serialized) != PublicKeySize {
		return nil, errors.New("invalid public key size")
	}
	blstPublicKey := new(blstPublicKey).Uncompress(serialized)
	if blstPublicKey == nil {
		return nil, errors.New("cannot uncompress given public key in bytes")
	}
	if !blstPublicKey.KeyValidate() {
		return nil, errors.New("invalid BLS public key")
	}
	publicKey.p = blstPublicKey
	return publicKey, nil
}

func (publicKey *BLSTPublicKey) ToBytes() []byte {
	return publicKey.p.Compress()
}

func (publicKey *BLSTPublicKey) ToHex() string {
	return hex.EncodeToString(publicKey.ToBytes())
}

func (publicKey *BLSTPublicKey) Serialized() SerializedPublicKey {
	serialized := [PublicKeySize]byte{}
	copy(serialized[:], publicKey.ToBytes())
	return serialized
}

func (publicKey *BLSTPublicKey) Address() [20]byte {
	address := [20]byte{}
	hash := sha256.Sum256(publicKey.ToBytes())
	copy(address[:], hash[:20])
	return address
}

func (publicKey *BLSTPublicKey) uncompressed() []byte {
	return publicKey.p.Serialize()
}

func (publicKey *BLSTPublicKey) Equal(other PublicKey) bool {
	return publicKey.p.Equals(other.(*BLSTPublicKey).p)
}

func (signature *BLSTSignature) FromBytes(serialized []byte) (Signature, error) {
	if len(serialized) != SignatureSize {
		return nil, errors.New("invalid signature key size")
	}
	blstSignature := new(blstSignature).Uncompress(serialized)
	if blstSignature == nil {
		return nil, errors.New("cannot uncompress given signature key in bytes")
	}
	if !blstSignature.KeyValidate() {
		return nil, errors.New("invalid BLS signature key")
	}
	signature.p = blstSignature
	return signature, nil
}

func (signature *BLSTSignature) ToBytes() []byte {
	return signature.p.Compress()
}

func (signature *BLSTSignature) ToHex() string {
	return hex.EncodeToString(signature.ToBytes())
}

func (signature *BLSTSignature) Serialized() SerializedSignature {
	serialized := [SignatureSize]byte{}
	copy(serialized[:], signature.ToBytes())
	return serialized
}

func (signature *BLSTSignature) Equal(other Signature) bool {
	return signature.p.Equals(other.(*BLSTSignature).p)
}

func (signature *BLSTSignature) Verify(publicKey PublicKey, message []byte) bool {
	return signature.p.
		Verify(
			blstOptionCheckSignatureSubgroupInVerification,
			publicKey.(*BLSTPublicKey).p,
			blstOptionValidatePublicKeyInVerification,
			message,
			dst,
		)
}

func (signature *BLSTSignature) FastAggregateVerify(publicKeys []PublicKey, message []byte) bool {
	blstPublicKeys := make([]*blstPublicKey, len(publicKeys))
	for i := 0; i < len(publicKeys); i++ {
		blstPublicKeys[i] = publicKeys[i].(*BLSTPublicKey).p
	}
	return signature.p.
		FastAggregateVerify(
			blstOptionCheckSignatureSubgroupInVerification,
			blstPublicKeys,
			message[:],
			dst,
		)
}

func (signature *BLSTSignature) AggregateVerify(publicKeys []PublicKey, messages [][]byte) bool {
	size := len(publicKeys)
	if size == 0 {
		return false
	}
	if len(messages) != size {
		return false
	}
	blstPublicKeys := make([]*blstPublicKey, len(publicKeys))
	for i := 0; i < size; i++ {
		blstPublicKeys[i] = publicKeys[i].(*BLSTPublicKey).p
	}
	return signature.p.
		AggregateVerify(
			blstOptionCheckSignatureSubgroupInVerification,
			blstPublicKeys,
			blstOptionValidatePublicKeyInVerification,
			messages,
			dst,
		)
}

func blstAggregateSignature(signatures []Signature) Signature {
	size := len(signatures)
	blstSignatures := make([]*blstSignature, size)
	for i := 0; i < size; i++ {
		blstSignatures[i] = signatures[i].(*BLSTSignature).p
	}
	aggregatedSignature := new(blst.P2Aggregate)
	aggregatedSignature.Aggregate(blstSignatures, false)
	return &BLSTSignature{aggregatedSignature.ToAffine()}
}

func blstAggregatePublicKey(publicKeys []PublicKey) PublicKey {
	size := len(publicKeys)
	blstPublicKeys := make([]*blstPublicKey, size)
	for i := 0; i < size; i++ {
		blstPublicKeys[i] = publicKeys[i].(*BLSTPublicKey).p
	}
	aggregatedPublicKey := new(blst.P1Aggregate)
	aggregatedPublicKey.Aggregate(blstPublicKeys, false)
	return &BLSTPublicKey{aggregatedPublicKey.ToAffine()}
}

func (aggregator *blstPublicKeyAggregator) add(publicKey PublicKey) {
	aggregator.p.Add(publicKey.(*BLSTPublicKey).p, false)
}

func (aggregator *blstPublicKeyAggregator) sub(publicKey PublicKey) {
	aggregator.add(negatePublicKey(publicKey))
}

func (aggregator *blstPublicKeyAggregator) affine() PublicKey {
	return &BLSTPublicKey{aggregator.p.ToAffine()}
}
