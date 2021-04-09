package bls

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	herumi "github.com/herumi/bls-eth-go-binary/bls"
)

type herumiPublicKey = herumi.PublicKey
type _herumiPublickeyAggregator = herumi.PublicKey
type herumiSecretKey = herumi.SecretKey
type herumiSignature = herumi.Sign

type HerumiPublicKey struct {
	p *herumiPublicKey
}

type herumiPublicKeyAggregator struct {
	p *_herumiPublickeyAggregator
}

type HerumiSecretKey struct {
	s *herumiSecretKey
}

type HerumiSignature struct {
	p *herumiSignature
}

func initHerumi() {
	if err := herumi.Init(herumi.BLS12_381); err != nil {
		panic(err)
	}
	if err := herumi.SetETHmode(herumi.EthModeDraft07); err != nil {
		panic(err)
	}
	herumi.VerifyPublicKeyOrder(true)
	herumi.VerifySignatureOrder(true)
}

func randHerumiSecretKey() SecretKey {
	secretKey := new(herumi.SecretKey)
	secretKey.SetByCSPRNG()
	return &HerumiSecretKey{secretKey}
}

func herumiSecretKeyFromBytes(in []byte) (SecretKey, error) {
	if len(in) != SecretKeySize {
		return nil, errSecretKeySize
	}
	secretKey := new(herumi.SecretKey)
	err := secretKey.Deserialize(in)
	if err != nil {
		return nil, errInvalidSecretKey
	}
	return &HerumiSecretKey{secretKey}, nil
}

func herumiSecretKeyFromBigEndianBytes(in []byte) (SecretKey, error) {
	if len(in) != SecretKeySize {
		return nil, errSecretKeySize
	}
	fixed := make([]byte, SecretKeySize)
	for i := 0; i < SecretKeySize; i++ {
		fixed[i] = in[SecretKeySize-i-1]
	}
	secretKey := new(herumi.SecretKey)
	err := secretKey.Deserialize(fixed)
	if err != nil {
		return nil, errInvalidSecretKey
	}
	return &HerumiSecretKey{secretKey}, nil
}

func (secretKey *HerumiSecretKey) equal(other SecretKey) bool {
	return secretKey.s.IsEqual(other.(*HerumiSecretKey).s)
}

func (secretKey *HerumiSecretKey) PublicKey() PublicKey {
	return &HerumiPublicKey{secretKey.s.GetPublicKey()}
}

func (secretKey *HerumiSecretKey) Sign(message []byte) Signature {
	herumiSignature := secretKey.s.Sign(string(message))
	return &HerumiSignature{herumiSignature}
}

func (secretKey *HerumiSecretKey) ToBytes() []byte {
	return secretKey.s.Serialize()
}

func (secretKey *HerumiSecretKey) ToBigEndianBytes() []byte {
	le := secretKey.ToBytes()
	be := make([]byte, SecretKeySize)
	for i := 0; i < SecretKeySize; i++ {
		be[i] = le[SecretKeySize-i-1]
	}
	return be
}

func (publicKey *HerumiPublicKey) ToBigEndianBytes() []byte {
	le := publicKey.ToBytes()
	be := make([]byte, PublicKeySize)
	for i := 0; i < PublicKeySize; i++ {
		be[i] = le[PublicKeySize-i-1]
	}
	return be
}

func (publicKey *HerumiPublicKey) FromBytes(serialized []byte) (PublicKey, error) {
	if len(serialized) != PublicKeySize {
		return nil, errPublicKeySize
	}
	herumiPublicKey := new(herumiPublicKey)
	if err := herumiPublicKey.Deserialize(serialized); err != nil {
		return nil, err
	}
	if !herumiPublicKey.IsValidOrder() {
		return nil, errors.New("invalid public key order")
	}
	if herumiPublicKey.IsZero() {
		return nil, errZeroPublicKey
	}
	publicKey.p = herumiPublicKey
	return publicKey, nil
}

func (publicKey *HerumiPublicKey) ToBytes() []byte {
	return publicKey.p.Serialize()
}

func (publicKey *HerumiPublicKey) ToHex() string {
	return hex.EncodeToString(publicKey.ToBytes())
}

func (publicKey *HerumiPublicKey) Serialized() SerializedPublicKey {
	serialized := [PublicKeySize]byte{}
	copy(serialized[:], publicKey.ToBytes())
	return serialized
}

func (publicKey *HerumiPublicKey) Address() [20]byte {
	address := [20]byte{}
	hash := sha256.Sum256(publicKey.ToBytes())
	copy(address[:], hash[:20])
	return address
}

func (publicKey *HerumiPublicKey) uncompressed() []byte {
	return publicKey.p.SerializeUncompressed()
}

func (publicKey *HerumiPublicKey) Equal(other PublicKey) bool {
	return publicKey.p.IsEqual(other.(*HerumiPublicKey).p)
}

func (signature *HerumiSignature) FromBytes(serialized []byte) (Signature, error) {
	if len(serialized) != SignatureSize {
		return nil, errSignatureSize
	}
	herumiSignature := new(herumiSignature)
	if err := herumiSignature.Deserialize(serialized); err != nil {
		return nil, err
	}
	if !herumiSignature.IsValidOrder() {
		return nil, errors.New("invalid signature order")
	}
	if herumiSignature.IsZero() {
		return nil, errZeroSignature
	}
	signature.p = herumiSignature
	return signature, nil
}

func (signature *HerumiSignature) ToBytes() []byte {
	return signature.p.Serialize()
}

func (signature *HerumiSignature) ToHex() string {
	return hex.EncodeToString(signature.ToBytes())
}

func (signature *HerumiSignature) Equal(other Signature) bool {
	return signature.p.IsEqual(other.(*HerumiSignature).p)
}

func (signature *HerumiSignature) Serialized() SerializedSignature {
	serialized := [SignatureSize]byte{}
	copy(serialized[:], signature.ToBytes())
	return serialized
}

func (signature *HerumiSignature) Verify(publicKey PublicKey, message []byte) bool {
	return signature.p.
		Verify(
			publicKey.(*HerumiPublicKey).p,
			string(message),
		)
}

func (signature *HerumiSignature) FastAggregateVerify(publicKeys []PublicKey, message []byte) bool {
	if len(publicKeys) == 0 {
		return false
	}
	herumiPublicKeys := make([]herumiPublicKey, len(publicKeys))
	for i := 0; i < len(publicKeys); i++ {
		herumiPublicKeys[i] = *publicKeys[i].(*HerumiPublicKey).p
	}
	return signature.p.
		FastAggregateVerify(
			herumiPublicKeys,
			message[:],
		)
}

func (signature *HerumiSignature) AggregateVerify(publicKeys []PublicKey, messages [][]byte) bool {
	size := len(publicKeys)
	if size == 0 {
		return false
	}
	if len(messages) != size {
		return false
	}
	herumiPublicKeys := make([]herumiPublicKey, size)
	_messages := []byte{}
	for i := 0; i < size; i++ {
		herumiPublicKeys[i] = *publicKeys[i].(*HerumiPublicKey).p
		_messages = append(_messages, messages[i][:]...)
	}
	return signature.p.
		AggregateVerify(
			herumiPublicKeys,
			_messages,
		)
}

func herumiAggregateSignature(signatures []Signature) Signature {
	size := len(signatures)
	herimuSignatures := make([]herumiSignature, size)
	for i := 0; i < size; i++ {
		herimuSignatures[i] = *signatures[i].(*HerumiSignature).p
	}
	aggregatedSignature := new(herumiSignature)
	aggregatedSignature.Aggregate(herimuSignatures)
	return &HerumiSignature{aggregatedSignature}
}

func herumiAggregatePublicKey(publicKeys []PublicKey) PublicKey {
	size := len(publicKeys)
	aggregatedPublicKey := new(herumiPublicKey)
	for i := 0; i < size; i++ {
		aggregatedPublicKey.Add(publicKeys[i].(*HerumiPublicKey).p)
	}
	return &HerumiPublicKey{aggregatedPublicKey}
}

func (aggregator *herumiPublicKeyAggregator) add(publicKey PublicKey) {
	aggregator.p.Add(publicKey.(*HerumiPublicKey).p)
}

func (aggregator *herumiPublicKeyAggregator) sub(publicKey PublicKey) {
	aggregator.add(negatePublicKey(publicKey))
}

func (aggregator *herumiPublicKeyAggregator) affine() PublicKey {
	return &HerumiPublicKey{aggregator.p}
}
