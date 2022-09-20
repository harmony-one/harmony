package linkedhashmap

type DoubleLinkedList[T any] struct {
	first *DoubleLinkedListNode[T] // pust only to front
	last  *DoubleLinkedListNode[T]
	count int
}

type DoubleLinkedListNode[T any] struct {
	Front *DoubleLinkedListNode[T]
	Back  *DoubleLinkedListNode[T]
	Value T
}

func (a *DoubleLinkedList[T]) Count() int {
	return a.count
}

func (a *DoubleLinkedList[T]) PushBack(value T) *DoubleLinkedListNode[T] {
	newNode := &DoubleLinkedListNode[T]{Value: value}
	if a.count == 0 {
		a.last = newNode
		a.first = newNode
	} else {
		back := a.last
		a.last = newNode
		newNode.Front = back
		back.Back = newNode
	}

	a.count++
	return newNode
}

func (a *DoubleLinkedList[T]) PopFront() *T {
	if a.count == 0 {
		return nil
	}
	first := a.first
	a.first = a.first.Back
	a.count--
	return &first.Value
}

func (a *DoubleLinkedList[T]) PopBack() *T {
	if a.count == 0 {
		return nil
	}
	last := a.last
	a.last = a.last.Front
	a.count--
	return &last.Value
}

func (a *DoubleLinkedList[T]) Front() *DoubleLinkedListNode[T] {
	if a.count == 0 {
		return nil
	}
	return a.first
}

func (a *DoubleLinkedList[T]) Back() *DoubleLinkedListNode[T] {
	if a.count == 0 {
		return nil
	}
	return a.last
}

func (a *DoubleLinkedList[T]) RemoveNode(node *DoubleLinkedListNode[T]) {
	if a.count == 0 || node == nil {
		return
	}

	if node.Front != nil && node.Front.Back != nil {
		node.Front.Back = node.Back
	}
	if node.Back != nil && node.Back.Front != nil {
		node.Back.Front = node.Front
	}
	if node == a.first { // first node
		a.first = a.first.Back
	}
	if node == a.last {
		a.last = a.last.Front
	}
	a.count--
}
