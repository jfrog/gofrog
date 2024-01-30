package datastructures

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func generateNewSetWithData() *Set[int] {
	set := MakeSet[int]()
	set.Add(3)
	set.Add(5)
	set.Add(7)

	return set
}

func TestSetExistsAndAdd(t *testing.T) {
	testSet := generateNewSetWithData()
	assert.True(t, testSet.Exists(3))
	assert.False(t, testSet.Exists(4))
}

func TestSetRemove(t *testing.T) {
	testSet := generateNewSetWithData()
	assert.NoError(t, testSet.Remove(5))
	assert.Equal(t, testSet.Size(), 2)
	assert.False(t, testSet.Exists(5))
}

func TestSetToSlice(t *testing.T) {
	testSet := generateNewSetWithData()
	slice := testSet.ToSlice()
	assert.Equal(t, len(slice), 3)
	assert.Contains(t, slice, 3)
	assert.Contains(t, slice, 5)
	assert.Contains(t, slice, 7)
}

func TestMakeSetFromElements(t *testing.T) {
	intSlice := []int{1, 2, 3}
	intSet := MakeSetFromElements(intSlice...)
	assert.ElementsMatch(t, intSet.ToSlice(), intSlice)

	stringSlice := []string{"frog", "frogger", "froggy"}
	stringSet := MakeSetFromElements(stringSlice...)
	assert.ElementsMatch(t, stringSet.ToSlice(), stringSlice)
}

func TestSetsIntersection(t *testing.T) {
	testSet := generateNewSetWithData()
	anotherSet := MakeSet[int]()
	intersectedSet := testSet.Intersect(anotherSet)
	assert.Equal(t, 0, intersectedSet.Size())

	anotherSet.Add(3)
	intersectedSet = testSet.Intersect(anotherSet)
	assert.Equal(t, 1, intersectedSet.Size())
}

func TestSetsUnion(t *testing.T) {
	testSet := generateNewSetWithData()
	anotherSet := MakeSet[int]()
	unionedSet := testSet.Union(anotherSet)
	assert.Equal(t, 3, unionedSet.Size())

	anotherSet.Add(4)
	unionedSet = testSet.Union(anotherSet)
	assert.Equal(t, 4, unionedSet.Size())
}
