package slash

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/shard"
)

type pair struct {
	Header    *block.Header `json:"header"`
	Signature *bls.Sign     `json:"signature"`
}

// Record is an proof of a slashing made by a witness of a double-signing event
type Record struct {
	Offender     shard.BlsPublicKey `json:"offender"`
	Signed       pair               `json:"signed"`
	DoubleSigned pair               `json:"double-signed"`
	Beneficiary  common.Address     `json:"beneficiary"` // the reporter who will get rewarded
}

// NewRecord ..
func NewRecord(
	offender shard.BlsPublicKey,
	signedHeader, doubleSignedHeader *block.Header,
	signedSignature, doubleSignedSignature *bls.Sign,
	beneficiary common.Address,
) Record {
	return Record{
		offender,
		pair{signedHeader, signedSignature},
		pair{doubleSignedHeader, doubleSignedSignature},
		beneficiary,
	}
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
