// Create and fulfill proof of work requests.
package pow

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type Algorithm string

const (
	Sha2BDay Algorithm = "sha2bday"
)

// Represents a proof-of-work request.
type Request struct {

	// The requested algorithm
	Alg Algorithm

	// The requested difficulty
	Difficulty uint32

	// Nonce to diversify the request
	Nonce []byte
}

// Represents a completed proof-of-work
type Proof struct {
	buf []byte
}

// Convenience function to create a new sha3bday proof-of-work request
// as a string
func NewRequest(difficulty uint32, nonce []byte) string {
	req := Request{
		Difficulty: difficulty,
		Nonce:      nonce,
		Alg:        Sha2BDay,
	}
	s, _ := req.MarshalText()
	return string(s)
}

func (proof Proof) MarshalText() ([]byte, error) {
	return []byte(base64.RawStdEncoding.EncodeToString(proof.buf)), nil
}

func (proof *Proof) UnmarshalText(buf []byte) error {
	var err error
	proof.buf, err = base64.RawStdEncoding.DecodeString(string(buf))
	return err
}

func (req Request) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s-%d-%s",
		req.Alg,
		req.Difficulty,
		string(base64.RawStdEncoding.EncodeToString(req.Nonce)))), nil
}

func (req *Request) UnmarshalText(buf []byte) error {
	bits := strings.SplitN(string(buf), "-", 3)
	if len(bits) != 3 {
		return fmt.Errorf("There should be two dashes in a PoW request")
	}
	alg := Algorithm(bits[0])
	if alg != Sha2BDay {
		return fmt.Errorf("%s: unsupported algorithm", bits[0])
	}
	req.Alg = alg
	diff, err := strconv.Atoi(bits[1])
	if err != nil {
		return err
	}
	req.Difficulty = uint32(diff)
	req.Nonce, err = base64.RawStdEncoding.DecodeString(bits[2])
	if err != nil {
		return err
	}
	return nil
}

// Convenience function to check whether a proof of work is fulfilled
func Check(request, proof string, data []byte) (bool, error) {
	var req Request
	var prf Proof
	err := req.UnmarshalText([]byte(request))
	if err != nil {
		return false, err
	}
	err = prf.UnmarshalText([]byte(proof))
	if err != nil {
		return false, err
	}
	return prf.Check(req, data), nil
}

// Fulfil the proof-of-work request.
func (req *Request) Fulfil(data []byte) Proof {
	switch req.Alg {
	case Sha2BDay:
		return Proof{fulfilSha2BDay(req.Nonce, req.Difficulty, data)}
	default:
		panic("No such algorithm")
	}
}

// Convenience function to fulfil the proof of work request
func Fulfil(request string, data []byte) (string, error) {
	var req Request
	err := req.UnmarshalText([]byte(request))
	if err != nil {
		return "", err
	}
	proof := req.Fulfil(data)
	s, _ := proof.MarshalText()
	return string(s), nil
}

// Check whether the proof is ok
func (proof *Proof) Check(req Request, data []byte) bool {
	switch req.Alg {
	case Sha2BDay:
		return checkSha2BDay(proof.buf, req.Nonce, data, req.Difficulty)
	default:
		panic("No such algorithm")
	}
}
