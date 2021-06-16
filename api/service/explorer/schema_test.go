package explorer

import (
	"bytes"
	"reflect"
	"testing"

	goversion "github.com/hashicorp/go-version"
)

var (
	testNormalIndexes  = makeTestNormalIndexes(100)
	testStakingIndexes = makeTestStakingIndexes(100)
)

func TestVersion(t *testing.T) {
	tests := []struct {
		hasKey bool
		write  string
		is100  bool
	}{
		{
			hasKey: false,
			write:  "",
			is100:  false,
		},
		{
			hasKey: true,
			write:  "0.0.9",
			is100:  false,
		},
		{
			hasKey: true,
			write:  "1.0.0",
			is100:  true,
		},
		{
			hasKey: true,
			write:  "1.0.1",
			is100:  true,
		},
	}
	for i, test := range tests {
		db := newMemDB()
		if test.hasKey {
			ver, err := goversion.NewVersion(test.write)
			if err != nil {
				t.Fatal(err)
			}
			if err := writeVersion(db, ver); err != nil {
				t.Fatal(err)
			}
		}
		got, err := isVersionV100(db)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.is100 {
			t.Errorf("Test %v: unexpected %v / %v", i, got, test.is100)
		}
	}
}

func TestTxnReadWrite(t *testing.T) {
	db := newMemDB()
	hash := makeTestTxHash(1)
	txRecord := &TxRecord{
		Hash:      hash.String(),
		Type:      Sent,
		Timestamp: "",
	}
	if err := writeTxn(db, hash, txRecord); err != nil {
		t.Fatal(err)
	}
	read, err := readTxnByHash(db, hash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read, txRecord) {
		t.Errorf("not deep equal")
	}
}

func TestNormalTxnIndexPrefix(t *testing.T) {
	index := testNormalIndexes[0]
	key := index.key()
	prefix := normalTxnIndexPrefixByAddr(index.addr)

	if !bytes.HasPrefix(key, prefix) {
		t.Errorf("does not have prefix")
	}
}

func TestStakingTxnIndexPrefix(t *testing.T) {
	index := testStakingIndexes[0]
	key := index.key()
	prefix := stakingTxnIndexPrefixByAddr(index.addr)

	if !bytes.HasPrefix(key, prefix) {
		t.Errorf("does not have prefix")
	}
}

//func

func makeTestNormalIndexes(size int) []normalTxnIndex {
	indexes := make([]normalTxnIndex, 0, size)
	for i := 0; i != size; i++ {
		indexes = append(indexes, normalTxnIndex{
			addr:        makeOneAddress(i / 10),
			blockNumber: uint64(i),
			txnIndex:    0,
			txnHash:     makeTestTxHash(i),
		})
	}
	return indexes
}

func makeTestStakingIndexes(size int) []stakingTxnIndex {
	indexes := make([]stakingTxnIndex, 0, size)
	for i := 0; i != size; i++ {
		indexes = append(indexes, stakingTxnIndex{
			addr:        makeOneAddress(i / 2),
			blockNumber: uint64(i),
			txnIndex:    0,
			txnHash:     makeTestTxHash(i + 999999),
		})
	}
	return indexes
}
