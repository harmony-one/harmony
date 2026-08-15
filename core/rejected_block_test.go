package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	"github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/core/types"
)

const rejectedShard1BlockHash = "0xc936581d391b74a620bf6636519834b14a9a2d4e9a5154867c8407f219d8a878"

func TestValidateBlockHashesRejectsEmbeddedCrossLink(t *testing.T) {
	block := blockWithRejectedCrossLink(t)

	if err := validateBlockHashes(block); !errors.Is(err, engine.ErrRejectedBlock) {
		t.Fatalf("validateBlockHashes() error = %v, want %v", err, engine.ErrRejectedBlock)
	}
}

func TestWriteBlockWithoutStateRejectsEmbeddedCrossLinkBeforeDatabaseWrite(t *testing.T) {
	block := blockWithRejectedCrossLink(t)
	var chain *BlockChainImpl

	if err := chain.WriteBlockWithoutState(block); !errors.Is(err, engine.ErrRejectedBlock) {
		t.Fatalf("WriteBlockWithoutState() error = %v, want %v", err, engine.ErrRejectedBlock)
	}
}

func blockWithRejectedCrossLink(t *testing.T) *types.Block {
	t.Helper()
	crossLinks := types.CrossLinks{{
		ShardIDF:     1,
		BlockNumberF: big.NewInt(94978279),
		ViewIDF:      new(big.Int),
		HashF:        common.HexToHash(rejectedShard1BlockHash),
		EpochF:       new(big.Int),
	}}
	encoded, err := rlp.EncodeToBytes(crossLinks)
	if err != nil {
		t.Fatal(err)
	}
	header := blockfactory.NewTestHeader().With().CrossLinks(encoded).Header()
	return types.NewBlock(header, nil, nil, nil, nil, nil)
}

func TestValidateBlockHashesRejectsIncomingReceiptSource(t *testing.T) {
	rejectedHeader := blockfactory.NewTestHeader().With().Extra([]byte("rejected source header")).Header()
	block := types.NewBlock(
		blockfactory.NewTestHeader(), nil, nil, nil,
		[]*types.CXReceiptsProof{{Header: rejectedHeader}}, nil,
	)

	validateHash := func(hash common.Hash) error {
		if hash == rejectedHeader.Hash() {
			return engine.ErrRejectedBlock
		}
		return nil
	}
	if err := validateBlockHashesWith(block, validateHash); !errors.Is(err, engine.ErrRejectedBlock) {
		t.Fatalf("validateBlockHashes() error = %v, want %v", err, engine.ErrRejectedBlock)
	}
}

func TestValidateBlockHashesLeavesMalformedCrossLinksToSemanticValidation(t *testing.T) {
	header := blockfactory.NewTestHeader().With().CrossLinks([]byte("not rlp")).Header()
	block := types.NewBlock(header, nil, nil, nil, nil, nil)

	if err := validateBlockHashes(block); err != nil {
		t.Fatalf("validateBlockHashes() error = %v, want nil", err)
	}
}
