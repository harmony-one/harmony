package chain

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
)

func TestVerifyCrossLinkRejectsAbandonedChainAnchors(t *testing.T) {
	tests := []types.CrossLink{
		{
			ShardIDF:     0,
			BlockNumberF: big.NewInt(92730036),
			HashF:        common.HexToHash("0x890473cdb9aa8dc5c0bbd54cf20b6d8d84bda60d3dcb2273443d34432d8539e8"),
		},
		{
			ShardIDF:     1,
			BlockNumberF: big.NewInt(94978279),
			HashF:        common.HexToHash("0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878"),
		},
	}

	for _, crossLink := range tests {
		if err := NewEngine().VerifyCrossLink(nil, crossLink); !errors.Is(err, engine.ErrRejectedBlock) {
			t.Fatalf("VerifyCrossLink() error = %v, want %v", err, engine.ErrRejectedBlock)
		}
	}
}
