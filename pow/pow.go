package pow

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
)

var (
	maxNonce = math.MaxUint32
)

const targetBits = 24

// ProofOfWork represents a proof-of-work
type ProofOfWork struct {
	Challenge  uint32
	target     *big.Int
	FinalNonce uint32
}

// NewProofOfWork builds and returns a ProofOfWork
func NewProofOfWork(c uint32) *ProofOfWork {
	target := big.NewInt(1)
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{Challenge: c, target: target, FinalNonce: 0}

	return pow
}

func (pow *ProofOfWork) prepareData(nonce uint32) []byte {
	challenge := make([]byte, 4)
	binary.LittleEndian.PutUint32(challenge, pow.Challenge)
	nonceB := make([]byte, 4)
	binary.LittleEndian.PutUint32(nonceB, nonce)
	data := bytes.Join(
		[][]byte{
			challenge,
			nonceB,
		},
		[]byte{},
	)
	return data
}

// Run performs a proof-of-work
func (pow *ProofOfWork) Run() int {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0
	for nonce < maxNonce {
		data := pow.prepareData(uint32(nonce))

		hash = sha256.Sum256(data)
		fmt.Printf("\r%x", hash)
		hashInt.SetBytes(hash[:])

		if hashInt.Cmp(pow.target) == -1 {
			pow.FinalNonce = uint32(nonce)
			break
		} else {
			nonce++
		}
	}
	fmt.Print("\n\n")
	return nonce
}

// Validate validates block's PoW
func (pow *ProofOfWork) Validate(nonce uint32) bool {
	var hashInt big.Int
	data := pow.prepareData(nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])
	isValid := hashInt.Cmp(pow.target) == -1
	return isValid
}
