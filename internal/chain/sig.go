package chain

import (
	"errors"

	bls_core "github.com/harmony-one/bls/ffi/go/bls"

	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/harmony-one/harmony/internal/utils"
)

// ReadSignatureBitmapByPublicKeys read the payload of signature and bitmap based on public keys
func ReadSignatureBitmapByPublicKeys(recvPayload []byte, publicKeys []*bls_core.PublicKey) (*bls_core.Sign, *bls.Mask, error) {
	if len(recvPayload) < 96 {
		return nil, nil, errors.New("payload not have enough length")
	}
	payload := append(recvPayload[:0:0], recvPayload...)
	//#### Read payload data
	// 96 byte of multi-sig
	offset := 0
	multiSig := payload[offset : offset+96]
	offset += 96
	// bitmap
	bitmap := payload[offset:]
	//#### END Read payload data

	aggSig := bls_core.Sign{}
	err := aggSig.Deserialize(multiSig)
	if err != nil {
		return nil, nil, errors.New("unable to deserialize multi-signature from payload")
	}
	mask, err := bls.NewMask(publicKeys, nil)
	if err != nil {
		utils.Logger().Warn().Err(err).Msg("onNewView unable to setup mask for prepared message")
		return nil, nil, errors.New("unable to setup mask from payload")
	}
	if err := mask.SetMask(bitmap); err != nil {
		utils.Logger().Warn().Err(err).Msg("mask.SetMask failed")
	}
	return &aggSig, mask, nil
}
