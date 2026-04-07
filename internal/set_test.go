package internal_test

import (
	"sort"
	"testing"
	"tiny-file-watcher/internal"

	"github.com/stretchr/testify/assert"
)

func TestNewSet_Empty(t *testing.T) {
	s := internal.NewSet[string]()
	assert.Empty(t, s.Items())
}

func TestNewSetWithSize_Empty(t *testing.T) {
	s := internal.NewSetWithSize[int](10)
	assert.Empty(t, s.Items())
	assert.Zero(t, s.Size())
}

func TestAdd_ContainsSingleItem(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("hello")
	assert.True(t, s.Contains("hello"))
}

func TestAdd_DoesNotContainAbsentItem(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("hello")
	assert.False(t, s.Contains("world"))
}

func TestAdd_Idempotent(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("dup")
	s.Add("dup")
	assert.Len(t, s.Items(), 1)
	assert.Equal(t, 1, s.Size())
}

func TestAdd_MultipleItems(t *testing.T) {
	s := internal.NewSet[int]()
	s.Add(1)
	s.Add(2)
	s.Add(3)
	assert.True(t, s.Contains(1))
	assert.True(t, s.Contains(2))
	assert.True(t, s.Contains(3))
	assert.Len(t, s.Items(), 3)
	assert.Equal(t, 3, s.Size())
}

func TestRemove_RemovesExistingItem(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("a")
	s.Remove("a")
	assert.False(t, s.Contains("a"))
	assert.Empty(t, s.Items())
	assert.Equal(t, 0, s.Size())
}

func TestRemove_NoOpForAbsentItem(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("a")
	s.Remove("missing")
	assert.True(t, s.Contains("a"))
	assert.Len(t, s.Items(), 1)
	assert.Equal(t, 1, s.Size())
}

func TestContains_EmptySet(t *testing.T) {
	s := internal.NewSet[string]()
	assert.False(t, s.Contains("anything"))
}

func TestItems_ReturnsAllElements(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("b")
	s.Add("a")
	s.Add("c")
	items := s.Items()
	sort.Strings(items)
	assert.Equal(t, []string{"a", "b", "c"}, items)
}

func TestItems_EmptySet(t *testing.T) {
	s := internal.NewSet[string]()
	assert.Empty(t, s.Items())
	assert.Zero(t, s.Size())
}

func TestSet_IntType(t *testing.T) {
	s := internal.NewSet[int]()
	s.Add(42)
	assert.True(t, s.Contains(42))
	assert.False(t, s.Contains(0))
	s.Remove(42)
	assert.False(t, s.Contains(42))
}

func TestSize_AfterDuplicateAdd(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("dup")
	s.Add("dup")
	assert.Equal(t, 1, s.Size())
}

func TestSize_AfterClear(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("a")
	s.Add("b")
	s.Clear()
	assert.Equal(t, 0, s.Size())
}

func TestSize_RemoveAbsentItem(t *testing.T) {
	s := internal.NewSet[string]()
	s.Add("a")
	s.Remove("missing")
	assert.Equal(t, 1, s.Size())
}
