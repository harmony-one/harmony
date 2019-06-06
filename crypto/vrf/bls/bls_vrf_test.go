package blsvrf

import (
	"bytes"
	"math"
	"testing"

	"github.com/harmony-one/harmony/crypto/bls"
)

func TestVRF1(t *testing.T) {
	blsSk := bls.RandPrivateKey()

	vrfSk := NewVRFSigner(blsSk)
	vrfPk := NewVRFVerifier(blsSk.GetPublicKey())

	m1 := []byte("data1")

	vrf, proof := vrfSk.Evaluate(m1)
	hash, err := vrfPk.ProofToHash(m1, proof)

	if err != nil {
		t.Errorf("error generating proof to hash")
	}

	if hash != vrf {
		t.Errorf("error hash doesn't match")
	}
}

func TestVRF2(t *testing.T) {
	blsSk := bls.RandPrivateKey()

	k := NewVRFSigner(blsSk)
	pk := NewVRFVerifier(blsSk.GetPublicKey())

	m1 := []byte("data1")
	m2 := []byte("data2")
	m3 := []byte("data2")
	index1, proof1 := k.Evaluate(m1)
	index2, proof2 := k.Evaluate(m2)
	index3, proof3 := k.Evaluate(m3)
	for _, tc := range []struct {
		m     []byte
		index [32]byte
		proof []byte
		err   error
	}{
		{m1, index1, proof1, nil},
		{m2, index2, proof2, nil},
		{m3, index3, proof3, nil},
		{m3, index3, proof2, nil},
		{m3, index3, proof1, ErrInvalidVRF},
	} {
		index, err := pk.ProofToHash(tc.m, tc.proof)
		if got, want := err, tc.err; got != want {
			t.Errorf("ProofToHash(%s, %x): %v, want %v", tc.m, tc.proof, got, want)
		}
		if err != nil {
			continue
		}
		if got, want := index, tc.index; got != want {
			t.Errorf("ProofToInex(%s, %x): %x, want %x", tc.m, tc.proof, got, want)
		}
	}
}

func TestRightTruncateProof(t *testing.T) {
	blsSk := bls.RandPrivateKey()

	k := NewVRFSigner(blsSk)
	pk := NewVRFVerifier(blsSk.GetPublicKey())

	data := []byte("data")
	_, proof := k.Evaluate(data)
	proofLen := len(proof)
	for i := 0; i < proofLen; i++ {
		proof = proof[:len(proof)-1]
		if i < 47 {
			continue
		}
		if _, err := pk.ProofToHash(data, proof); err == nil {
			t.Errorf("Verify unexpectedly succeeded after truncating %v bytes from the end of proof", i)
		}
	}
}

func TestLeftTruncateProof(t *testing.T) {
	blsSk := bls.RandPrivateKey()

	k := NewVRFSigner(blsSk)
	pk := NewVRFVerifier(blsSk.GetPublicKey())

	data := []byte("data")
	_, proof := k.Evaluate(data)
	proofLen := len(proof)
	for i := 0; i < proofLen; i++ {
		proof = proof[1:]
		if _, err := pk.ProofToHash(data, proof); err == nil {
			t.Errorf("Verify unexpectedly succeeded after truncating %v bytes from the beginning of proof", i)
		}
	}
}

func TestBitFlip(t *testing.T) {
	blsSk := bls.RandPrivateKey()

	k := NewVRFSigner(blsSk)
	pk := NewVRFVerifier(blsSk.GetPublicKey())

	data := []byte("data")
	_, proof := k.Evaluate(data)
	for i := 0; i < len(proof)*8; i++ {
		// Flip bit in position i.
		if _, err := pk.ProofToHash(data, flipBit(proof, i)); err == nil {
			t.Errorf("Verify unexpectedly succeeded after flipping bit %v of vrf", i)
		}
	}
}

func flipBit(a []byte, pos int) []byte {
	index := int(math.Floor(float64(pos) / 8))
	b := a[index]
	b ^= (1 << uint(math.Mod(float64(pos), 8.0)))

	var buf bytes.Buffer
	buf.Write(a[:index])
	buf.Write([]byte{b})
	buf.Write(a[index+1:])
	return buf.Bytes()
}
