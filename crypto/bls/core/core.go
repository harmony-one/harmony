// Package bls configures the Herumi BLS implementation for Harmony consensus.
package bls

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"unsafe"

	herumi "github.com/herumi/bls-eth-go-binary/bls"
)

type (
	ID = herumi.ID

	// SecretKey wraps Herumi's key with Harmony-safe cgo boundaries.
	SecretKey struct{ herumi.SecretKey }

	// PublicKey wraps Herumi's public key with Harmony-specific operations.
	PublicKey struct{ herumi.PublicKey }

	// Sign wraps a Herumi BLS signature.
	Sign struct{ herumi.Sign }
)

const BLS12_381 = herumi.BLS12_381

// This is the G1 generator produced by the historical Harmony BLS_SWAP_G
// implementation. Keeping it preserves all existing public keys.
const harmonyGenerator = "e500361ff315734cccd8f9b721ec159995e9e622be17afd41ac2f037a583e81b98c2320e0bf853a8f929e89e3d8ff504"

func init() {
	if err := Init(BLS12_381); err != nil {
		panic(err)
	}
}

// Init initializes Herumi with Harmony's consensus-critical group layout,
// serialization, hash-to-curve mode and public-key generator.
func Init(curve int) error {
	if err := herumi.Init(curve); err != nil {
		return err
	}
	herumi.SetETHserialization(false)
	if err := herumi.SetMapToMode(0); err != nil {
		return fmt.Errorf("bls: failed to select Harmony map-to-curve mode: %w", err)
	}
	if err := herumi.SetETHmode(herumi.EthModeOld); err != nil {
		return err
	}
	raw, err := hex.DecodeString(harmonyGenerator)
	if err != nil {
		return err
	}
	var generator PublicKey
	if err := generator.Deserialize(raw); err != nil {
		return fmt.Errorf("bls: decode Harmony generator: %w", err)
	}
	if err := herumi.SetGeneratorOfPublicKey(&generator.PublicKey); err != nil {
		return fmt.Errorf("bls: install Harmony generator: %w", err)
	}
	return nil
}

// GetAddress derives the Harmony address of a BLS public key.
func (pub *PublicKey) GetAddress() [20]byte {
	var address [20]byte
	if pub == nil {
		return address
	}
	hash := sha256.Sum256(pub.Serialize())
	copy(address[:], hash[:len(address)])
	return address
}

// GetPublicKey returns the public key corresponding to this secret key.
func (secret *SecretKey) GetPublicKey() *PublicKey {
	if secret == nil {
		return nil
	}
	public := secret.SecretKey.GetPublicKey()
	if public == nil {
		return nil
	}
	return &PublicKey{PublicKey: *public}
}

// IsEqual reports whether two secret keys are equal.
func (secret *SecretKey) IsEqual(rhs *SecretKey) bool {
	return secret != nil && rhs != nil && secret.SecretKey.IsEqual(&rhs.SecretKey)
}

// Sign signs a string using Harmony's configured BLS mode.
func (secret *SecretKey) Sign(message string) *Sign {
	if secret == nil {
		return nil
	}
	signature := secret.SecretKey.Sign(message)
	if signature == nil {
		return nil
	}
	return &Sign{Sign: *signature}
}

// SignHash copies the hash before crossing the cgo boundary.
func (secret *SecretKey) SignHash(hash []byte) *Sign {
	if secret == nil {
		return nil
	}
	signature := secret.SecretKey.SignHash(append([]byte(nil), hash...))
	if signature == nil {
		return nil
	}
	return &Sign{Sign: *signature}
}

// SignByte copies the message before crossing the cgo boundary.
func (secret *SecretKey) SignByte(message []byte) *Sign {
	if secret == nil {
		return nil
	}
	signature := secret.SecretKey.SignByte(append([]byte(nil), message...))
	if signature == nil {
		return nil
	}
	return &Sign{Sign: *signature}
}

// VerifyHash copies the hash before crossing the cgo boundary.
func (signature *Sign) VerifyHash(public *PublicKey, hash []byte) bool {
	if signature == nil || public == nil {
		return false
	}
	return signature.Sign.VerifyHash(
		&public.PublicKey,
		append([]byte(nil), hash...),
	)
}

// VerifyByte copies the message before crossing the cgo boundary.
func (signature *Sign) VerifyByte(public *PublicKey, message []byte) bool {
	if signature == nil || public == nil {
		return false
	}
	return signature.Sign.VerifyByte(
		&public.PublicKey,
		append([]byte(nil), message...),
	)
}

// Verify verifies a string signature.
func (signature *Sign) Verify(public *PublicKey, message string) bool {
	if signature == nil || public == nil {
		return false
	}
	return signature.Sign.Verify(&public.PublicKey, message)
}

// Add adds another public key.
func (pub *PublicKey) Add(rhs *PublicKey) {
	if pub != nil && rhs != nil {
		pub.PublicKey.Add(&rhs.PublicKey)
	}
}

// IsEqual reports whether two public keys are equal.
func (pub *PublicKey) IsEqual(rhs *PublicKey) bool {
	return pub != nil && rhs != nil && pub.PublicKey.IsEqual(&rhs.PublicKey)
}

// Sub subtracts another BLS public key.
func (pub *PublicKey) Sub(rhs *PublicKey) {
	if pub == nil || rhs == nil {
		return
	}
	out := (*herumi.G1)(unsafe.Pointer(&pub.PublicKey))
	herumi.G1Sub(out, out, (*herumi.G1)(unsafe.Pointer(&rhs.PublicKey)))
}

// Add adds another signature.
func (signature *Sign) Add(rhs *Sign) {
	if signature != nil && rhs != nil {
		signature.Sign.Add(&rhs.Sign)
	}
}

// IsEqual reports whether two signatures are equal.
func (signature *Sign) IsEqual(rhs *Sign) bool {
	return signature != nil && rhs != nil && signature.Sign.IsEqual(&rhs.Sign)
}
