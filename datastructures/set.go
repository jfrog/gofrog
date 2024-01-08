package datastructures

import "fmt"

type Set[T comparable] struct {
	container map[T]struct{}
}

// MakeSet initialize the set
func MakeSet[T comparable]() *Set[T] {
	return &Set[T]{
		container: make(map[T]struct{}),
	}
}

func MakeSetFromElements[T comparable](elements ...T) *Set[T] {
	set := MakeSet[T]()
	for _, element := range elements {
		set.Add(element)
	}
	return set
}

func (set *Set[T]) Exists(key T) bool {
	_, exists := set.container[key]
	return exists
}

func (set *Set[T]) Add(key T) {
	set.container[key] = struct{}{}
}

func (set *Set[T]) AddElements(elements ...T) {
	for _, element := range elements {
		set.Add(element)
	}
}

func (set *Set[T]) Remove(key T) error {
	_, exists := set.container[key]
	if !exists {
		return fmt.Errorf("remove Error: item doesn't exist in set")
	}
	delete(set.container, key)
	return nil
}

func (set *Set[T]) Size() int {
	return len(set.container)
}

func (set *Set[T]) ToSlice() []T {
	var slice []T
	for key := range set.container {
		slice = append(slice, key)
	}

	return slice
}
