package chainread

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/harmony-one/harmony/core/rawdb"
)

// TestKeySchemaAgainstRawdb pins the re-derived exact keys byte-for-byte
// against what the production rawdb writers put on disk.
func TestKeySchemaAgainstRawdb(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	hash := common.HexToHash("0x30c35d2f2291e4b27debe7862956cf7a0cc7abefc044273d6823567335086d8d")
	const height = uint64(92730034)

	if err := rawdb.WriteCanonicalHash(db, hash, height); err != nil {
		t.Fatal(err)
	}
	got, err := db.Get(HeaderHashKey(height))
	if err != nil || !bytes.Equal(got, hash.Bytes()) {
		t.Fatalf("canonical key mismatch: %x %v", got, err)
	}

	if err := rawdb.WriteHeaderNumber(db, hash, height); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get(HeaderNumberKey(hash)); err != nil {
		t.Fatalf("reverse-number key mismatch: %v", err)
	}

	sig := []byte("aggregate-signature-and-bitmap")
	if err := rawdb.WriteBlockCommitSig(db, height, sig); err != nil {
		t.Fatal(err)
	}
	got, err = db.Get(BlockCommitSigKey(height))
	if err != nil || !bytes.Equal(got, sig) {
		t.Fatalf("block-sig key mismatch: %x %v", got, err)
	}

	epoch := big.NewInt(3002)
	ss := []byte("shard-state-bytes")
	if err := rawdb.WriteShardStateBytes(db, epoch, ss); err != nil {
		t.Fatal(err)
	}
	got, err = db.Get(ShardStateKey(epoch))
	if err != nil || !bytes.Equal(got, ss) {
		t.Fatalf("ss key mismatch: %x %v", got, err)
	}

	if err := rawdb.WriteHeadHeaderHash(db, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get(HeadHeaderKey); err != nil {
		t.Fatalf("LastHeader key mismatch: %v", err)
	}
	if err := rawdb.WriteHeadBlockHash(db, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Get(HeadBlockKey); err != nil {
		t.Fatalf("LastBlock key mismatch: %v", err)
	}

	// Literal shapes.
	if !bytes.Equal(BlockCommitSigKey(height)[:10], []byte("block-sig-")) {
		t.Fatal("block-sig prefix")
	}
	if !bytes.Equal(ShardStateKey(epoch), append([]byte("ss"), 0x0b, 0xba)) {
		t.Fatalf("ss<3002> literal: %x", ShardStateKey(epoch))
	}
	wantCanonical := append(append([]byte("h"), 0, 0, 0, 0, 0x05, 0x86, 0xf2, 0xb2), 'n') // 92730034 = 0x0586F2B2
	if !bytes.Equal(HeaderHashKey(height), wantCanonical) {
		t.Fatalf("canonical literal: %x want %x", HeaderHashKey(height), wantCanonical)
	}
}
