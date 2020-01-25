package slash

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/votepower"
	"github.com/harmony-one/harmony/core/state"
	common2 "github.com/harmony-one/harmony/internal/common"
)

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	BlockHash       common.Hash
	BlockNumber     *big.Int `json:"block-number"`
	Signature       votepower.BallotResults
	DoubleSignature votepower.BallotResults
	ShardID         uint32         `json:"shard-id"`
	Epoch           *big.Int       `json:"epoch"`
	Beneficiary     common.Address // the reporter who will get rewarded
}

// TODO(Edgar) Implement Verify and Apply

// Verify checks that the signature is valid
func Verify(candidate *Record) error {
	return nil
}

// Apply ..
func Apply(state *state.DB, slashes []byte) error {
	return nil
}

// MarshalJSON ..
func (r *Record) MarshalJSON() ([]byte, error) {
	type sig struct {
		AggregateSig string `json:"aggregated-signature"`
		Bitmap       string `json:"bitmap"`
	}

	type rec struct {
		Record,
		BlockHash string `json:"block-hash"`
		Signature       sig
		DoubleSignature sig
		Beneficiary     string `json:"beneficiary"`
	}
	one, err := common2.AddressToBech32(r.Beneficiary)

	if err != nil {
		sigP, bitmapP := r.Signature.EncodePair()
		sigD, bitmapD := r.DoubleSignature.EncodePair()

		return json.Marshal(rec{
			Signature:       sig{sigP, bitmapP},
			DoubleSignature: sig{sigD, bitmapD},
			BlockHash:       r.BlockHash.Hex(),
			Beneficiary:     one,
		})
	}
	return nil, err
}
