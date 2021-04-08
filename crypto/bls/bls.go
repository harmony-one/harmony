package bls

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"

	lru "github.com/hashicorp/golang-lru"
)

const (
	blsPubKeyCacheSize = 1024
)

var (
	// BLSPubKeyCache is the Cache of the Deserialized BLS PubKey
	BLSPubKeyCache, _ = lru.New(blsPubKeyCacheSize)
)

var (
	errPubKeyCast        = errors.New("cast error")
	errZeroSecretKey     = errors.New("zero secret key")
	errZeroPublicKey     = errors.New("zero public key")
	errInfinitePublicKey = errors.New("infinite public key")
	errZeroSignature     = errors.New("zero signature")
	errInfiniteSignature = errors.New("infinite signature")
	errSecretKeySize     = errors.New("invalid secret key size")
	errInvalidSecretKey  = errors.New("invalid secret key")
	errPublicKeySize     = errors.New("invalid public key size")
	errSignatureSize     = errors.New("invalid signature size")
)

// Library selection
const (
	libraryHerumi = "herumi"
	libraryBLST   = "blst"
)

// BLSLib points which library is in use
var BLSLibrary = libraryHerumi

// SignatureSize is size of compressed signature in bytes
const SignatureSize = 96

// PublicKeySize is size of compressed public key in bytes
const PublicKeySize = 48

// SecretKeySize is size of secret key in bytes
const SecretKeySize = 32

var dst = []byte("BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_")

var zeroSecretKey = make([]byte, SecretKeySize)
var zeroPublicKey = make([]byte, PublicKeySize)
var infinitePublicKey []byte
var zeroSignature = make([]byte, SignatureSize)
var infiniteSignature []byte

const tryOlderSeralization = true

var fieldOrder *big.Int

func init() {
	switch BLSLibrary {
	case libraryHerumi:
		initHerumi()
	case libraryBLST:
		initBLST()
	}
	fieldOrderStr := "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaaab"
	var ok bool
	fieldOrder, ok = new(big.Int).SetString(fieldOrderStr, 16)
	if !ok {
		panic("cannot set field order")
	}
	var err error
	infinitePublicKeyStr := "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	infinitePublicKey, err = hex.DecodeString(infinitePublicKeyStr)
	if err != nil {
		panic("cannot set infinite public key")
	}
	infiniteSignatureStr := "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	infiniteSignature, err = hex.DecodeString(infiniteSignatureStr)
	if err != nil {
		panic("cannot set infinite signature")
	}

}

func UseHerumi() {
	BLSLibrary = libraryHerumi
	initHerumi()
}

func UseBLST() {
	BLSLibrary = libraryBLST
}

type SecretKey interface {
	Sign(message []byte) Signature
	PublicKey() PublicKey
	ToBytes() []byte
	equal(SecretKey) bool
}

type PublicKey interface {
	FromBytes(serialized []byte) (PublicKey, error)
	ToBytes() []byte
	Serialized() SerializedPublicKey
	ToHex() string
	Address() [20]byte
	Equal(other PublicKey) bool
	uncompressed() []byte
}

type publicKeyAggregator interface {
	add(PublicKey)
	sub(PublicKey)
	affine() PublicKey
}

type Signature interface {
	FromBytes(serialized []byte) (Signature, error)
	ToBytes() []byte
	Serialized() SerializedSignature
	ToHex() string
	Equal(other Signature) bool
	Verify(publicKey PublicKey, message []byte) bool
	FastAggregateVerify(publicKeys []PublicKey, message []byte) bool
	AggregateVerify(publicKeys []PublicKey, messages [][]byte) bool
}

type SerializedPublicKey [PublicKeySize]byte
type SerializedSignature [SignatureSize]byte

func RandSecretKey() SecretKey {
	var secretKey SecretKey
	switch BLSLibrary {
	case libraryHerumi:
		secretKey = randHerumiSecretKey()
	case libraryBLST:
		secretKey = randBLSTSecretKey()
	}
	return secretKey
}

func SecretKeyFromBytes(_secretKey []byte) (SecretKey, error) {
	if len(_secretKey) != SecretKeySize {
		return nil, errSecretKeySize
	}
	if bytes.Equal(zeroSecretKey, _secretKey) {
		return nil, errZeroSecretKey
	}
	var secretKey SecretKey
	var err error
	switch BLSLibrary {
	case libraryHerumi:
		secretKey, err = herumiSecretKeyFromBytes(_secretKey)
	case libraryBLST:
		secretKey, err = blstSecretKeyFromBytes(_secretKey)
	}
	if err != nil {
		return nil, err
	}
	return secretKey, nil
}

func PublicKeyFromBytes(serialized []byte) (PublicKey, error) {
	if len(serialized) != PublicKeySize {
		return nil, errPublicKeySize
	}
	if bytes.Equal(zeroPublicKey, serialized) {
		return nil, errZeroPublicKey
	}
	if bytes.Equal(infinitePublicKey, serialized) {
		return nil, errInfinitePublicKey
	}
	key := string(serialized)
	if k, ok := BLSPubKeyCache.Get(key); ok {
		if pk, ok := k.(PublicKey); ok {
			return pk, nil
		}
		return nil, errPubKeyCast
	}

	var publicKey PublicKey
	var err error
	switch BLSLibrary {
	case libraryHerumi:
		publicKey, err = new(HerumiPublicKey).FromBytes(serialized)
	case libraryBLST:
		publicKey, err = new(BLSTPublicKey).FromBytes(serialized)
	}
	if err != nil {
		if tryOlderSeralization {
			var errInner error
			publicKey, errInner = publicKeyFromBytesOld(serialized)
			if errInner != nil {
				return nil, err // return original error
			}
		}
	} else {
		BLSPubKeyCache.Add(key, publicKey)
	}

	return publicKey, nil
}

func publicKeyFromBytesOld(serialized []byte) (PublicKey, error) {
	if len(serialized) != PublicKeySize {
		return nil, errPublicKeySize
	}
	_serialized := fixSerializedPublicKey(serialized)
	key := string(_serialized)
	if k, ok := BLSPubKeyCache.Get(key); ok {
		if pk, ok := k.(PublicKey); ok {
			return pk, nil
		}
		return nil, errPubKeyCast
	}

	var publicKey PublicKey
	var err error
	switch BLSLibrary {
	case libraryHerumi:
		publicKey, err = new(HerumiPublicKey).FromBytes(_serialized)
	case libraryBLST:
		publicKey, err = new(BLSTPublicKey).FromBytes(_serialized)
	}
	if err != nil {
		return nil, err
	}

	BLSPubKeyCache.Add(key, publicKey)
	return publicKey, nil
}

func publicKeyFromUncompresesd(uncompressed []byte) (PublicKey, error) {
	if len(uncompressed) != 2*PublicKeySize {
		return nil, errPublicKeySize
	}
	var publicKey PublicKey
	var err error
	switch BLSLibrary {
	case libraryHerumi:
		_publicKey := new(herumiPublicKey)
		err = _publicKey.DeserializeUncompressed(uncompressed)
		publicKey = &HerumiPublicKey{_publicKey}
	case libraryBLST:
		publicKey = &BLSTPublicKey{new(blstPublicKey).Deserialize(uncompressed)}
	}
	if err != nil {
		return nil, err
	}
	return publicKey, nil
}

func newPublicKeyAggregator() publicKeyAggregator {
	var aggregator publicKeyAggregator
	switch BLSLibrary {
	case libraryHerumi:
		aggregator = &herumiPublicKeyAggregator{new(_herumiPublickeyAggregator)}
	case libraryBLST:
		aggregator = &blstPublicKeyAggregator{new(_blstPublicKeyAggregator)}
	}
	return aggregator
}

func SignatureFromBytes(serialized []byte) (Signature, error) {
	if len(serialized) != SignatureSize {
		return nil, errSignatureSize
	}
	if bytes.Equal(zeroSignature, serialized) {
		return nil, errZeroSignature
	}
	if bytes.Equal(infiniteSignature, serialized) {
		return nil, errInfiniteSignature
	}
	var signature Signature
	var err error
	switch BLSLibrary {
	case libraryHerumi:
		signature, err = new(HerumiSignature).FromBytes(serialized)
	case libraryBLST:
		signature, err = new(BLSTSignature).FromBytes(serialized)
	}
	if err != nil {
		return nil, err
	}
	return signature, nil
}

func (serializedPublicKey SerializedPublicKey) ToHex() string {
	return hex.EncodeToString(serializedPublicKey[:])
}

func (serizlizedPublicKey SerializedPublicKey) MarshalText() (text []byte, err error) {
	text = make([]byte, SignatureSize)
	hex.Encode(text, serizlizedPublicKey[:])
	return text, nil
}

func AggreagatePublicKeys(publicKeys []PublicKey) PublicKey {
	var publicKey PublicKey
	switch BLSLibrary {
	case libraryHerumi:
		publicKey = herumiAggregatePublicKey(publicKeys)
	case libraryBLST:
		publicKey = blstAggregatePublicKey(publicKeys)
	}
	return publicKey
}

func AggreagateSignatures(signatures []Signature) Signature {
	var signature Signature
	switch BLSLibrary {
	case libraryHerumi:
		signature = herumiAggregateSignature(signatures)
	case libraryBLST:
		signature = blstAggregateSignature(signatures)
	}
	return signature
}

func negatePublicKey(publicKey PublicKey) PublicKey {
	uncompressed := publicKey.uncompressed()
	y := new(big.Int).Set(fieldOrder)
	// y = p - y
	y.Sub(new(big.Int).SetBytes(uncompressed[48:]), y)
	// pad back to 48 bytes
	yBytes := y.Bytes()
	off := 48 - len(yBytes)
	if off != 0 {
		yBytes = append(make([]byte, off), yBytes...)
	}
	copy(uncompressed[48:], yBytes[:])
	ret, _ := publicKeyFromUncompresesd(uncompressed)
	return ret
}

func fixSerializedPublicKey(old []byte) []byte {
	// reverse bytes order
	fixed := make([]byte, 48)
	for i := 0; i < 48; i++ {
		fixed[i] = old[48-i-1]
	}
	// add compression tag
	fixed[0] = fixed[0] | (1 << 7)
	return fixed
}
