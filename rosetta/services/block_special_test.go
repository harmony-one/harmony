package services

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core"
	internalCommon "github.com/harmony-one/harmony/internal/common"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/rosetta/common"
)

var (
	oneBig     = big.NewInt(1e18)
	tenOnes    = new(big.Int).Mul(big.NewInt(10), oneBig)
	twelveOnes = new(big.Int).Mul(big.NewInt(12), oneBig)
	gasPrice   = big.NewInt(10000)
)

func TestGetPseudoTransactionForGenesis(t *testing.T) {
	genesisSpec := core.NewGenesisSpec(nodeconfig.Testnet, 0)
	txs := getPseudoTransactionForGenesis(genesisSpec)
	for acc := range genesisSpec.Alloc {
		found := false
		for _, tx := range txs {
			if acc == *tx.To() {
				found = true
				break
			}
		}
		if !found {
			t.Error("unable to find genesis account in generated pseudo transactions")
		}
	}
}

func TestSpecialCaseTransactionIdentifier(t *testing.T) {
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	testB32Address := "one10g7kfque6ew2jjfxxa6agkdwk4wlyjuncp6gwz"
	testAddress := internalCommon.MustBech32ToAddress(testB32Address)
	refTxID := &types.TransactionIdentifier{
		Hash: fmt.Sprintf("%v_%v_%v", testBlkHash.String(), testB32Address, SpecialGenesisTxID.String()),
	}
	specialTxID := getSpecialCaseTransactionIdentifier(
		testBlkHash, testAddress, SpecialGenesisTxID,
	)
	if !reflect.DeepEqual(refTxID, specialTxID) {
		t.Fatal("invalid for mate for special case TxID")
	}
	unpackedBlkHash, unpackedAddress, rosettaError := unpackSpecialCaseTransactionIdentifier(
		specialTxID, SpecialGenesisTxID,
	)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if unpackedAddress != testAddress {
		t.Errorf("expected unpacked address to be %v not %v", testAddress.String(), unpackedAddress.String())
	}
	if unpackedBlkHash.String() != testBlkHash.String() {
		t.Errorf("expected blk hash to be %v not %v", unpackedBlkHash.String(), testBlkHash.String())
	}

	_, _, rosettaError = unpackSpecialCaseTransactionIdentifier(
		&types.TransactionIdentifier{Hash: ""}, SpecialGenesisTxID,
	)
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if rosettaError.Code != common.CatchAllError.Code {
		t.Error("expected error code to be catch call error")
	}
}
