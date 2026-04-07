package internal

type Set[T comparable] struct {
	items map[T]struct{}
	size  int
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{items: make(map[T]struct{}, 0)}
}

func NewSetWithSize[T comparable](len int) *Set[T] {
	return &Set[T]{items: make(map[T]struct{}, len)}
}

func (s *Set[T]) Add(item T) {
	if _, exists := s.items[item]; exists {
		return
	}
	s.items[item] = struct{}{}
	s.size += 1
}

func (s *Set[T]) Remove(item T) {
	if _, exists := s.items[item]; !exists {
		return
	}
	delete(s.items, item)
	s.size -= 1
}

func (s *Set[T]) Clear() {
	s.items = make(map[T]struct{}, 0)
	s.size = 0
}

func (s *Set[T]) Size() int {
	return s.size
}

func (s *Set[T]) Contains(item T) bool {
	_, exists := s.items[item]
	return exists
}

func (s *Set[T]) Items() []T {
	keys := make([]T, 0, s.size)
	for key := range s.items {
		keys = append(keys, key)
	}
	return keys
}
