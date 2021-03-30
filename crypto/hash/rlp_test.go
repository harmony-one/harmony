package hash

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

// This test file test the bench mark of running `FromRLP` with or without usage of sync.Pool
// in single-threaded / multi-threaded case.
//
// Here is the test results:
//
//     >  go test -bench=. -benchmem
//     goos: darwin
//     goarch: amd64
//     pkg: github.com/harmony-one/harmony/crypto/hash
//     BenchmarkFromRLP_MultiThreaded-12                4841481               263 ns/op             560 B/op          5 allocs/op
//     BenchmarkFromRLPNoPool_MultiThreaded-12          3665227               344 ns/op            1008 B/op          6 allocs/op
//     BenchmarkFromRLP_SingleThreaded-12               1303647               909 ns/op             560 B/op          5 allocs/op
//     BenchmarkFromRLPNoPool_SingleThreaded-12         1296238               920 ns/op            1008 B/op          6 allocs/op
//     PASS
//     ok      github.com/harmony-one/harmony/crypto/hash      7.441s
//
// We can see that FromRLP with sync pool implementation has better memory and CPU performance over
// the implementation without sync pool. This is because the sync pool implementation will reuse
// the hasher and results in better performance both in CPU and memory. The huge CPU difference
// in multi-threading benchmark actually results from sync.Pool's optimization over golang's GC
// mechanism.

func BenchmarkFromRLP_MultiThreaded(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			FromRLP(testObj)
		}
	})
}

func BenchmarkFromRLPNoPool_MultiThreaded(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fromRLPNoPool(testObj)
		}
	})
}

func BenchmarkFromRLP_SingleThreaded(b *testing.B) {
	for i := 0; i != b.N; i++ {
		FromRLP(testObj)
	}
}

func BenchmarkFromRLPNoPool_SingleThreaded(b *testing.B) {
	for i := 0; i != b.N; i++ {
		fromRLPNoPool(testObj)
	}
}

// fromRLPNoPool is a compared function to showcase the FromRLP function with sync pool
func fromRLPNoPool(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

var testObj = testStruct{
	field1: "harmony",
	field2: 1024,
}

type testStruct struct {
	field1 string
	field2 int
}
