package rawdb

import (
	"testing"

	ethRawDB "github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/harmony-one/harmony/crypto/bls"
)

func TestLeaderRotationMeta(t *testing.T) {
	db := ethRawDB.NewMemoryDatabase()
	err := WriteLeaderRotationMeta(db, make([]byte, bls.PublicKeySizeInBytes), 1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	pub, epoch, count, shifts, err := ReadLeaderRotationMeta(db)
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
	if shifts != 3 {
		t.Fatal("invalid shifts")
	}
}
