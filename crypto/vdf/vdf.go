// Note this is a proof-of-concept implementation of a delay function
// and the security properties are not guaranteed.
// A more secure implementation of the VDF by Wesolowski (https://eprint.iacr.org/2018/623.pdf)
// will be done soon.
package vdf

import "golang.org/x/crypto/sha3"

type VDF struct {
	difficulty int
	input      [32]byte
	output     [32]byte
	finished   chan [32]byte
}

func (vdf *VDF) execute() {
	tempResult := vdf.input
	for i := 0; i < vdf.difficulty; i++ {
		tempResult = sha3.Sum256(tempResult[:])
	}
	vdf.output = tempResult
	vdf.finished <- vdf.output
}
