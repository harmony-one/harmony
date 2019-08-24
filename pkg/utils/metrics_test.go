package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test for BToMb.
func TestBToMb(t *testing.T) {
	a := uint64(1024*1024 + 1)
	assert.Equal(t, BToMb(a), uint64(1), "should be equal to 1")

	a = uint64(1024*1024 - 1)
	assert.Equal(t, BToMb(a), uint64(0), "should be equal to 0")

	a = uint64(1024 * 1024)
	assert.Equal(t, BToMb(a), uint64(1), "should be equal to 0")
}
