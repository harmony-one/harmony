package linkedhashmap

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoubleLinkedList(t *testing.T) {
	l := DoubleLinkedList[*value]{}
	for i := 0; i < 3; i++ { // to be sure previous iterations didn't break something
		require.Equal(t, 0, l.Count())

		l.PushBack(&value{1})
		l.PushBack(&value{2})
		l.PushBack(&value{3})
		l.PushBack(&value{4})

		require.Equal(t, 4, l.Count())

		require.Equal(t, &value{1}, *l.PopFront())
		require.Equal(t, 3, l.Count())

		require.Equal(t, &value{4}, *l.PopBack())
		require.Equal(t, 2, l.Count())

		require.Equal(t, &value{2}, l.Front().Value)
		l.RemoveNode(l.Front())

		require.Equal(t, &value{3}, l.Back().Value)
		l.RemoveNode(l.Back())

		// this methods shouldn't affect
		require.Empty(t, l.Front())
		require.Empty(t, l.Back())
		require.Empty(t, l.PopFront())
		require.Empty(t, l.PopBack())
		l.RemoveNode(nil)
	}
}
