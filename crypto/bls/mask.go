package bls

import (
	"github.com/pkg/errors"
)

// Mask represents a cosigning participation bitmask.
type Mask struct {
	Bitmap          []byte
	Publics         []PublicKey
	PublicsIndex    map[SerializedPublicKey]int
	aggregatePublic publicKeyAggregator
}

func (m *Mask) AggregatePublic() PublicKey {
	return m.aggregatePublic.affine()
}

// SeparateSigAndMask parse the commig signature data into signature and bitmap.
func SeparateSigAndMask(commitSigs []byte) ([]byte, []byte, error) {
	if len(commitSigs) < SignatureSize {
		return nil, nil, errors.Errorf("no mask data found in commit sigs: %x", commitSigs)
	}
	//#### Read payload data from committed msg
	aggSig := make([]byte, SignatureSize)
	bitmap := make([]byte, len(commitSigs)-SignatureSize)
	offset := 0
	copy(aggSig[:], commitSigs[offset:offset+SignatureSize])
	offset += SignatureSize
	copy(bitmap[:], commitSigs[offset:])
	//#### END Read payload data from committed msg
	return aggSig, bitmap, nil
}

// NewMask returns a new participation bitmask for cosigning where all
// cosigners are disabled by default. If a public key is given it verifies that
// it is present in the list of keys and sets the corresponding index in the
// bitmask to 1 (enabled).
func NewMask(publics []PublicKey, myKey PublicKey) (*Mask, error) {
	index := map[SerializedPublicKey]int{}
	publicKeys := make([]PublicKey, len(publics))
	for i, key := range publics {
		publicKeys[i] = publics[i]
		index[key.Serialized()] = i
	}
	m := &Mask{
		Publics:      publicKeys,
		PublicsIndex: index,
	}
	m.Bitmap = make([]byte, m.Len())
	m.aggregatePublic = newPublicKeyAggregator()
	if myKey != nil {
		i, found := m.PublicsIndex[myKey.Serialized()]
		if found {
			m.SetBit(i, true)
			found = true
		}
		if !found {
			return nil, errors.New("key not found")
		}
	}
	return m, nil
}

// Clear clears the existing bits and aggregate public keys.
func (m *Mask) Clear() {
	m.Bitmap = make([]byte, m.Len())
	m.aggregatePublic = newPublicKeyAggregator()
}

// Mask returns a copy of the participation bitmask.
func (m *Mask) Mask() []byte {
	clone := make([]byte, len(m.Bitmap))
	copy(clone[:], m.Bitmap)
	return clone
}

// Len returns the Bitmap length in bytes.
func (m *Mask) Len() int {
	return (len(m.Publics) + 7) >> 3
}

// SetMask sets the participation bitmask according to the given byte slice
// interpreted in little-endian order, i.e., bits 0-7 of byte 0 correspond to
// cosigners 0-7, bits 0-7 of byte 1 correspond to cosigners 8-15, etc.
func (m *Mask) SetMask(mask []byte) error {
	if m.Len() != len(mask) {
		return errors.Errorf(
			"mismatching bitmap lengths expectedBitmapLength %d providedBitmapLength %d",
			m.Len(),
			len(mask),
		)
	}
	for i := range m.Publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if ((m.Bitmap[byt] & msk) == 0) && ((mask[byt] & msk) != 0) {
			m.Bitmap[byt] ^= msk // flip bit in Bitmap from 0 to 1
			m.aggregatePublic.add(m.Publics[i])
		}
		if ((m.Bitmap[byt] & msk) != 0) && ((mask[byt] & msk) == 0) {
			m.Bitmap[byt] ^= msk // flip bit in Bitmap from 1 to 0
			m.aggregatePublic.sub(m.Publics[i])
		}
	}
	return nil
}

// SetBit enables (enable: true) or disables (enable: false) the bit
// in the participation Bitmap of the given cosigner.
func (m *Mask) SetBit(i int, enable bool) error {
	if i >= len(m.Publics) {
		return errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	if ((m.Bitmap[byt] & msk) == 0) && enable {
		m.Bitmap[byt] ^= msk // flip bit in Bitmap from 0 to 1
		m.aggregatePublic.add(m.Publics[i])
	}
	if ((m.Bitmap[byt] & msk) != 0) && !enable {
		m.Bitmap[byt] ^= msk // flip bit in Bitmap from 1 to 0
		m.aggregatePublic.sub(m.Publics[i])
	}
	return nil
}

// GetPubKeyFromMask will return pubkeys which masked either zero or one depending on the flag
// it is used to show which signers are signed or not in the cosign message
func (m *Mask) GetPubKeyFromMask(flag bool) []PublicKey {
	pubKeys := []PublicKey{}
	for i := range m.Publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if flag {
			if (m.Bitmap[byt] & msk) != 0 {
				pubKeys = append(pubKeys, m.Publics[i])
			}
		} else {
			if (m.Bitmap[byt] & msk) == 0 {
				pubKeys = append(pubKeys, m.Publics[i])
			}
		}
	}
	return pubKeys
}

// GetSignedPubKeysFromBitmap will return pubkeys that are signed based on the specified bitmap.
func (m *Mask) GetSignedPubKeysFromBitmap(bitmap []byte) ([]PublicKey, error) {
	if m.Len() != len(bitmap) {
		return nil, errors.Errorf(
			"mismatching bitmap lengths expectedBitmapLength %d providedBitmapLength %d",
			m.Len(),
			len(bitmap),
		)
	}
	pubKeys := []PublicKey{}
	// For details about who bitmap is structured, refer to func SetMask
	for i := range m.Publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if (bitmap[byt] & msk) != 0 {
			pubKeys = append(pubKeys, m.Publics[i])
		}
	}
	return pubKeys, nil
}

// IndexEnabled checks whether the given index is enabled in the Bitmap or not.
func (m *Mask) IndexEnabled(i int) (bool, error) {
	if i >= len(m.Publics) {
		return false, errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	return ((m.Bitmap[byt] & msk) != 0), nil
}

// KeyEnabled checks whether the index, corresponding to the given key, is
// enabled in the Bitmap or not.
func (m *Mask) KeyEnabled(public SerializedPublicKey) (bool, error) {
	i, found := m.PublicsIndex[public]
	if found {
		return m.IndexEnabled(i)
	}
	return false, errors.New("key not found")
}

// SetKey set the bit in the Bitmap for the given cosigner
func (m *Mask) SetKey(public SerializedPublicKey, enable bool) error {
	i, found := m.PublicsIndex[public]
	if found {
		return m.SetBit(i, enable)
	}
	return errors.New("key not found")
}

// SetKeysAtomic set the bit in the Bitmap for the given cosigners only when all the cosigners are present in the map.
func (m *Mask) SetKeysAtomic(publics []PublicKey, enable bool) error {
	indexes := make([]int, len(publics))
	for i, key := range publics {
		index, found := m.PublicsIndex[key.Serialized()]
		if !found {
			return errors.New("key not found")
		}
		indexes[i] = index
	}
	for _, index := range indexes {
		err := m.SetBit(index, enable)
		if err != nil {
			return err
		}
	}
	return nil
}

// CountEnabled returns the number of enabled nodes in the CoSi participation
// Bitmap.
func (m *Mask) CountEnabled() int {
	// hw is hamming weight
	hw := 0
	for i := range m.Publics {
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
	return len(m.Publics)
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
