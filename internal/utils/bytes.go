package utils

import (
	"encoding/hex"
	"math/big"
)

// use to look up number of 1 bit in 4 bits
var halfByteLookup = [16]int{0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4}

// FromHex returns the bytes represented by the hexadecimal string s.
// s may be prefixed with "0x".
func FromHex(s string) []byte {
	if len(s) > 1 {
		if s[0:2] == "0x" || s[0:2] == "0X" {
			s = s[2:]
		}
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return Hex2Bytes(s)
}

// Hex2Bytes returns the bytes represented by the hexadecimal string str.
func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)
	return h
}

// counts number of one bits in a byte
func countOneBitsInByte(by byte) int {
	return halfByteLookup[by&0x0F] + halfByteLookup[by>>4]
}

// CountOneBits counts the number of 1 bit in byte array
func CountOneBits(arr []byte) int64 {
	if arr == nil {
		return 0
	}
	if len(arr) == 0 {
		return 0
	}
	count := 0
	for i := range arr {
		count += countOneBitsInByte(arr[i])
	}
	return int64(count)
}

func BytesMiddle(a, b []byte) []byte {
	if a == nil && b == nil {
		return []byte{128}
	}

	if len(a) > len(b) {
		tmp := make([]byte, len(a))
		if b == nil {
			for i, _ := range tmp {
				tmp[i] = 255
			}
		}
		copy(tmp, b)
		b = tmp
	} else if len(a) < len(b) {
		tmp := make([]byte, len(b))
		if a == nil {
			for i, _ := range tmp {
				tmp[i] = 0
			}
		}
		copy(tmp, a)
		a = tmp
	}

	aI := big.NewInt(0).SetBytes(a)
	bI := big.NewInt(0).SetBytes(b)

	aI.Add(aI, bI)
	aI.Div(aI, big.NewInt(2))
	return aI.Bytes()
}
