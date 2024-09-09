package safeconvert

import (
	"errors"
	"math"
)

// IntToUint converts int to uint safely, checking for negative values.
func IntToUint(i int) (uint, error) {
	if i < 0 {
		return 0, errors.New("cannot convert negative int to uint")
	}
	return uint(i), nil
}

// UintToInt converts uint to int safely, checking for overflow.
func UintToInt(u uint) (int, error) {
	if u > math.MaxInt {
		return 0, errors.New("integer overflow: uint value exceeds max int value")
	}
	return int(u), nil
}

// Int64ToUint64 converts int64 to uint64 safely, checking for negative values.
func Int64ToUint64(i int64) (uint64, error) {
	if i < 0 {
		return 0, errors.New("cannot convert negative int64 to uint64")
	}
	return uint64(i), nil
}

// Uint64ToInt64 converts uint64 to int64 safely, checking for overflow.
func Uint64ToInt64(u uint64) (int64, error) {
	if u > math.MaxInt64 {
		return 0, errors.New("integer overflow: uint64 value exceeds max int64 value")
	}
	return int64(u), nil
}

// SafeUint64ToInt converts uint64 to int safely, checking for overflow.
func Uint64ToInt(u uint64) (int, error) {
	if u > uint64(math.MaxInt) {
		return 0, errors.New("integer overflow: uint64 value exceeds max int value")
	}
	return int(u), nil
}
