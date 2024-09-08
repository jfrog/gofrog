package safeconvert

import (
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
)

func TestSafeIntToUint(t *testing.T) {
	tests := []struct {
		input       int
		expected    uint
		errExpected bool
	}{
		{input: 10, expected: 10},
		{input: -1, expected: 0, errExpected: true},
		{input: 0, expected: 0},
	}

	for _, test := range tests {
		result, err := IntToUint(test.input)
		if test.errExpected {
			assert.Error(t, err, "Expected an error for input: %d", test.input)
		} else {
			assert.NoError(t, err, "Did not expect an error for input: %d", test.input)
			assert.Equal(t, test.expected, result, "Expected result does not match")
		}
	}
}

func TestSafeUintToInt(t *testing.T) {
	tests := []struct {
		input       uint
		expected    int
		errExpected bool
	}{
		{input: 10, expected: 10},
		{input: uint(math.MaxInt), expected: math.MaxInt},
		{input: uint(math.MaxInt) + 1, expected: 0, errExpected: true},
	}

	for _, test := range tests {
		result, err := UintToInt(test.input)
		if test.errExpected {
			assert.Error(t, err, "Expected an error for input: %d", test.input)
		} else {
			assert.NoError(t, err, "Did not expect an error for input: %d", test.input)
			assert.Equal(t, test.expected, result, "Expected result does not match")
		}
	}
}

func TestSafeInt64ToUint64(t *testing.T) {
	tests := []struct {
		input       int64
		expected    uint64
		errExpected bool
	}{
		{input: 10, expected: 10},
		{input: -1, expected: 0, errExpected: true},
		{input: 0, expected: 0},
	}

	for _, test := range tests {
		result, err := Int64ToUint64(test.input)
		if test.errExpected {
			assert.Error(t, err, "Expected an error for input: %d", test.input)
		} else {
			assert.NoError(t, err, "Did not expect an error for input: %d", test.input)
			assert.Equal(t, test.expected, result, "Expected result does not match")
		}
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		input       uint64
		expected    int64
		errExpected bool
	}{
		{input: 10, expected: 10},
		{input: math.MaxInt64, expected: math.MaxInt64},
		{input: math.MaxInt64 + 1, expected: 0, errExpected: true},
	}

	for _, test := range tests {
		result, err := Uint64ToInt64(test.input)
		if test.errExpected {
			assert.Error(t, err, "Expected an error for input: %d", test.input)
		} else {
			assert.NoError(t, err, "Did not expect an error for input: %d", test.input)
			assert.Equal(t, test.expected, result, "Expected result does not match")
		}
	}
}
