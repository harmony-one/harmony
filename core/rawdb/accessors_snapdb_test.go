package rawdb

import (
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	blockfactory "github.com/harmony-one/harmony/block/factory"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
)

func TestSnapdbInfo(t *testing.T) {
	src := &SnapdbInfo{
		NetworkType:         nodeconfig.Mainnet,
		BlockHeader:         blockfactory.NewTestHeader(),
		AccountCount:        12,
		OffchainDataDumped:  true,
		IndexerDataDumped:   true,
		StateDataDumped:     false,
		DumpedSize:          100 * 1024 * 1024 * 1024,
		LastAccountKey:      hexutil.MustDecode("0x1339383fd90ed804e28464763a13fafad66dd0f88434b8e5d8b410eb75a10331"),
		LastAccountStateKey: hexutil.MustDecode("0xa940a0bb9eca4f9d5eee3f4059f458bd4a05bb1d680c5f7c781b06bc43c20df6"),
	}
	db := NewMemoryDatabase()
	if err := WriteSnapdbInfo(db, src); err != nil {
		t.Fatal(err)
	}
	info := ReadSnapdbInfo(db)
	if src.BlockHeader.String() != info.BlockHeader.String() {
		t.Fatal("header of SnapdbInfo doest not equal")
	}
	src.BlockHeader = info.BlockHeader
	if !reflect.DeepEqual(src, info) {
		t.Fatal("snapdbInfo doest not equal")
	}
}
