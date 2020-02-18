package slash

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/shard"
)

const (
	signerABLSPublicHex = "be23bc3c93fe14a25f3533" +
		"feee1cff1c60706845a4907" +
		"c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a"
	signerAHeaderHashHex = "0x68bf572c03e36b4b7a4f268797d18" +
		"7027b288bc69725084bab5bfe6214cb8ddf"
	signerABLSSignature = "ab14e519485b70d8af76ae83205290792d679a8a11eb9a12d21787cc0b73" +
		"96e340d88b2a0c39888d0c9cec1" +
		"2c4a09b06b2eec3e851f08f3070f3" +
		"804b35fe4a2033725f073623e3870756141ebc" +
		"2a6495478930c428f6e6b25f292dab8552d30c"

	signerBBLSPublicHex = "be23bc3c93fe14a25f3533feee1cff1c60706845a490" +
		"7c5df58bc19f5d1760bfff06fe7c9d1f596b18fdf529e0508e0a"
	signerBHeaderHashHex = "0x1dbf572c03e36b4b7a4f268797d187" +
		"027b288bc69725084bab5bfe6214cb8ddf"
	signerBBLSSignature = "0894d55a541a90ada11535866e5a848d9d6a2b5c30" +
		"932a95f48133f886140cefbe4d690eddd0540d246df1fec" +
		"8b4f719ad9de0bc822f0a1bf70e78b321a5e4462ba3e3efd" +
		"cd24c21b9cb24ed6b26f02785a2cdbd168696c5f4a49b6c00f00994"
)

var (
	signerA, signerB       = &bls.PublicKey{}, &bls.PublicKey{}
	hashA, hashB           = common.Hash{}, common.Hash{}
	signatureA, signatureB = &bls.Sign{}, &bls.Sign{}

	unit = func() interface{} {
		// Ballot A setup
		signerA.DeserializeHexStr(signerABLSPublicHex)
		headerHashA, _ := hex.DecodeString(signerAHeaderHashHex)
		hashA = common.BytesToHash(headerHashA)
		signatureA.DeserializeHexStr(signerABLSSignature)

		// Ballot B setup
		signerB.DeserializeHexStr(signerBBLSPublicHex)
		headerHashB, _ := hex.DecodeString(signerBHeaderHashHex)
		hashB = common.BytesToHash(headerHashB)
		signatureB.DeserializeHexStr(signerBBLSSignature)

		return nil
	}()

	blsWrapA, blsWrapB = *shard.FromLibBLSPublicKeyUnsafe(signerA),
		*shard.FromLibBLSPublicKeyUnsafe(signerB)

	exampleSlash = Records{
		Record{
			ConflictingBallots: ConflictingBallots{
				AlreadyCastBallot: votepower.Ballot{
					SignerPubKey:    blsWrapA,
					BlockHeaderHash: hashA,
					Signature:       signatureA,
				},
				DoubleSignedBallot: votepower.Ballot{
					SignerPubKey:    blsWrapB,
					BlockHeaderHash: hashB,
					Signature:       signatureB,
				},
			},
			Evidence: Evidence{
				Moment: Moment{
					Epoch:        big.NewInt(6),
					Height:       big.NewInt(37),
					TimeUnixNano: big.NewInt(1582049233802498300),
					ViewID:       38,
					ShardID:      0,
				},
				ProposalHeader: &block.Header{},
			},
		},
	}
)

func TestDidAnyoneDoubleSign(t *testing.T) {
	t.Log("Unimplemented")
}

func TestApply(t *testing.T) {
	t.Log("Unimplemented")
}
