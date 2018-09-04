package pow

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
)

func checkSha2BDay(proof []byte, nonce, data []byte, diff uint32) bool {
	if len(proof) != 24 {
		return false
	}
	prefix1 := proof[:8]
	prefix2 := proof[8:16]
	prefix3 := proof[16:]
	if bytes.Equal(prefix1, prefix2) || bytes.Equal(prefix2, prefix3) ||
		bytes.Equal(prefix1, prefix3) {
		return false
	}
	resBuf := make([]byte, 32)
	h := sha256.New()
	h.Write(prefix1)
	h.Write(data)
	h.Write(nonce)
	h.Sum(resBuf[:0])
	res1 := binary.BigEndian.Uint64(resBuf) & ((1 << diff) - 1)
	h.Reset()
	h.Write(prefix2)
	h.Write(data)
	h.Write(nonce)
	h.Sum(resBuf[:0])
	res2 := binary.BigEndian.Uint64(resBuf) & ((1 << diff) - 1)
	h.Reset()
	h.Write(prefix3)
	h.Write(data)
	h.Write(nonce)
	h.Sum(resBuf[:0])
	res3 := binary.BigEndian.Uint64(resBuf) & ((1 << diff) - 1)
	return res1 == res2 && res2 == res3
}

func fulfilSha2BDay(nonce []byte, diff uint32, data []byte) []byte {
	// TODO make multithreaded if the difficulty is high enough.
	//      For light proof-of-work requests, the overhead of parallelizing is
	//      not worth it.
	type Pair struct {
		First, Second uint64
	}
	var i uint64 = 1
	prefix := make([]byte, 8)
	resBuf := make([]byte, 32)
	lut := make(map[uint64]Pair)
	h := sha256.New()
	for {
		binary.BigEndian.PutUint64(prefix, i)
		h.Write(prefix)
		h.Write(data)
		h.Write(nonce)
		h.Sum(resBuf[:0])
		res := binary.BigEndian.Uint64(resBuf) & ((1 << diff) - 1)
		pair, ok := lut[res]
		if ok {
			if pair.Second != 0 {
				ret := make([]byte, 24)
				binary.BigEndian.PutUint64(ret, pair.First)
				binary.BigEndian.PutUint64(ret[8:], pair.Second)
				copy(ret[16:], prefix)
				return ret
			}

			lut[res] = Pair{First: pair.First, Second: i}
		} else {
			lut[res] = Pair{First: i}
		}
		h.Reset()
		i++
	}
}
