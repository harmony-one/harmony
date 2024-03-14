package bn256

import (
	"bytes"

	"github.com/consensys/gnark-crypto/ecc"
	gnark "github.com/consensys/gnark/backend/groth16"
)

func FromBytesToVerifyingKey(verifyingKey []byte) (gnark.VerifyingKey, error) {
	vk := gnark.NewVerifyingKey(ecc.BN254)
	_, err := vk.ReadFrom(bytes.NewReader(verifyingKey))
	if err != nil {
		return nil, err
	}
	return vk, nil
}

func FromBytesToProof(proofBytes []byte) (gnark.Proof, error) {
	proof := gnark.NewProof(ecc.BN254)
	_, err := proof.ReadFrom(bytes.NewReader(proofBytes))
	if err != nil {
		return nil, err
	}
	return proof, nil
}
