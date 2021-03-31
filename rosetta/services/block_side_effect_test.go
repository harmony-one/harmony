package services

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/coinbase/rosetta-sdk-go/types"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/rosetta/common"
)

var (
	oneBig     = big.NewInt(1e18)
	tenOnes    = new(big.Int).Mul(big.NewInt(10), oneBig)
	twelveOnes = new(big.Int).Mul(big.NewInt(12), oneBig)
	gasPrice   = big.NewInt(10000)
)

func TestSideEffectTransactionIdentifier(t *testing.T) {
	testBlkHash := ethcommon.HexToHash("0x1a06b0378d63bf589282c032f0c85b32827e3a2317c2f992f45d8f07d0caa238")
	refTxID := &types.TransactionIdentifier{
		Hash: fmt.Sprintf("%v_%v", testBlkHash.String(), SideEffectTransactionSuffix),
	}
	specialTxID := getSideEffectTransactionIdentifier(testBlkHash)
	if !reflect.DeepEqual(refTxID, specialTxID) {
		t.Fatal("invalid for mate for special case TxID")
	}
	unpackedBlkHash, rosettaError := unpackSideEffectTransactionIdentifier(specialTxID)
	if rosettaError != nil {
		t.Fatal(rosettaError)
	}
	if unpackedBlkHash.String() != testBlkHash.String() {
		t.Errorf("expected blk hash to be %v not %v", unpackedBlkHash.String(), testBlkHash.String())
	}

	_, rosettaError = unpackSideEffectTransactionIdentifier(&types.TransactionIdentifier{Hash: ""})
	if rosettaError == nil {
		t.Fatal("expected rosetta error")
	}
	if rosettaError.Code != common.CatchAllError.Code {
		t.Error("expected error code to be catch call error")
	}
}
