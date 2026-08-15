package engine

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// ErrRejectedBlock is returned for a block that consensus must never accept.
var ErrRejectedBlock = errors.New("block rejected by hash")

var rejectedBlockHashes = map[common.Hash]struct{}{
	// Shard 0 retains block 92,730,034. Reject its original child so a dirty
	// rolled-back database cannot reattach the abandoned branch.
	common.HexToHash("0x5de06979a333f20afb8b245a8cf44472dc5bfc7383a57ddee48e1809bcee7c5d"): {},
	// Keep the first confirmed malicious shard-0 block rejected as defense in
	// depth, including for embedded block references.
	common.HexToHash("0x890473cdb9aa8dc5c0bbd54cf20b6d8d84bda60d3dcb2273443d34432d8539e8"): {},
	common.HexToHash("0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878"): {},
}

// ValidateBlockHash rejects an abandoned chain anchor. Descendants cannot
// attach once their anchor is rejected.
func ValidateBlockHash(hash common.Hash) error {
	if _, rejected := rejectedBlockHashes[hash]; rejected {
		return fmt.Errorf("%w: %s", ErrRejectedBlock, hash.Hex())
	}
	return nil
}
