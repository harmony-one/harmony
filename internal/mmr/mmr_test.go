package mmr

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/harmony-one/harmony/internal/mmr/db"
	"golang.org/x/crypto/sha3"
)

func TestMMR(t *testing.T) {
	memoryBasedDb1 := db.NewMemorybaseddb(0, map[int64][]byte{})
	memoryBasedMmr1 := New(memoryBasedDb1, big.NewInt(0))
	const BlocksPerEpoch = 32768
	for i := 0; i < BlocksPerEpoch; i++ {
		hash := Keccak256([]byte(hexutil.EncodeUint64(uint64(i))))
		memoryBasedMmr1.Append(hash)
	}

	if memoryBasedMmr1.GetLeafLength() != BlocksPerEpoch {
		t.Fatal("invalid leaf length")
	}

	hasher := sha3.NewLegacyKeccak256()
	for i := int64(0); i < BlocksPerEpoch; i++ {
		expectedValue := Keccak256([]byte(hexutil.EncodeUint64(uint64(i))))
		leafValue := memoryBasedMmr1.Get(i)
		if !bytes.Equal(leafValue, expectedValue) {
			t.Fatal("leafValue not expected")
		}
		root, width, _, peaks, _ := memoryBasedMmr1.GetMerkleProof(i)
		if width != BlocksPerEpoch {
			t.Fatal("invalid leaf length")
		}
		hasher.Reset()
		for _, peak := range peaks {
			hasher.Write(peak)
		}
		if !bytes.Equal(hasher.Sum(nil), root) {
			t.Fatal("invlaid root hahs")
		}
	}
}
