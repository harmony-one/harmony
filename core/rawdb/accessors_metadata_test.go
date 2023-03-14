package rawdb

import (
	"testing"

	ethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/harmony-one/harmony/crypto/bls"
)

func TestLeaderContinuousBlocksCount(t *testing.T) {
	db := ethRawDB.NewMemoryDatabase()
	err := WriteLeaderContinuousBlocksCount(db, make([]byte, bls.PublicKeySizeInBytes), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	pub, epoch, count, err := ReadLeaderContinuousBlocksCount(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != bls.PublicKeySizeInBytes {
		t.Fatal("invalid leader public key size")
	}
	if epoch != 1 {
		t.Fatal("invalid epoch")
	}
	if count != 2 {
		t.Fatal("invalid count")
	}
}
