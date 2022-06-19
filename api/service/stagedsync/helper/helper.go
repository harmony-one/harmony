package helper

import (
	"fmt"
	"hash"
	"math/big"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

// BigToAddress returns Address with byte values of b.
// If b is larger than len(h), b will be cropped from the left.
func BigToAddress(b *big.Int) common.Address { return common.BytesToAddress(b.Bytes()) }

// HexToAddress returns Address with byte values of s.
// If s is larger than len(h), s will be cropped from the left.
func HexToAddress(s string) common.Address { return common.BytesToAddress(common.FromHex(s)) }

// Report gives off a warning requesting the user to submit an issue to the github tracker.
func Report(extra ...interface{}) {
	fmt.Fprintln(os.Stderr, "You've encountered a sought after, hard to reproduce bug. Please report this to the developers <3 https://github.com/ledgerwatch/erigon/issues")
	fmt.Fprintln(os.Stderr, extra...)

	_, file, line, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "%v:%v\n", file, line)

	debug.PrintStack()

	fmt.Fprintln(os.Stderr, "#### BUG! PLEASE REPORT ####")
}

// PrintDepricationWarning prinst the given string in a box using fmt.Println.
func PrintDepricationWarning(str string) {
	line := strings.Repeat("#", len(str)+4)
	emptyLine := strings.Repeat(" ", len(str))
	fmt.Printf(`
%s
# %s #
# %s #
# %s #
%s

`, line, emptyLine, str, emptyLine, line)
}

// keccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type keccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

type Hasher struct {
	Sha keccakState
}

var hasherPool = make(chan *Hasher, 128)

func NewHasher() *Hasher {
	var h *Hasher
	select {
	case h = <-hasherPool:
	default:
		h = &Hasher{Sha: sha3.NewLegacyKeccak256().(keccakState)}
	}
	return h
}

func ReturnHasherToPool(h *Hasher) {
	select {
	case hasherPool <- h:
	default:
		fmt.Printf("Allowing Hasher to be garbage collected, pool is full\n")
	}
}

func HashData(data []byte) (common.Hash, error) {
	h := NewHasher()
	defer ReturnHasherToPool(h)
	h.Sha.Reset()

	_, err := h.Sha.Write(data)
	if err != nil {
		return common.Hash{}, err
	}

	var buf common.Hash
	_, err = h.Sha.Read(buf[:])
	if err != nil {
		return common.Hash{}, err
	}
	return buf, nil
}
