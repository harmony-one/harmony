package downloader

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func BenchmarkNewVerifiedSigKey(b *testing.B) {
	var bh common.Hash
	commitSig := make([]byte, 100)
	for i := 0; i != len(commitSig); i++ {
		commitSig[i] = 0xf
	}

	for i := 0; i != b.N; i++ {
		newVerifiedSigKey(bh, commitSig)
	}
}
