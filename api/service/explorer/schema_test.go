package explorer

import (
	"bytes"
	"reflect"
	"sort"
	"testing"
	"time"

	goversion "github.com/hashicorp/go-version"
)

var (
	testNormalIndexes  = makeTestNormalIndexes(10)
	testStakingIndexes = makeTestStakingIndexes(10)
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
		Hash:      hash,
		Timestamp: time.Unix(100, 0),
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

func TestGetNormalTxnHashesByAccount(t *testing.T) {
	db := newMemDB()

	txnIndexes := makeTestNormalIndexes(10)
	for _, index := range txnIndexes {
		if err := writeNormalTxnIndex(db, index, txSent); err != nil {
			t.Fatal(err)
		}
	}
	// Checking results.
	for i := 0; i != 10; i++ {
		addr := makeOneAddress(i)
		hashes, tts, err := getNormalTxnHashesByAccount(db, addr)
		if err != nil {
			t.Fatal(err)
		}
		if exp := i; exp != len(hashes) {
			t.Errorf("get normal txn hashes not expected: %v / %v", len(hashes), exp)
		}
		for _, tt := range tts {
			if tt != txSent {
				t.Errorf("unexpected type")
			}
		}
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

func TestTxHashFromNormalTxnIndexKey(t *testing.T) {
	index := testNormalIndexes[0]
	key := index.key()
	txHash, err := txnHashFromNormalTxnIndexKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if txHash != index.txnHash {
		t.Fatal(err)
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

func TestTxHashFromStakingTxnIndexKey(t *testing.T) {
	index := testStakingIndexes[0]
	key := index.key()
	txHash, err := txnHashFromStakingTxnIndexKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if txHash != index.txnHash {
		t.Fatal(err)
	}
}

func TestGetStakingTxnHashesByAccount(t *testing.T) {
	db := newMemDB()

	txnIndexes := makeTestStakingIndexes(10)
	for _, index := range txnIndexes {
		if err := writeStakingTxnIndex(db, index, txSent); err != nil {
			t.Fatal(err)
		}
	}
	// Checking results.
	for i := 0; i != 10; i++ {
		addr := makeOneAddress(i)
		hashes, tts, err := getStakingTxnHashesByAccount(db, addr)
		if err != nil {
			t.Fatal(err)
		}
		if exp := i; exp != len(hashes) {
			t.Errorf("get normal txn hashes not expected: %v / %v", len(hashes), exp)
		}
		if len(hashes) != len(tts) {
			t.Errorf("hash size not equal to TxType size")
		}
		for _, tt := range tts {
			if tt != txSent {
				t.Errorf("unexpected type")
			}
		}
	}
}

func TestGetAllAddresses(t *testing.T) {
	db := newMemDB()
	addrs := makeAddresses(10)
	for _, addr := range addrs {
		if err := writeAddressEntry(db, addr); err != nil {
			t.Fatal(err)
		}
	}

	gotAddrs, err := getAllAddresses(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotAddrs) != len(addrs) {
		t.Fatalf("unexpected size %v / %v", len(gotAddrs), len(addrs))
	}
	sort.SliceStable(addrs, func(i, j int) bool {
		return bytes.Compare([]byte(addrs[i]), []byte(addrs[j])) < 0
	})
	// got addresses are expected to be sorted as addrs
	for i := range gotAddrs {
		got := gotAddrs[i]
		exp := addrs[i]
		if got != exp {
			t.Errorf("unexpected address: %v / %v", got, exp)
		}
	}
}

func makeAddresses(size int) []oneAddress {
	var addrs []oneAddress
	for i := 0; i != size; i++ {
		addrs = append(addrs, makeOneAddress(i))
	}
	return addrs
}

func makeTestNormalIndexes(addrNum int) []normalTxnIndex {
	num := addrNum * (addrNum + 1) / 2
	indexes := make([]normalTxnIndex, 0, num)
	for i := 0; i != addrNum; i++ {
		for j := 0; j != i; j++ {
			indexes = append(indexes, normalTxnIndex{
				addr:        makeOneAddress(i),
				blockNumber: uint64(j),
				txnIndex:    0,
				txnHash:     makeTestTxHash(j),
			})
		}
	}
	return indexes
}

func makeTestStakingIndexes(addrNum int) []stakingTxnIndex {
	num := addrNum * (addrNum + 1) / 2
	indexes := make([]stakingTxnIndex, 0, num)
	for i := 0; i != addrNum; i++ {
		for j := 0; j != i; j++ {
			indexes = append(indexes, stakingTxnIndex{
				addr:        makeOneAddress(i),
				blockNumber: uint64(j),
				txnIndex:    0,
				txnHash:     makeTestTxHash(j + 999999),
			})
		}
	}
	return indexes
}
