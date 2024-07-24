package chain

import (
	bls_core "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
)

// ReadSignatureBitmapByPublicKeys read the payload of signature and bitmap based on public keys
func ReadSignatureBitmapByPublicKeys(recvPayload []byte, publicKeys []bls.PublicKeyWrapper) (*bls_core.Sign, *bls.Mask, error) {
	sig, bitmap, err := ParseCommitSigAndBitmap(recvPayload)
	if err != nil {
		return nil, nil, err
	}
	return DecodeSigBitmap(sig, bitmap, publicKeys)
}

// ParseCommitSigAndBitmap parse the commitSigAndBitmap to signature + bitmap
func ParseCommitSigAndBitmap(payload []byte) (bls.SerializedSignature, []byte, error) {
	if len(payload) < bls.BLSSignatureSizeInBytes {
		return bls.SerializedSignature{}, nil, errors.New("payload not have enough length")
	}
	var (
		sig    bls.SerializedSignature
		bitmap = make([]byte, len(payload)-bls.BLSSignatureSizeInBytes)
	)
	copy(sig[:], payload[:bls.BLSSignatureSizeInBytes])
	copy(bitmap, payload[bls.BLSSignatureSizeInBytes:])

	return sig, bitmap, nil
}

// DecodeSigBitmap decode and parse the signature, bitmap with the given public keys
func DecodeSigBitmap(sigBytes bls.SerializedSignature, bitmap []byte, pubKeys []bls.PublicKeyWrapper) (*bls_core.Sign, *bls.Mask, error) {
	aggSig := bls_core.Sign{}
	err := aggSig.Deserialize(sigBytes[:])
	if err != nil {
		return nil, nil, errors.New("unable to deserialize multi-signature from payload")
	}
	mask := bls.NewMask(pubKeys)
	if err := mask.SetMask(bitmap); err != nil {
		utils.Logger().Warn().Err(errors.WithStack(err)).Msgf("mask.SetMask failed %+v", err)
		return nil, nil, errors.Errorf("mask.SetMask failed %+v", err)
	}
	return &aggSig, mask, nil
}
