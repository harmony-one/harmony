/*
Package crypto implements the collective signing (CoSi) algorithm as presented in
the paper "Keeping Authorities 'Honest or Bust' with Decentralized Witness
Cosigning" by Ewa Syta et al. See https://arxiv.org/abs/1503.08768. This
package only provides the functionality for the cryptographic operations of
CoSi. All network-related operations have to be handled elsewhere. Below we
describe a high-level overview of the CoSi protocol (using a star communication
topology). We refer to the research paper for further details on communication
over trees, exception mechanisms and signature verification policies.

The CoSi protocol has four phases executed between a list of participants P
having a protocol leader (index i = 0) and a list of other nodes (index i > 0).
The secret key of node i is denoted by a_i and the public key by A_i = [a_i]G
(where G is the base point of the underlying group and [...] denotes scalar
multiplication). The aggregate public key is given as A = \sum{i ∈ P}(A_i).

1. Announcement: The leader broadcasts an announcement to the other nodes
optionally including the message M to be signed. Upon receiving an announcement
message, a node starts its commitment phase.

2. Commitment: Each node i (including the leader) picks a random scalar v_i,
computes its commitment V_i = [v_i]G and sends V_i back to the leader. The
leader waits until it has received enough commitments (according to some
policy) from the other nodes or a timer has run out. Let P' be the nodes that
have sent their commitments. The leader computes an aggregate commitment V from
all commitments he has received, i.e., V = \sum{j ∈ P'}(V_j) and creates a
participation bitmask Z. The leader then broadcasts V and Z to the other
participations together with the message M if it was not sent in phase 1. Upon
receiving a commitment message, a node starts the challenge phase.

3. Challenge: Each node i computes the collective challenge c = H(V || A || M)
using a cryptographic hash function H (here: SHA512), computes its
response r_i = v_i + c*a_i and sends it back to the leader.

4. Response: The leader waits until he has received replies from all nodes in
P' or a timer has run out. If he has not enough replies he aborts. Finally,
the leader computes the aggregate response r = \sum{j ∈ P'}(r_j) and publishes
(V,r,Z) as the signature for the message M.
*/
package crypto

import (
	"errors"
	"fmt"

	"github.com/dedis/kyber"
)

// Commit returns a random scalar v, generated from the given suite,
// and a corresponding commitment V = [v]G. If the given cipher stream is nil,
// a random stream is used.
func Commit(suite Suite) (v kyber.Scalar, V kyber.Point) {
	random := suite.Scalar().Pick(suite.RandomStream())
	commitment := suite.Point().Mul(random, nil)
	return random, commitment
}

// AggregateCommitments returns the sum of the given commitments and the
// bitwise OR of the corresponding masks.
func AggregateCommitments(suite Suite, commitments []kyber.Point, masks [][]byte) (sum kyber.Point, commits []byte, err error) {
	if len(commitments) != len(masks) {
		return nil, nil, errors.New("mismatching lengths of commitment and mask slices")
	}
	aggCom := suite.Point().Null()
	aggMask := make([]byte, len(masks[0]))

	for i := range commitments {
		aggCom = suite.Point().Add(aggCom, commitments[i])
		aggMask, err = AggregateMasks(aggMask, masks[i])
		if err != nil {
			return nil, nil, err
		}
	}
	return aggCom, aggMask, nil
}

// AggregateCommitmentsOnly returns the sum of the given commitments.
func AggregateCommitmentsOnly(suite Suite, commitments []kyber.Point) kyber.Point {
	aggCom := suite.Point().Null()

	for i := range commitments {
		aggCom = suite.Point().Add(aggCom, commitments[i])
	}
	return aggCom
}

// Challenge creates the collective challenge from the given aggregate
// commitment V, aggregate public key A, and message M, i.e., it returns
// c = H(V || A || M).
func Challenge(suite Suite, commitment, public kyber.Point, message []byte) (kyber.Scalar, error) {
	if commitment == nil {
		return nil, errors.New("no commitment provided")
	}
	if message == nil {
		return nil, errors.New("no message provided")
	}
	hash := suite.Hash()
	if _, err := commitment.MarshalTo(hash); err != nil {
		return nil, err
	}
	if _, err := public.MarshalTo(hash); err != nil {
		return nil, err
	}
	hash.Write(message)
	return suite.Scalar().SetBytes(hash.Sum(nil)), nil
}

// Response creates the response from the given random scalar v, (collective)
// challenge c, and private key a, i.e., it returns r = v + c*a.
func Response(suite Suite, private, random, challenge kyber.Scalar) (kyber.Scalar, error) {
	if private == nil {
		return nil, errors.New("no private key provided")
	}
	if random == nil {
		return nil, errors.New("no random scalar provided")
	}
	if challenge == nil {
		return nil, errors.New("no challenge provided")
	}
	// TODO: figure out why in the paper it says r = v - cx
	ca := suite.Scalar().Mul(private, challenge)
	return ca.Add(random, ca), nil
}

// AggregateResponses returns the sum of given responses.
func AggregateResponses(suite Suite, responses []kyber.Scalar) (kyber.Scalar, error) {
	if responses == nil {
		return nil, errors.New("no responses provided")
	}
	r := suite.Scalar().Zero()
	for i := range responses {
		r = r.Add(r, responses[i])
	}
	return r, nil
}

// Sign returns the collective signature from the given (aggregate) commitment
// V, (aggregate) response r, and participation bitmask Z using the EdDSA
// format, i.e., the signature is V || r || Z.
func Sign(suite Suite, commitment kyber.Point, response kyber.Scalar, mask *Mask) ([]byte, error) {
	if commitment == nil {
		return nil, errors.New("no commitment provided")
	}
	if response == nil {
		return nil, errors.New("no response provided")
	}
	if mask == nil {
		return nil, errors.New("no mask provided")
	}
	lenV := suite.PointLen()
	lenSig := lenV + suite.ScalarLen()
	VB, err := commitment.MarshalBinary()
	if err != nil {
		return nil, errors.New("marshalling of commitment failed")
	}
	RB, err := response.MarshalBinary()
	if err != nil {
		return nil, errors.New("marshalling of signature failed")
	}
	sig := make([]byte, lenSig+mask.Len())
	copy(sig[:], VB)
	copy(sig[lenV:lenSig], RB)
	copy(sig[lenSig:], mask.mask)
	return sig, nil
}

// Verify checks the given cosignature on the provided message using the list
// of public keys and cosigning policy.
func Verify(suite Suite, publics []kyber.Point, message, sig []byte, policy Policy) error {
	if publics == nil {
		return errors.New("no public keys provided")
	}
	if message == nil {
		return errors.New("no message provided")
	}
	if sig == nil {
		return errors.New("no signature provided")
	}
	if policy == nil {
		policy = CompletePolicy{}
	}

	lenCom := suite.PointLen()
	VBuff := sig[:lenCom]
	V := suite.Point()
	if err := V.UnmarshalBinary(VBuff); err != nil {
		return errors.New("unmarshalling of commitment failed")
	}

	// Unpack the aggregate response
	lenRes := lenCom + suite.ScalarLen()
	rBuff := sig[lenCom:lenRes]
	r := suite.Scalar().SetBytes(rBuff)

	// Unpack the participation mask and get the aggregate public key
	mask, err := NewMask(suite, publics, nil)
	if err != nil {
		return err
	}
	mask.SetMask(sig[lenRes:])
	A := mask.AggregatePublic
	ABuff, err := A.MarshalBinary()
	if err != nil {
		return errors.New("marshalling of aggregate public key failed")
	}

	// Recompute the challenge
	hash := suite.Hash()
	hash.Write(VBuff)
	hash.Write(ABuff)
	hash.Write(message)
	buff := hash.Sum(nil)
	k := suite.Scalar().SetBytes(buff)

	// k * -aggPublic + s * B = k*-A + s*B
	// from s = k * a + r => s * B = k * a * B + r * B <=> s*B = k*A + r*B
	// <=> s*B + k*-A = r*B
	minusPublic := suite.Point().Neg(A)
	kA := suite.Point().Mul(k, minusPublic)
	sB := suite.Point().Mul(r, nil)
	left := suite.Point().Add(kA, sB)

	if !left.Equal(V) {
		return errors.New("recreated response is different from signature")
	}
	if !policy.Check(mask) {
		return errors.New("the policy is not fulfilled")
	}

	return nil
}

// Mask represents a cosigning participation bitmask.
type Mask struct {
	mask            []byte
	publics         []kyber.Point
	AggregatePublic kyber.Point
}

// NewMask returns a new participation bitmask for cosigning where all
// cosigners are disabled by default. If a public key is given it verifies that
// it is present in the list of keys and sets the corresponding index in the
// bitmask to 1 (enabled).
func NewMask(suite Suite, publics []kyber.Point, myKey kyber.Point) (*Mask, error) {
	m := &Mask{
		publics: publics,
	}
	m.mask = make([]byte, m.Len())
	m.AggregatePublic = suite.Point().Null()
	if myKey != nil {
		found := false
		for i, key := range publics {
			if key.Equal(myKey) {
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
	clone := make([]byte, len(m.mask))
	copy(clone[:], m.mask)
	return clone
}

// Len returns the mask length in bytes.
func (m *Mask) Len() int {
	return (len(m.publics) + 7) >> 3
}

// SetMask sets the participation bitmask according to the given byte slice
// interpreted in little-endian order, i.e., bits 0-7 of byte 0 correspond to
// cosigners 0-7, bits 0-7 of byte 1 correspond to cosigners 8-15, etc.
func (m *Mask) SetMask(mask []byte) error {
	if m.Len() != len(mask) {
		return fmt.Errorf("mismatching mask lengths")
	}
	for i := range m.publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if ((m.mask[byt] & msk) == 0) && ((mask[byt] & msk) != 0) {
			m.mask[byt] ^= msk // flip bit in mask from 0 to 1
			m.AggregatePublic.Add(m.AggregatePublic, m.publics[i])
		}
		if ((m.mask[byt] & msk) != 0) && ((mask[byt] & msk) == 0) {
			m.mask[byt] ^= msk // flip bit in mask from 1 to 0
			m.AggregatePublic.Sub(m.AggregatePublic, m.publics[i])
		}
	}
	return nil
}

// SetBit enables (enable: true) or disables (enable: false) the bit
// in the participation mask of the given cosigner.
func (m *Mask) SetBit(i int, enable bool) error {
	if i >= len(m.publics) {
		return errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	if ((m.mask[byt] & msk) == 0) && enable {
		m.mask[byt] ^= msk // flip bit in mask from 0 to 1
		m.AggregatePublic.Add(m.AggregatePublic, m.publics[i])
	}
	if ((m.mask[byt] & msk) != 0) && !enable {
		m.mask[byt] ^= msk // flip bit in mask from 1 to 0
		m.AggregatePublic.Sub(m.AggregatePublic, m.publics[i])
	}
	return nil
}

// IndexEnabled checks whether the given index is enabled in the mask or not.
func (m *Mask) IndexEnabled(i int) (bool, error) {
	if i >= len(m.publics) {
		return false, errors.New("index out of range")
	}
	byt := i >> 3
	msk := byte(1) << uint(i&7)
	return ((m.mask[byt] & msk) != 0), nil
}

// KeyEnabled checks whether the index, corresponding to the given key, is
// enabled in the mask or not.
func (m *Mask) KeyEnabled(public kyber.Point) (bool, error) {
	for i, key := range m.publics {
		if key.Equal(public) {
			return m.IndexEnabled(i)
		}
	}
	return false, errors.New("key not found")
}

// SetKey set the bit in the mask for the given cosigner
func (m *Mask) SetKey(public kyber.Point, enable bool) error {
	for i, key := range m.publics {
		if key.Equal(public) {
			return m.SetBit(i, enable)
		}
	}
	return errors.New("key not found")
}

// CountEnabled returns the number of enabled nodes in the CoSi participation
// mask.
func (m *Mask) CountEnabled() int {
	// hw is hamming weight
	hw := 0
	for i := range m.publics {
		byt := i >> 3
		msk := byte(1) << uint(i&7)
		if (m.mask[byt] & msk) != 0 {
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
		return nil, errors.New("mismatching mask lengths")
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
