package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
)

// TestCXReceiptWithoutRecipientDoesNotRoundTrip records why a cross-shard
// transfer needs a recipient: a receipt with none encodes, so it can be
// committed to by a block header, but it cannot be decoded again, so the
// destination shard can never read it back.
func TestCXReceiptWithoutRecipientDoesNotRoundTrip(t *testing.T) {
	r := &types.CXReceipt{
		TxHash: types.EmptyRootHash, To: nil,
		ShardID: 0, ToShardID: 1, Amount: big.NewInt(5),
	}
	enc, err := rlp.EncodeToBytes(r)
	if err != nil {
		t.Fatalf("expected a receipt without a recipient to encode: %v", err)
	}
	var back types.CXReceipt
	if err := rlp.DecodeBytes(enc, &back); err == nil {
		t.Fatal("expected decoding a receipt without a recipient to fail")
	}
}

// TestCrossShardTransactionRequiresRecipient checks that a cross-shard
// transaction carrying no recipient is not treated as a transfer once strict
// validation is active, and that the earlier behaviour is left unchanged.
func TestCrossShardTransactionRequiresRecipient(t *testing.T) {
	key, _ := crypto.GenerateKey()
	chain, db, header, _ := getTestEnvironment(*key)
	header = header.With().Epoch(big.NewInt(1)).Number(big.NewInt(1)).Header()
	header.SetShardID(0)

	tx := types.NewCrossShardTransaction(
		0, nil, 0, 1, big.NewInt(100), 1000000, big.NewInt(1), []byte{0x60, 0x00},
	)
	signed, err := types.SignTx(tx, types.MakeSigner(chain.Config(), header.Epoch()), key)
	if err != nil {
		t.Fatal(err)
	}

	strictCfg := *params.TestChainConfig
	strictCfg.StrictStateValidationEpoch = big.NewInt(0)
	if got := getTransactionType(&strictCfg, header, signed); got != types.InvalidTx {
		t.Errorf("expected InvalidTx for a cross-shard tx without a recipient, got %v", got)
	}

	legacyCfg := *params.TestChainConfig
	legacyCfg.StrictStateValidationEpoch = params.EpochTBD
	if got := getTransactionType(&legacyCfg, header, signed); got != types.SubtractionOnly {
		t.Errorf("pre-fork behaviour changed: got %v", got)
	}

	// Under strict validation the transaction must not produce a receipt.
	gp := new(GasPool).AddGas(10000000)
	usedGas := uint64(0)
	bank := crypto.PubkeyToAddress(key.PublicKey)
	chain.chainConfig = &strictCfg
	_, cx, _, _, err := ApplyTransaction(chain, &bank, gp, db, header, signed, &usedGas, vm.Config{})
	if err == nil {
		t.Fatal("expected ApplyTransaction to reject the transaction")
	}
	if cx != nil {
		t.Fatalf("expected no cross-shard receipt, got To=%v", cx.To)
	}
}
