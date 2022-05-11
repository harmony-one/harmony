package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrosslinkStorage(t *testing.T) {
	s := NewCrosslinkStorage()
	s.Set(1, 5)
	require.EqualValues(t, s.Get(1), 5)
	require.EqualValues(t, s.Get(2), 0)
}
