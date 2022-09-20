package linkedhashmap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type value struct {
	v int
}

func TestHashMap(t *testing.T) {
	h := New[int, *value]()
	for i := 0; i < 3; i++ {
		require.Equal(t, 0, h.Count())
		require.Nil(t, h.Next())
		require.False(t, h.HasNext())

		h.Add(1, &value{1})
		require.Equal(t, 1, h.Count())

		h.Add(2, &value{2})
		require.Equal(t, 2, h.Count())

		require.Equal(t, &value{1}, h.Pop().Value)
		require.Equal(t, 1, h.Count())
		require.Equal(t, &value{2}, h.Next().Value)
		h.Remove(h.Next().Key)
	}
}
