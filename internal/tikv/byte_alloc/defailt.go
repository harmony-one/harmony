package byte_alloc

var defaultAllocator *Allocator

func init() {
	defaultAllocator = NewAllocator()
}

func Get(size int) []byte {
	return defaultAllocator.Get(size)
}

func Put(buf []byte) {
	defaultAllocator.Put(buf)
}
