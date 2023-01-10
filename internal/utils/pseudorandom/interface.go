package pseudorandom

type Pseudorandom interface {
	// Uint32n a pseudo random uint32 based on seed.
	Uint32n(maxN uint32) uint32
}
