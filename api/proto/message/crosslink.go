package message

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/types"
)

func fromBytes(b []byte) *big.Int {
	out := big.Int{}
	out.SetBytes(b)
	return &out
}

func CreateCrossLinkV2(cx *CrosslinkMessage) *types.CrossLinkV2 {
	hash := common.Hash{}
	copy(hash[:], cx.Hash)

	signature := [96]byte{}
	copy(signature[:], cx.Signature)

	bitmap := make([]byte, len(cx.Bitmap))
	copy(bitmap[:], cx.Bitmap)

	proposer := common.Address{}
	proposer.SetBytes(cx.Proposer)

	return &types.CrossLinkV2{
		CrossLinkV1: types.CrossLinkV1{
			HashF:        hash,
			BlockNumberF: fromBytes(cx.BlockNumber),
			ViewIDF:      fromBytes(cx.ViewId),
			SignatureF:   signature,
			BitmapF:      bitmap,
			ShardIDF:     cx.ShardId,
			EpochF:       fromBytes(cx.Epoch),
		},
		Proposer: proposer,
	}
}
