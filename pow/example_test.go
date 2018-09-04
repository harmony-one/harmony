package pow_test

import (
	"fmt" // imported as pow

	"github.com/simple-rules/harmony-benchmark/pow"
)

func Example() {
	// Create a proof of work request with difficulty 5
	req := pow.NewRequest(5, []byte("some random nonce"))
	fmt.Printf("req:   %s\n", req)

	// Fulfil the proof of work
	proof, _ := pow.Fulfil(req, []byte("some bound data"))
	fmt.Printf("proof: %s\n", proof)

	// Check if the proof is correct
	ok, _ := pow.Check(req, proof, []byte("some bound data"))
	fmt.Printf("check: %v", ok)

	// Output: req:   sha2bday-5-c29tZSByYW5kb20gbm9uY2U
	// proof: AAAAAAAAAAMAAAAAAAAADgAAAAAAAAAb
	// check: true
}
