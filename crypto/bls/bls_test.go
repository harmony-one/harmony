package bls

import (
	"bytes"
	"encoding/hex"
	"testing"

	"crypto/rand"
)

// test cases
// * empty
// * equality

const n = 100

func randPublicKey() PublicKey {
	return RandSecretKey().PublicKey()
}

func randSignature() Signature {
	secretKey := RandSecretKey()
	message := make([]byte, 32)
	_, err := rand.Read(message)
	if err != nil {
		panic(err)
	}
	return secretKey.Sign(message)
}

func TestSecretKeySerialization(t *testing.T) {

	test := func(t *testing.T) {
		var err error
		_, err = SecretKeyFromBytes(zeroSecretKey)
		if err != errZeroSecretKey {
			t.Fatalf("avoid zero secret key, lib: %s", BLSLibrary)
		}
		shortSecretKey := make([]byte, 31)
		shortSecretKey[0] = 1
		_, err = SecretKeyFromBytes(shortSecretKey)
		if err != errSecretKeySize {
			t.Fatalf("short secret key, lib: %s", BLSLibrary)
		}
		longSecretKey := make([]byte, 33)
		longSecretKey[0] = 1
		_, err = SecretKeyFromBytes(longSecretKey)
		if err != errSecretKeySize {
			t.Fatalf("long secret key, lib: %s", BLSLibrary)
		}
		secretKeyBytes, err := hex.DecodeString("73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000001")
		if err != nil {
			t.Fatal(err)
		}
		_, err = SecretKeyFromBytes(secretKeyBytes)
		if err != errInvalidSecretKey {
			t.Fatalf("avoid large secret key, lib: %s", BLSLibrary)
		}
		secretKeyBytes, err = hex.DecodeString("73eda753299d7d483339d80809a1d80553bda402fffe5bfeffffffff00000000")
		if err != nil {
			t.Fatal(err)
		}
		secretKey, err := SecretKeyFromBytes(secretKeyBytes)
		if err != nil {
			t.Fatalf("valid secret key must pass, lib: %s", BLSLibrary)
		}
		if !bytes.Equal(secretKeyBytes, secretKey.ToBytes()) {
			t.Fatalf("serialization failed, a, lib: %s", BLSLibrary)
		}
		for i := 0; i < n; i++ {
			secretKey1 := RandSecretKey()
			if bytes.Equal(secretKey1.ToBytes(), zeroSecretKey) {
				t.Fatalf("random generates zero secret key lib: %s", BLSLibrary)
			}
			secretKey2, err := SecretKeyFromBytes(secretKey1.ToBytes())
			if err != nil {
				t.Fatalf("serialization failed, b, lib: %s", BLSLibrary)
			}
			if !secretKey1.equal(secretKey2) {
				t.Fatalf("serialization failed, c, lib: %s", BLSLibrary)
			}
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func TestPublicKeySerialization(t *testing.T) {
	test := func(t *testing.T) {
		var err error
		_, err = PublicKeyFromBytes(zeroPublicKey)
		if err != errZeroPublicKey {
			t.Fatalf("avoid zero public key, lib: %s", BLSLibrary)
		}
		_, err = PublicKeyFromBytes(infinitePublicKey)
		if err != errInfinitePublicKey {
			t.Fatalf("avoid infinite public key, lib: %s", BLSLibrary)
		}
		shortPublicKey := make([]byte, 47)
		shortPublicKey[0] = 1
		_, err = PublicKeyFromBytes(shortPublicKey)
		if err != errPublicKeySize {
			t.Fatalf("short public key, lib: %s", BLSLibrary)
		}
		longPublicKey := make([]byte, 49)
		longPublicKey[0] = 1
		_, err = PublicKeyFromBytes(longPublicKey)
		if err != errPublicKeySize {
			t.Fatalf("long public key, lib: %s", BLSLibrary)
		}
		for i := 0; i < n; i++ {
			publicKey1 := randPublicKey()
			publicKey2, err := PublicKeyFromBytes(publicKey1.ToBytes())
			if err != nil {
				t.Fatalf("serialization failed, b, lib: %s", BLSLibrary)
			}
			if !publicKey1.Equal(publicKey2) {
				t.Fatalf("serialization failed, c, lib: %s", BLSLibrary)
			}
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func TestSignatureSerialization(t *testing.T) {
	test := func(t *testing.T) {
		var err error
		_, err = SignatureFromBytes(zeroSignature)
		if err != errZeroSignature {
			t.Fatalf("avoid zero signature, lib: %s", BLSLibrary)
		}
		_, err = SignatureFromBytes(infiniteSignature)
		if err != errInfiniteSignature {
			t.Fatalf("avoid infinite signature, lib: %s", BLSLibrary)
		}
		shortSignature := make([]byte, 95)
		shortSignature[0] = 1
		_, err = SignatureFromBytes(shortSignature)
		if err != errSignatureSize {
			t.Fatalf("short signature, lib: %s", BLSLibrary)
		}
		longSignature := make([]byte, 97)
		longSignature[0] = 1
		_, err = SignatureFromBytes(longSignature)
		if err != errSignatureSize {
			t.Fatalf("long signature, lib: %s", BLSLibrary)
		}
		for i := 0; i < n; i++ {
			signature1 := randSignature()
			signature2, err := SignatureFromBytes(signature1.ToBytes())
			if err != nil {
				t.Fatalf("serialization failed, b, lib: %s", BLSLibrary)
			}
			if !signature1.Equal(signature2) {
				t.Fatalf("serialization failed, c, lib: %s", BLSLibrary)
			}
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func TestVerify(t *testing.T) {
	test := func(t *testing.T) {
		message1, message2 := []byte("test 1"), []byte("test 2")

		secretKey1 := RandSecretKey()
		publicKey1 := secretKey1.PublicKey()
		secretKey2 := RandSecretKey()
		publicKey2 := secretKey2.PublicKey()

		signature := secretKey1.Sign(message1)

		if !signature.Verify(publicKey1, message1) {
			t.Fatalf("must be verified, lib: %s", BLSLibrary)
		}
		if signature.Verify(publicKey1, message2) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
		if signature.Verify(publicKey2, message1) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func TestFastAggregateVerify(t *testing.T) {

	const nPublicKeys = 10

	test := func(t *testing.T) {
		message1, message2 := []byte("test 1"), []byte("test 2")

		publicKeys := make([]PublicKey, nPublicKeys)
		signatures := make([]Signature, nPublicKeys)
		missingPublicKeys := make([]PublicKey, nPublicKeys-1)
		missingSignatures := make([]Signature, nPublicKeys-1)
		for i := 0; i < nPublicKeys; i++ {
			secretKey := RandSecretKey()
			publicKey := secretKey.PublicKey()
			signature := secretKey.Sign(message1)

			signatures[i] = signature
			publicKeys[i] = publicKey
			if i != nPublicKeys-1 {
				missingPublicKeys[i] = publicKey
				missingSignatures[i] = signature
			}
		}

		aggregatedSignature := AggreagateSignatures(signatures)
		badAggregatedSignature := AggreagateSignatures(missingSignatures)

		if !aggregatedSignature.FastAggregateVerify(publicKeys, message1) {
			t.Fatalf("must be verified, lib: %s", BLSLibrary)
		}
		if aggregatedSignature.FastAggregateVerify(publicKeys, message2) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
		if aggregatedSignature.FastAggregateVerify(missingPublicKeys, message1) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
		if badAggregatedSignature.FastAggregateVerify(publicKeys, message1) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func TestAggregateVerify(t *testing.T) {

	const nPublicKeys = 10

	test := func(t *testing.T) {

		messages1 := make([][]byte, nPublicKeys)
		messages2 := make([][]byte, nPublicKeys)
		publicKeys := make([]PublicKey, nPublicKeys)
		signatures := make([]Signature, nPublicKeys)
		missingSignatures := make([]Signature, nPublicKeys-1)
		for i := 0; i < nPublicKeys; i++ {
			message1 := make([]byte, 32)
			rand.Read(message1)
			message2 := make([]byte, 32)
			rand.Read(message2)
			secretKey := RandSecretKey()
			publicKey := secretKey.PublicKey()
			signature := secretKey.Sign(message1)

			messages1[i] = message1
			messages2[i] = message2
			signatures[i] = signature
			publicKeys[i] = publicKey
			if i != nPublicKeys-1 {
				missingSignatures[i] = signature
			}
		}
		aggregatedSignature := AggreagateSignatures(signatures)
		badAggregatedSignature := AggreagateSignatures(missingSignatures)

		if !aggregatedSignature.AggregateVerify(publicKeys, messages1) {
			t.Fatalf("must be verified, lib: %s", BLSLibrary)
		}
		if aggregatedSignature.AggregateVerify(publicKeys, messages2) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
		if badAggregatedSignature.AggregateVerify(publicKeys, messages1) {
			t.Fatalf("must not be verified, lib: %s", BLSLibrary)
		}
	}
	UseHerumi()
	test(t)
	UseBLST()
	test(t)
}

func benchmarkVerify(t *testing.B) {
	message := []byte("test 1")
	secretKey := RandSecretKey()
	publicKey := secretKey.PublicKey()
	signature := secretKey.Sign(message)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		signature.Verify(publicKey, message)
	}
}

func BenchmarkVerifyHerumi(t *testing.B) {
	UseHerumi()
	benchmarkVerify(t)
}

func BenchmarkVerifyBLST(t *testing.B) {
	UseBLST()
	benchmarkVerify(t)
}

func benchmarkFastAggregateVerify(n int, t *testing.B) {
	message := []byte("test")
	publicKeys := make([]PublicKey, n)
	signatures := make([]Signature, n)
	for i := 0; i < n; i++ {
		secretKey := RandSecretKey()
		publicKey := secretKey.PublicKey()
		signature := secretKey.Sign(message)
		signatures[i] = signature
		publicKeys[i] = publicKey
	}
	aggregatedSignature := AggreagateSignatures(signatures)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		aggregatedSignature.FastAggregateVerify(publicKeys, message)
	}
}

func BenchmarkFastAggregateVerifyHerumi(t *testing.B) {
	UseHerumi()
	for _, i := range []int{10, 100, 100} {
		benchmarkFastAggregateVerify(i, t)
	}
}

func BenchmarkFastAggregateVerifyBLST(t *testing.B) {
	UseBLST()
	for _, i := range []int{10, 100, 100} {
		benchmarkFastAggregateVerify(i, t)
	}
}

func benchmarkAggregateVerify(n int, t *testing.B) {

	messages := make([][]byte, n)
	publicKeys := make([]PublicKey, n)
	signatures := make([]Signature, n)

	for i := 0; i < n; i++ {
		message := make([]byte, 32)
		rand.Read(message)
		secretKey := RandSecretKey()
		publicKey := secretKey.PublicKey()
		signature := secretKey.Sign(message)
		signatures[i] = signature
		publicKeys[i] = publicKey
		messages[i] = message
	}
	aggregatedSignature := AggreagateSignatures(signatures)
	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		aggregatedSignature.AggregateVerify(publicKeys, messages)
	}
}

func BenchmarkAggregateVerifyHerumi(t *testing.B) {
	UseHerumi()
	for _, i := range []int{10, 100, 100} {
		benchmarkAggregateVerify(i, t)
	}
}

func BenchmarkAggregateVerifyBLST(t *testing.B) {
	UseBLST()
	for _, i := range []int{10, 100, 100} {
		benchmarkAggregateVerify(i, t)
	}
}
