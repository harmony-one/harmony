package linkedhashmap

import "fmt"

type internal[K, V any] struct {
	key   *DoubleLinkedListNode[K]
	value V
}

type HashMap[K comparable, V any] struct {
	m          map[K]internal[K, V]
	linkedList DoubleLinkedList[K]
}

func New[K comparable, V any]() *HashMap[K, V] {
	return &HashMap[K, V]{
		m:          make(map[K]internal[K, V]),
		linkedList: DoubleLinkedList[K]{},
	}
}

func (a *HashMap[K, V]) Add(k K, v V) {
	if node, ok := a.m[k]; ok {
		a.linkedList.RemoveNode(node.key)
	}

	a.m[k] = internal[K, V]{
		key:   a.linkedList.PushBack(k),
		value: v,
	}
}

func (a *HashMap[K, V]) Count() int {
	return len(a.m)
}

func (a *HashMap[K, V]) Pop() *Entry[K, V] {
	if len(a.m) == 0 {
		return nil
	}
	key := a.linkedList.PopFront()
	if v, ok := a.m[*key]; ok {
		delete(a.m, *key)
		return &Entry[K, V]{*key, v.value}
	}
	panic(fmt.Sprintf("key %v not found", *key))
}

func (a *HashMap[K, V]) HasNext() bool {
	return len(a.m) > 0
}

func (a *HashMap[K, V]) Next() *Entry[K, V] {
	if len(a.m) == 0 {
		return nil
	}
	key := a.linkedList.Front()
	if v, ok := a.m[key.Value]; ok {
		return &Entry[K, V]{key.Value, v.value}
	}
	panic(fmt.Sprintf("key %v not found", *key))
}

func (a *HashMap[K, V]) Remove(key K) {
	if v, ok := a.m[key]; ok {
		a.linkedList.RemoveNode(v.key)
		delete(a.m, key)
	}
}

func (a *HashMap[K, V]) Contains(key K) bool {
	_, ok := a.m[key]
	return ok
}
