// from https://github.com/xtaci/smux/blob/master/alloc.go
package byte_alloc

import (
	"sync"
)

// magic number
var debruijinPos = [...]byte{0, 9, 1, 10, 13, 21, 2, 29, 11, 14, 16, 18, 22, 25, 3, 30, 8, 12, 20, 28, 15, 17, 24, 7, 19, 27, 23, 6, 26, 5, 4, 31}

// Allocator for incoming frames, optimized to prevent overwriting after zeroing
type Allocator struct {
	buffers []sync.Pool
}

// NewAllocator initiates a []byte allocator for frames less than 1048576 bytes,
// the waste(memory fragmentation) of space allocation is guaranteed to be
// no more than 50%.
func NewAllocator() *Allocator {
	alloc := new(Allocator)
	alloc.buffers = make([]sync.Pool, 21) // 1B -> 1M
	for k := range alloc.buffers {
		size := 1 << uint32(k)
		alloc.buffers[k].New = func() interface{} {
			return make([]byte, size)
		}
	}
	return alloc
}

// Get a []byte from pool with most appropriate cap
func (alloc *Allocator) Get(size int) []byte {
	if size <= 0 {
		return nil
	}

	if size > 1048576 {
		return make([]byte, size)
	}

	bits := msb(size)
	if size == 1<<bits {
		return alloc.buffers[bits].Get().([]byte)[:size]
	} else {
		return alloc.buffers[bits+1].Get().([]byte)[:size]
	}
}

// Put returns a []byte to pool for future use,
// which the cap must be exactly 2^n
func (alloc *Allocator) Put(buf []byte) {
	bufCap := cap(buf)
	bits := msb(bufCap)
	if bufCap == 0 || bufCap > 65536 || bufCap != 1<<bits {
		return
	}

	alloc.buffers[bits].Put(buf)
	return
}

// msb return the pos of most significiant bit
// Equivalent to: uint(math.Floor(math.Log2(float64(n))))
// http://supertech.csail.mit.edu/papers/debruijn.pdf
func msb(size int) byte {
	v := uint32(size)
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	return debruijinPos[(v*0x07C4ACDD)>>27]
}
