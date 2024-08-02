package gen

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
)

func New(seed int64) Gen {
	return NewRand(rand.New(rand.NewSource(seed)))
}

func NewRand(r *rand.Rand) Gen {
	return Gen{
		r: r,
	}
}

type Gen struct {
	r *rand.Rand
}

func (a Gen) Address() common.Address {
	var addr common.Address
	a.r.Read(addr[:])
	return addr
}

func (a Gen) Uint64() uint64 {
	return a.r.Uint64()
}

func (a Gen) Uint32() uint32 {
	return a.r.Uint32()
}

func (a Gen) BigInt() *big.Int {
	return big.NewInt(a.r.Int63())
}

func (a Gen) Hash() common.Hash {
	var hash common.Hash
	a.r.Read(hash[:])
	return hash
}

func (a Gen) Bytes96() [96]byte {
	var b [96]byte
	a.r.Read(b[:])
	return b
}

func (a Gen) BytesN(n int) []byte {
	b := make([]byte, n)
	a.r.Read(b)
	return b
}
