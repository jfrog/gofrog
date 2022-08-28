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
