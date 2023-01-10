package pseudorandom

func NewXorshiftPseudorandom(seed uint32) *XorshiftPseudorandom {
	if seed == 0 {
		seed = 1
	}
	return &XorshiftPseudorandom{
		x: seed,
	}
}

type XorshiftPseudorandom struct {
	x uint32
}

// Uint32 returns pseudorandom uint32.
//
// It is unsafe to call this method from concurrent goroutines.
func (r *XorshiftPseudorandom) Uint32() uint32 {
	// See https://en.wikipedia.org/wiki/Xorshift
	x := r.x
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	r.x = x
	return x
}

// Uint32n returns pseudorandom uint32 in the range [0..maxN).
//
// It is unsafe to call this method from concurrent goroutines.
func (r *XorshiftPseudorandom) Uint32n(maxN uint32) uint32 {
	x := r.Uint32()
	// See http://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return uint32((uint64(x) * uint64(maxN)) >> 32)
}
