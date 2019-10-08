// Package vdf is a proof-of-concept implementation of a delay function
// and the security properties are not guaranteed.
// A more secure implementation of the VDF by Wesolowski (https://eprint.iacr.org/2018/623.pdf)
// will be done soon.
package vdf

import "golang.org/x/crypto/sha3"

// VDF is the struct holding necessary state for a hash chain delay function.
type VDF struct {
	difficulty int
	input      [32]byte
	output     [32]byte
	outputChan chan [32]byte
	finished   bool
}

// New create a new instance of VDF.
func New(difficulty int, input [32]byte) *VDF {
	return &VDF{
		difficulty: difficulty,
		input:      input,
		outputChan: make(chan [32]byte),
	}
}

// GetOutputChannel returns the vdf output channel.
func (vdf *VDF) GetOutputChannel() chan [32]byte {
	return vdf.outputChan
}

// Execute runs the VDF until it's finished and put the result into output channel.
func (vdf *VDF) Execute() {
	vdf.finished = false
	tempResult := vdf.input
	for i := 0; i < vdf.difficulty; i++ {
		tempResult = sha3.Sum256(tempResult[:])
	}
	vdf.output = tempResult
	// TODO ek – limit concurrency
	go func() {
		vdf.outputChan <- vdf.output
	}()
	vdf.finished = true
}

// IsFinished returns whether the vdf execution is finished or not.
func (vdf *VDF) IsFinished() bool {
	return vdf.finished
}

// GetOutput returns the vdf output, which can be bytes of 0s is the vdf is not finished.
func (vdf *VDF) GetOutput() [32]byte {
	return vdf.output
}
