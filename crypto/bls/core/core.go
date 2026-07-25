// Package core configures the Herumi BLS implementation for Harmony consensus.
package bls

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"unsafe"

	herumi "github.com/herumi/bls-eth-go-binary/bls"
)

type (
	ID        = herumi.ID
	SecretKey = herumi.SecretKey
	PublicKey = herumi.PublicKey
	Sign      = herumi.Sign
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
		return fmt.Errorf("bls: failed to select Harmony map-to-curve mode")
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
	if err := herumi.SetGeneratorOfPublicKey(&generator); err != nil {
		return fmt.Errorf("bls: install Harmony generator: %w", err)
	}
	return nil
}

// GetAddress derives the Harmony address of a BLS public key.
func GetAddress(pub *PublicKey) [20]byte {
	var address [20]byte
	if pub == nil {
		return address
	}
	hash := sha256.Sum256(pub.Serialize())
	copy(address[:], hash[:len(address)])
	return address
}

// Sub subtracts one BLS public key from another.
func Sub(pub, rhs *PublicKey) {
	if pub == nil || rhs == nil {
		return
	}
	out := (*herumi.G1)(unsafe.Pointer(pub))
	herumi.G1Sub(out, out, (*herumi.G1)(unsafe.Pointer(rhs)))
}
