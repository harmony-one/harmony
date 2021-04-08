package chain

import (
	"errors"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// ReadSignatureBitmapByPublicKeys read the payload of signature and bitmap based on public keys
func ReadSignatureBitmapByPublicKeys(recvPayload []byte, publicKeys []bls.PublicKey) (bls.Signature, *bls.Mask, error) {
	sig, bitmap, err := ParseCommitSigAndBitmap(recvPayload)
	if err != nil {
		return nil, nil, err
	}
	return DecodeSigBitmap(sig, bitmap, publicKeys)
}

// ParseCommitSigAndBitmap parse the commitSigAndBitmap to signature + bitmap
func ParseCommitSigAndBitmap(payload []byte) (bls.SerializedSignature, []byte, error) {
	if len(payload) < bls.SignatureSize {
		return bls.SerializedSignature{}, nil, errors.New("payload not have enough length")
	}
	var (
		sig    bls.SerializedSignature
		bitmap = make([]byte, len(payload)-bls.SignatureSize)
	)
	copy(sig[:], payload[:bls.SignatureSize])
	copy(bitmap, payload[bls.SignatureSize:])

	return sig, bitmap, nil
}

// DecodeSigBitmap decode and parse the signature, bitmap with the given public keys
func DecodeSigBitmap(sigBytes bls.SerializedSignature, bitmap []byte, pubKeys []bls.PublicKey) (bls.Signature, *bls.Mask, error) {
	aggSig, err := bls.SignatureFromBytes(sigBytes[:])
	if err != nil {
		return nil, nil, errors.New("unable to deserialize multi-signature from payload")
	}
	mask, err := bls.NewMask(pubKeys, nil)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("onNewView unable to setup mask for prepared message")
		return nil, nil, errors.New("unable to setup mask from payload")
	}
	if err := mask.SetMask(bitmap); err != nil {
		utils.Logger().Warn().Err(err).Msg("mask.SetMask failed")
	}
	return aggSig, mask, nil
}
