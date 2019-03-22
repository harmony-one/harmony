package bls

import (
	"errors"
	"fmt"

	"github.com/harmony-one/bls/ffi/go/bls"
)

func init() {
	bls.Init(bls.BLS12_381)
}

// RandPrivateKey returns a random private key.
func RandPrivateKey() *bls.SecretKey {
	sec := bls.SecretKey{}
	sec.SetByCSPRNG()
	return &sec
}

// BytesToBlsPublicKey converts bytes into bls.PublicKey pointer.
func BytesToBlsPublicKey(bytes []byte) (*bls.PublicKey, error) {
	pubKey := &bls.PublicKey{}
	err := pubKey.Deserialize(bytes)
	return pubKey, err
}

// AggregateSig aggregates all the BLS signature into a single multi-signature.
func AggregateSig(sigs []*bls.Sign) *bls.Sign {
	var aggregatedSig bls.Sign
	for _, sig := range sigs {
		aggregatedSig.Add(sig)
	}
	return &aggregatedSig
}

// Mask represents a cosigning participation bitmask.
type Mask struct {
	Bitmap          []byte
	publics         []*bls.PublicKey
	AggregatePublic *bls.PublicKey
}

// NewMask returns a new participation bitmask for cosigning where all
// cosigners are disabled by default. If a public key is given it verifies that
// it is present in the list of keys and sets the corresponding index in the
// bitmask to 1 (enabled).
func NewMask(publics []*bls.PublicKey, myKey *bls.PublicKey) (*Mask, error) {
	m := &Mask{
		publics: publics,
	}
	m.Bitmap = make([]byte, m.Len())
	m.AggregatePublic = &bls.PublicKey{}
	if myKey != nil {
		found := false
		for i, key := range publics {
			if key.IsEqual(myKey) {
				m.SetBit(i, true)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("key not found")
		}
	}
	return m, nil
}

// Mask returns a copy of the participation bitmask.
func (m *Mask) Mask() []byte {
	clone := make([]byte, len(m.Bitmap))
	copy(clone[:], m.Bitmap)
	return clone
}

// Len returns the Bitmap length in bytes.
func (m *Mask) Len() int {
	return (len(m.publics) + 7) >> 3
}

// SetMask sets the participation bitmask according to the given byte slice
// interpreted in little-endian order, i.e., bits 0-7 of byte 0 correspond to
// cosigners 0-7, bits 0-7 of byte 1 correspond to cosigners 8-15, etc.
func (m *Mask) SetMask(mask []byte) error {
	if m.Len() != len(mask) {
		return fmt.Errorf("mismatching Bitmap lengths")
	}
	for i := range m.publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if ((m.Bitmap[byt] & msk) == 0) && ((mask[byt] & msk) != 0) {
			m.Bitmap[byt] ^= msk // flip bit in Bitmap from 0 to 1
			m.AggregatePublic.Add(m.publics[i])
		}
		if ((m.Bitmap[byt] & msk) != 0) && ((mask[byt] & msk) == 0) {
			m.Bitmap[byt] ^= msk // flip bit in Bitmap from 1 to 0
			m.AggregatePublic.Sub(m.publics[i])
		}
	}
	return nil
}

// SetBit enables (enable: true) or disables (enable: false) the bit
// in the participation Bitmap of the given cosigner.
func (m *Mask) SetBit(i int, enable bool) error {
	if i >= len(m.publics) {
		return errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	if ((m.Bitmap[byt] & msk) == 0) && enable {
		m.Bitmap[byt] ^= msk // flip bit in Bitmap from 0 to 1
		m.AggregatePublic.Add(m.publics[i])
	}
	if ((m.Bitmap[byt] & msk) != 0) && !enable {
		m.Bitmap[byt] ^= msk // flip bit in Bitmap from 1 to 0
		m.AggregatePublic.Sub(m.publics[i])
	}
	return nil
}

// GetPubKeyFromMask will return pubkeys which masked either zero or one depending on the flag
// it is used to show which signers are signed or not in the cosign message
func (m *Mask) GetPubKeyFromMask(flag bool) []*bls.PublicKey {
	pubKeys := []*bls.PublicKey{}
	for i := range m.publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if flag == true {
			if (m.Bitmap[byt] & msk) != 0 {
				pubKeys = append(pubKeys, m.publics[i])
			}
		} else {
			if (m.Bitmap[byt] & msk) == 0 {
				pubKeys = append(pubKeys, m.publics[i])
			}
		}
	}
	return pubKeys
}

// IndexEnabled checks whether the given index is enabled in the Bitmap or not.
func (m *Mask) IndexEnabled(i int) (bool, error) {
	if i >= len(m.publics) {
		return false, errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	return ((m.Bitmap[byt] & msk) != 0), nil
}

// KeyEnabled checks whether the index, corresponding to the given key, is
// enabled in the Bitmap or not.
func (m *Mask) KeyEnabled(public *bls.PublicKey) (bool, error) {
	for i, key := range m.publics {
		if key.IsEqual(public) {
			return m.IndexEnabled(i)
		}
	}
	return false, errors.New("key not found")
}

// SetKey set the bit in the Bitmap for the given cosigner
func (m *Mask) SetKey(public *bls.PublicKey, enable bool) error {
	for i, key := range m.publics {
		if key.IsEqual(public) {
			return m.SetBit(i, enable)
		}
	}
	return errors.New("key not found")
}

// CountEnabled returns the number of enabled nodes in the CoSi participation
// Bitmap.
func (m *Mask) CountEnabled() int {
	// hw is hamming weight
	hw := 0
	for i := range m.publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if (m.Bitmap[byt] & msk) != 0 {
			hw++
		}
	}
	return hw
}

// CountTotal returns the total number of nodes this CoSi instance knows.
func (m *Mask) CountTotal() int {
	return len(m.publics)
}

// AggregateMasks computes the bitwise OR of the two given participation masks.
func AggregateMasks(a, b []byte) ([]byte, error) {
	if len(a) != len(b) {
		return nil, errors.New("mismatching Bitmap lengths")
	}
	m := make([]byte, len(a))
	for i := range m {
		m[i] = a[i] | b[i]
	}
	return m, nil
}

// Policy represents a fully customizable cosigning policy deciding what
// cosigner sets are and aren't sufficient for a collective signature to be
// considered acceptable to a verifier. The Check method may inspect the set of
// participants that cosigned by invoking cosi.Mask and/or cosi.MaskBit, and may
// use any other relevant contextual information (e.g., how security-critical
// the operation relying on the collective signature is) in determining whether
// the collective signature was produced by an acceptable set of cosigners.
type Policy interface {
	Check(m *Mask) bool
}

// CompletePolicy is the default policy requiring that all participants have
// cosigned to make a collective signature valid.
type CompletePolicy struct {
}

// Check verifies that all participants have contributed to a collective
// signature.
func (p CompletePolicy) Check(m *Mask) bool {
	return m.CountEnabled() == m.CountTotal()
}

// ThresholdPolicy allows to specify a simple t-of-n policy requring that at
// least the given threshold number of participants t have cosigned to make a
// collective signature valid.
type ThresholdPolicy struct {
	thold int
}

// NewThresholdPolicy returns a new ThresholdPolicy with the given threshold.
func NewThresholdPolicy(thold int) *ThresholdPolicy {
	return &ThresholdPolicy{thold: thold}
}

// Check verifies that at least a threshold number of participants have
// contributed to a collective signature.
func (p ThresholdPolicy) Check(m *Mask) bool {
	return m.CountEnabled() >= p.thold
}
