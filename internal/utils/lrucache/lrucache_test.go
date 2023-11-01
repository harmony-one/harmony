package lrucache

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeys(t *testing.T) {
	c := NewCache[int, int](10)

	for i := 0; i < 3; i++ {
		c.Set(i, i)
	}
	m := map[int]int{
		0: 0,
		1: 1,
		2: 2,
	}
	keys := c.Keys()

	m2 := map[int]int{}
	for _, k := range keys {
		m2[k] = k
	}

	require.Equal(t, m, m2)
}
