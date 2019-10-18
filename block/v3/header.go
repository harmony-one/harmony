package v3

import (
	"github.com/harmony-one/harmony/block/v2"
)

// Header v3 has the same structure as v2 header
// It is used to identify the body v3 which including staking txs
type Header struct {
	v2.Header
}

// NewHeader creates a new header object.
func NewHeader() *Header {
	return &Header{*v2.NewHeader()}
}
