package main

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core"
)

func TestAllowedTxsParse(t *testing.T) {
	testData := []byte(`
		0x7A6Ed0a905053A21C15cB5b4F39b561B6A3FE50f->0x855Ac656956AF761439f4a451c872E812E3900a4:0x
		0x7A6Ed0a905053A21C15cB5b4F39b561B6A3FE50f->one1np293efrmv74xyjcz0kk3sn53x0fm745f2hsuc:0xa9059cbb
		one1s4dvv454dtmkzsulffz3epewsyhrjq9y0g3fqz->0x985458E523dB3d53125813eD68c274899e9DfAb4:0xa9059cbb
		one1s4dvv454dtmkzsulffz3epewsyhrjq9y0g3fqz->one10fhdp2g9q5azrs2ukk608x6krd4rleg0ueskug:0x
	`)
	expected := map[ethCommon.Address][]core.AllowedTxData{
		common.HexToAddress("0x7A6Ed0a905053A21C15cB5b4F39b561B6A3FE50f"): {
			core.AllowedTxData{
				To:   common.HexToAddress("0x855Ac656956AF761439f4a451c872E812E3900a4"),
				Data: common.FromHex("0x"),
			},
			core.AllowedTxData{
				To:   common.HexToAddress("0x985458E523dB3d53125813eD68c274899e9DfAb4"),
				Data: common.FromHex("0xa9059cbb"),
			},
		},
		common.HexToAddress("0x855Ac656956AF761439f4a451c872E812E3900a4"): {
			core.AllowedTxData{
				To:   common.HexToAddress("0x985458E523dB3d53125813eD68c274899e9DfAb4"),
				Data: common.FromHex("0xa9059cbb"),
			},
			core.AllowedTxData{
				To:   common.HexToAddress("0x7A6Ed0a905053A21C15cB5b4F39b561B6A3FE50f"),
				Data: common.FromHex("0x"),
			},
		},
	}
	got, err := parseAllowedTxs(testData)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(expected) {
		t.Errorf("lenght of allowed transactions not equal, got: %d expected: %d", len(got), len(expected))
	}
	for from, txsData := range got {
		for i, txData := range txsData {
			expectedTxData := expected[from][i]
			if expectedTxData.To != txData.To || !bytes.Equal(expectedTxData.Data, txData.Data) {
				t.Errorf("txData not equal: got: %v expected: %v", txData, expectedTxData)
			}
		}
	}
}
