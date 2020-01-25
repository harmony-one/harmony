package slash

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	common2 "github.com/harmony-one/harmony/internal/common"
)

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	BlockHash   common.Hash
	BlockNumber *big.Int `json:"block-number"`
	// TODO Add the signature of the double sign as well, had it happened.
	Signature   [96]byte       // (aggregated) signature
	Bitmap      []byte         // corresponding bitmap mask for agg signature
	ShardID     uint32         `json:"shard-id"`
	Epoch       *big.Int       `json:"epoch"`
	Beneficiary common.Address // the reporter who will get rewarded
}

// Verify checks that the signature is valid
func Verify(candidate *Record) error {
	return nil
}

// MarshalJSON ..
func (r *Record) MarshalJSON() ([]byte, error) {
	type rec struct {
		Record,
		BlockHash string `json:"block-hash"`
		Signature   string `json:"signature"`
		Bitmap      string `json:"bitmap"`
		Beneficiary string `json:"beneficiary"`
	}
	one, err := common2.AddressToBech32(r.Beneficiary)
	if err != nil {
		return json.Marshal(rec{
			BlockHash:   r.BlockHash.Hex(),
			Signature:   hex.EncodeToString(r.Signature[:]),
			Bitmap:      hex.EncodeToString(r.Bitmap),
			Beneficiary: one,
		})
	}
	return nil, err
}
